// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"unicode/utf8"

	json "github.com/goccy/go-json"
)

// ErrHALFieldConflict is returned when user data already contains a reserved
// top-level HAL property. Emitting those fields would create duplicate JSON
// object keys, which are not interoperable.
var ErrHALFieldConflict = errors.New("hal: data already contains reserved HAL field")

// MarshalJSON implements the json.Marshaler interface.
// It serializes the wrapped Data and splices in the HAL "_links" and "_embedded"
// fields into the resulting JSON object.
func (e Envelope) MarshalJSON() ([]byte, error) {
	// Caller-supplied precomputed links are validated at construction time;
	// surface that error rather than emitting corrupted JSON.
	if e.precomputedErr != nil {
		return nil, e.precomputedErr
	}

	// OPTIMIZATION: Fast path for pre-computed JSON
	if e.precomputedJSON != nil {
		return e.marshalPrecomputed()
	}
	return e.marshalGenerated()
}

func (e Envelope) marshalPrecomputed() ([]byte, error) {
	if e.Data == nil {
		return e.precomputedJSON, nil
	}
	dataBytes, err := json.Marshal(e.Data)
	if err != nil {
		return nil, err
	}
	isDataNull, isDataEmptyObj, err := checkJSONStructure(dataBytes)
	if err != nil {
		return nil, err
	}
	if isDataNull || isDataEmptyObj {
		return e.precomputedJSON, nil
	}
	if err := validateDataJSON(dataBytes); err != nil {
		return nil, err
	}
	if err := checkReservedHALFields(dataBytes); err != nil {
		return nil, err
	}
	return splicePrecomputed(dataBytes, e.precomputedJSON), nil
}

func (e Envelope) marshalGenerated() ([]byte, error) {
	// 1. Marshal the underlying data
	dataBytes, err := e.marshalData()
	if err != nil {
		return nil, err
	}

	// 2. Validate data is an object (so we can inject fields)
	isDataNull, isEmptyObj, err := checkJSONStructure(dataBytes)
	if err != nil {
		return nil, err
	}
	if err := validateDataJSON(dataBytes); err != nil {
		return nil, err
	}

	// Reject data that already defines reserved HAL properties; splicing
	// would emit duplicate JSON keys.
	if !isDataNull && !isEmptyObj {
		if err := checkReservedHALFields(dataBytes); err != nil {
			return nil, err
		}
	}

	// 3. Prepare HAL metadata (_links, _embedded)
	metaBytes, err := e.marshalMeta()
	if err != nil {
		return nil, err
	}

	// 4. Combine
	return spliceJSON(dataBytes, metaBytes, isDataNull, isEmptyObj), nil
}

func validateDataJSON(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("hal: data must be valid UTF-8")
	}
	return nil
}

func splicePrecomputed(data, linksJSON []byte) []byte {
	data = bytes.TrimSpace(data)
	linksJSON = bytes.TrimSpace(linksJSON)

	// linksJSON is already {"_links":{...}}
	// We need to combine data and linksJSON
	if len(data) == 0 || bytes.Equal(data, []byte("null")) || isEmptyJSONObject(data) {
		return linksJSON
	}
	if len(linksJSON) == 0 || isEmptyJSONObject(linksJSON) {
		return data
	}
	if !hasJSONObjectDelimiters(data) || !hasJSONObjectDelimiters(linksJSON) {
		return data
	}
	// Remove trailing } from data and { from linksJSON
	result := make([]byte, 0, len(data)+len(linksJSON)-jsonTrailingChars)
	result = append(result, data[:len(data)-1]...)
	result = append(result, ',')
	result = append(result, linksJSON[1:]...)
	return result
}

func hasJSONObjectDelimiters(b []byte) bool {
	return len(b) >= 2 && b[0] == '{' && b[len(b)-1] == '}'
}

// isEmptyJSONObject reports whether b is an object containing only whitespace.
func isEmptyJSONObject(b []byte) bool {
	if !hasJSONObjectDelimiters(b) {
		return false
	}
	return len(bytes.TrimSpace(b[1:len(b)-1])) == 0
}

func (e Envelope) marshalData() ([]byte, error) {
	if e.Data == nil {
		return nil, nil
	}
	return json.Marshal(e.Data)
}

// checkJSONStructure returns (isNull, isEmptyObject, error).
// Optimized: uses bytes comparison to avoid string allocation.
func checkJSONStructure(b []byte) (bool, bool, error) {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 {
		return true, false, nil
	}

	// Null data (including typed nil pointers and json.RawMessage("null"))
	// is normalized to an empty resource.
	if bytes.Equal(trimmed, []byte("null")) {
		return true, false, nil
	}

	// Must start with '{'
	if trimmed[0] != '{' {
		return false, false, errors.New("hal: data must be a JSON object to splice")
	}

	// Check for an empty object, including whitespace-only bodies.
	if isEmptyJSONObject(trimmed) {
		return false, true, nil
	}

	return false, false, nil
}

// findDuplicateJSONKeys scans for duplicate object member names at any depth.
// Duplicate names are non-interoperable and cannot be detected after decoding
// into a map.
type jsonFrame struct {
	object  bool
	keys    map[string]struct{}
	wantKey bool
}

func newJSONTokenScanner(data []byte) *stdjson.Decoder {
	dec := stdjson.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec
}

type jsonDuplicateScanner struct {
	stack    []jsonFrame
	seenRoot bool
	done     bool
}

func findDuplicateJSONKeys(data []byte) error {
	dec := newJSONTokenScanner(data)
	var scanner jsonDuplicateScanner

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return scanner.finish()
		}
		if err != nil {
			return err
		}
		if err := scanner.scan(tok); err != nil {
			return err
		}
	}
}

func (s *jsonDuplicateScanner) finish() error {
	if !s.seenRoot || !s.done {
		return errors.New("unexpected EOF")
	}
	return nil
}

func (s *jsonDuplicateScanner) scan(tok stdjson.Token) error {
	if s.done {
		return errors.New("unexpected trailing JSON value")
	}
	if len(s.stack) == 0 {
		s.seenRoot = true
	}

	var err error
	if delim, ok := tok.(stdjson.Delim); ok {
		s.stack, err = scanJSONDelimiter(s.stack, delim)
	} else {
		err = scanJSONObjectToken(s.stack, tok)
	}
	if err != nil {
		return err
	}
	if len(s.stack) == 0 {
		s.done = true
	}
	return nil
}

func scanJSONDelimiter(stack []jsonFrame, delim stdjson.Delim) ([]jsonFrame, error) {
	switch delim {
	case '{':
		return append(stack, jsonFrame{
			object:  true,
			keys:    make(map[string]struct{}),
			wantKey: true,
		}), nil
	case '[':
		return append(stack, jsonFrame{}), nil
	default:
		if len(stack) == 0 {
			return nil, errors.New("unbalanced closing delimiter")
		}
		stack = stack[:len(stack)-1]
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].wantKey = true
		}
		return stack, nil
	}
}

func scanJSONObjectToken(stack []jsonFrame, tok stdjson.Token) error {
	n := len(stack)
	if n == 0 || !stack[n-1].object {
		return nil
	}
	top := &stack[n-1]
	if !top.wantKey {
		top.wantKey = true
		return nil
	}
	key, ok := tok.(string)
	if !ok {
		return fmt.Errorf("object key has type %T, want string", tok)
	}
	if _, exists := top.keys[key]; exists {
		return fmt.Errorf("duplicate object key %q", key)
	}
	top.keys[key] = struct{}{}
	top.wantKey = false
	return nil
}

// checkReservedHALFields reports a conflict only when a reserved name appears
// as a top-level object key. Nested state fields remain ordinary JSON.
func checkReservedHALFields(data []byte) error {
	// Cheap pre-check before allocating a token scanner.
	if !mayContainReservedHALField(data) {
		return nil
	}

	dec := stdjson.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("hal: inspect data object: %w", err)
	}
	delim, ok := tok.(stdjson.Delim)
	if !ok || delim != '{' {
		return nil
	}

	for dec.More() {
		if err := checkHALFieldKey(dec); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("hal: inspect data object end: %w", err)
	}
	var trailing stdjson.RawMessage
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("hal: data must contain exactly one JSON object")
	}
	return nil
}

func mayContainReservedHALField(data []byte) bool {
	return bytes.Contains(data, []byte(fieldLinks)) ||
		bytes.Contains(data, []byte(fieldEmbedded)) ||
		bytes.Contains(data, []byte(`\u`))
}

func isReservedHALField(key string) bool {
	return key == fieldLinks || key == fieldEmbedded
}

func checkHALFieldKey(dec *stdjson.Decoder) error {
	keyToken, err := dec.Token()
	if err != nil {
		return fmt.Errorf("hal: inspect data object key: %w", err)
	}
	key, ok := keyToken.(string)
	if !ok {
		return fmt.Errorf("hal: data object key has type %T, want string", keyToken)
	}
	if isReservedHALField(key) {
		return fmt.Errorf("%w: %q", ErrHALFieldConflict, key)
	}

	var value stdjson.RawMessage
	if err := dec.Decode(&value); err != nil {
		return fmt.Errorf("hal: inspect data object value: %w", err)
	}
	return nil
}

func (e Envelope) marshalMeta() ([]byte, error) {
	// Build a local view instead of mutating e.links: an Envelope may be
	// marshaled more than once, including concurrently.
	links := e.links
	if e.instance != nil {
		if curies := e.instance.resolveCuries(e.links); len(curies) > 0 {
			links = make(map[string]any, len(e.links)+1)
			for rel, value := range e.links {
				links[rel] = value
			}
			links[relCuries] = mergeCuries(links[relCuries], curies)
		}
	}

	// Fast path: no links
	if len(links) == 0 {
		if len(e.embedded) > 0 {
			return json.Marshal(map[string]any{fieldEmbedded: e.embedded})
		}
		return nil, nil
	}

	// Only create map if needed
	if len(e.embedded) > 0 {
		return json.Marshal(map[string]any{
			fieldLinks:    links,
			fieldEmbedded: e.embedded,
		})
	}

	return json.Marshal(map[string]any{fieldLinks: links})
}

// mergeCuries appends resolved CURIE declarations to any user-supplied curies
// relation without nesting relation arrays.
func mergeCuries(existing any, curies []Link) any {
	merged := make([]any, 0, len(curies)+1)
	merged = appendRelationValue(merged, existing)
	for _, curie := range curies {
		merged = append(merged, curie)
	}
	return merged
}

func appendRelationValue(dst []any, value any) []any {
	switch v := value.(type) {
	case nil:
		return dst
	case []any:
		return append(dst, v...)
	case []Link:
		for _, link := range v {
			dst = append(dst, link)
		}
		return dst
	default:
		return append(dst, v)
	}
}

func spliceJSON(data []byte, meta []byte, isDataNull, isDataEmptyObj bool) []byte {
	if len(meta) == 0 {
		if isDataNull {
			return []byte("{}")
		}
		return data
	}

	if isDataNull || isDataEmptyObj {
		return meta
	}

	// Direct slice allocation - avoid bytes.Buffer overhead
	result := make([]byte, len(data)+len(meta)-1)
	n := copy(result, data[:len(data)-1])
	result[n] = ','
	copy(result[n+1:], meta[1:])
	return result
}

func (e *Envelope) computeLinks(ctx context.Context) {
	if e.Data == nil {
		return
	}

	t := reflect.TypeOf(e.Data)
	if gen, ok := e.instance.lookupGenerator(t); ok {
		links := gen(ctx, e.Data)
		for _, l := range links {
			e.AddLink(l)
		}
		return
	}

	if e.instance.strictMode {
		ptrT := reflect.PointerTo(t)
		if _, ok := e.instance.lookupGenerator(ptrT); ok {
			panic(fmt.Sprintf("hal: strict mode error. Passed value type %v, but generator registered for pointer type %v", t, ptrT))
		}
		// Strict check: if it's a struct (or ptr to struct) and no generator exists, panic.
		if t.Kind() == reflect.Struct || (t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct) {
			panic(fmt.Sprintf("hal: strict mode error. No generator registered for type %v", t))
		}
	}
}

// AddLink appends a link to the envelope.
// If a link with the same Relation (Rel) already exists, it is converted to a slice
// of links as per the HAL specification.
func (e *Envelope) AddLink(l Link) {
	e.addLinkRaw(l.Rel, l)
}

func (e *Envelope) addLinkRaw(rel string, val any) {
	if e.links == nil {
		e.links = make(map[string]any, defaultLinksCapacity)
	}
	appendLinkValue(e.links, rel, val)
}

// appendLinkValue adds a relation value with the multiplicity rules used by
// the HAL serializer: one value is an object, repeated values form an array.
func appendLinkValue(links map[string]any, rel string, val any) {
	if existing, ok := links[rel]; ok {
		if slice, isSlice := existing.([]any); isSlice {
			links[rel] = append(slice, val)
		} else {
			links[rel] = []any{existing, val}
		}
	} else {
		links[rel] = val
	}
}
