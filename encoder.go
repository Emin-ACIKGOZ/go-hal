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

// ErrAddLinkPrecomputed is returned when AddLink is called on an envelope whose
// links are static or precomputed. Those links are immutable in the envelope.
var ErrAddLinkPrecomputed = errors.New("hal: cannot add links to a static or precomputed envelope")

// MarshalJSON implements the json.Marshaler interface.
// It serializes the wrapped Data and splices in the HAL "_links" and "_embedded"
// fields into the resulting JSON object. The value receiver is preserved so
// Envelope values and *Envelope pointers both marshal; direct *Envelope
// children are marshaled manually with a shared depth and ancestor scope.
func (e Envelope) MarshalJSON() ([]byte, error) {
	return e.marshalEnvelope(0, nil, nil)
}

func (e Envelope) marshalEnvelope(depth int, ancestors map[*Envelope]struct{}, scope *curieScope) ([]byte, error) {
	// Caller-supplied precomputed links are validated at construction time;
	// surface that error rather than emitting corrupted JSON.
	if e.err != nil {
		return nil, e.err
	}
	if depth > maxEmbeddedDepth {
		return nil, fmt.Errorf("%w: maximum depth %d", ErrEmbeddedDepthExceeded, maxEmbeddedDepth)
	}
	if len(e.embedded) > 0 && ancestors == nil {
		ancestors = make(map[*Envelope]struct{})
	}

	// OPTIMIZATION: Fast path for pre-computed JSON
	if e.precomputedJSON != nil {
		return e.marshalPrecomputed(depth, ancestors, scope)
	}
	return e.marshalGenerated(depth, ancestors, scope)
}

//nolint:cyclop // Linear precomputed splice path with explicit early returns.
func (e Envelope) marshalPrecomputed(depth int, ancestors map[*Envelope]struct{}, scope *curieScope) ([]byte, error) {
	embeddedJSON, err := e.marshalEmbedded(depth, ancestors, scope)
	if err != nil {
		return nil, err
	}
	base := e.precomputedJSON
	if len(embeddedJSON) > 0 {
		base = combinePrecomputedEmbedded(base, embeddedJSON)
	}
	if e.Data == nil {
		return bytes.Clone(base), nil
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
		return bytes.Clone(base), nil
	}
	if err := validateDataJSON(dataBytes); err != nil {
		return nil, err
	}
	if err := e.scanStrictData(dataBytes, isDataNull, isDataEmptyObj); err != nil {
		return nil, err
	}
	if err := checkReservedHALFields(dataBytes); err != nil {
		return nil, err
	}
	return splicePrecomputed(dataBytes, base), nil
}

func combinePrecomputedEmbedded(base, embeddedJSON []byte) []byte {
	base = bytes.TrimSpace(base)
	if len(base) == 0 || bytes.Equal(base, []byte("{}")) {
		out := make([]byte, 0, len(embeddedJSON)+precomputedLinksPrefixLen)
		out = append(out, `{"`+fieldEmbedded+`":`...)
		out = append(out, embeddedJSON...)
		out = append(out, '}')
		return out
	}
	out := make([]byte, 0, len(base)+len(embeddedJSON)+precomputedLinksPrefixLen)
	out = append(out, base[:len(base)-1]...)
	out = append(out, `,"`+fieldEmbedded+`":`...)
	out = append(out, embeddedJSON...)
	out = append(out, '}')
	return out
}

func (e Envelope) marshalGenerated(depth int, ancestors map[*Envelope]struct{}, scope *curieScope) ([]byte, error) {
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
	if err := e.scanStrictData(dataBytes, isDataNull, isEmptyObj); err != nil {
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
	metaBytes, err := e.marshalMeta(depth, ancestors, scope)
	if err != nil {
		return nil, err
	}

	// 4. Combine
	return spliceJSON(dataBytes, metaBytes, isDataNull, isEmptyObj), nil
}

func (e Envelope) scanStrictData(data []byte, isNull, isEmptyObject bool) error {
	if e.instance == nil || !e.instance.strictJSON || isNull || isEmptyObject {
		return nil
	}
	if err := findDuplicateJSONKeys(data); err != nil {
		return fmt.Errorf("hal: data: %w", err)
	}
	return nil
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
	// We need to combine data and linksJSON. An empty link set never replaces
	// data, even when the data object is empty.
	if len(linksJSON) == 0 || isEmptyJSONObject(linksJSON) {
		return data
	}
	if len(data) == 0 || bytes.Equal(data, []byte("null")) || isEmptyJSONObject(data) {
		return linksJSON
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
	b = bytes.TrimSpace(b)
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

// ErrDuplicateJSONKey is returned when JSON contains duplicate object member
// names. Duplicate names are non-interoperable and cannot be detected after
// decoding into a map.
var ErrDuplicateJSONKey = errors.New("hal: duplicate JSON key")

const duplicateKeyDisplayLimit = 128

// DuplicateKeyError reports the first duplicate object key and its byte offset.
type DuplicateKeyError struct {
	Key    string
	Offset int64
}

func (e *DuplicateKeyError) Error() string {
	return fmt.Sprintf("hal: duplicate JSON key %q at offset %d", e.Key, e.Offset)
}

// Unwrap supports errors.Is(err, ErrDuplicateJSONKey).
func (e *DuplicateKeyError) Unwrap() error { return ErrDuplicateJSONKey }

func newDuplicateKeyError(key string, offset int64) error {
	return &DuplicateKeyError{Key: capDuplicateKey(key), Offset: offset}
}

func capDuplicateKey(key string) string {
	if len(key) <= duplicateKeyDisplayLimit {
		return key
	}
	cut := duplicateKeyDisplayLimit
	for cut > 0 && !utf8.RuneStart(key[cut]) {
		cut--
	}
	return key[:cut] + "..."
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
		offset := dec.InputOffset()
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return scanner.finish()
		}
		if err != nil {
			return err
		}
		if err := scanner.scan(tok, offset); err != nil {
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

func (s *jsonDuplicateScanner) scan(tok stdjson.Token, offset int64) error {
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
		err = scanJSONObjectToken(s.stack, tok, offset)
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

func scanJSONObjectToken(stack []jsonFrame, tok stdjson.Token, offset int64) error {
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
		return newDuplicateKeyError(key, offset)
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

//nolint:cyclop // Metadata assembly is linear; splitting hides the splice rules.
func (e Envelope) marshalMeta(depth int, ancestors map[*Envelope]struct{}, scope *curieScope) ([]byte, error) {
	// Build a local view instead of mutating e.links: an Envelope may be
	// marshaled more than once, including concurrently.
	links := e.links
	var generated []Link
	if e.instance != nil {
		generated = e.instance.resolveCuriesExcept(e.links, scope)
	}
	if _, hasUser := e.links[relCuries]; hasUser || len(generated) > 0 {
		links = make(map[string]any, len(e.links)+1)
		for rel, value := range e.links {
			links[rel] = value
		}
		merged, err := mergeCuries(links[relCuries], generated, e.instance != nil && e.instance.strictCuries)
		if err != nil {
			return nil, err
		}
		links[relCuries] = merged
	}

	if e.instance != nil {
		shaped, err := applyLinkShapes(links, e.instance.shapeFor)
		if err != nil {
			return nil, err
		}
		links = shaped
	}

	declarations, err := curieLinks(links[relCuries])
	if err != nil {
		return nil, err
	}
	childScope := extendCurieScope(scope, declarations)

	embeddedJSON, err := e.marshalEmbedded(depth, ancestors, childScope)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 && len(embeddedJSON) == 0 {
		return nil, nil
	}
	if len(links) == 0 {
		return wrapMetaObject(nil, embeddedJSON), nil
	}
	linksJSON, err := json.Marshal(links)
	if err != nil {
		return nil, err
	}
	return wrapMetaObject(linksJSON, embeddedJSON), nil
}

func wrapMetaObject(linksJSON, embeddedJSON []byte) []byte {
	out := make([]byte, 0, len(linksJSON)+len(embeddedJSON)+precomputedLinksPrefixLen*2+precomputedLinksWrapperLen)
	out = append(out, '{')
	if len(linksJSON) > 0 {
		out = append(out, `"`+fieldLinks+`":`...)
		out = append(out, linksJSON...)
	}
	if len(embeddedJSON) > 0 {
		if len(linksJSON) > 0 {
			out = append(out, ',')
		}
		out = append(out, `"`+fieldEmbedded+`":`...)
		out = append(out, embeddedJSON...)
	}
	out = append(out, '}')
	return out
}

func (e Envelope) marshalEmbedded(depth int, ancestors map[*Envelope]struct{}, scope *curieScope) ([]byte, error) {
	if len(e.embedded) == 0 {
		return nil, nil
	}
	if ancestors == nil {
		ancestors = make(map[*Envelope]struct{})
	}
	values := make(map[string]stdjson.RawMessage, len(e.embedded))
	for rel, value := range e.embedded {
		raw, err := marshalEmbeddedResource(value, depth+1, ancestors, scope)
		if err != nil {
			return nil, fmt.Errorf("hal: embedded relation %q: %w", rel, err)
		}
		values[rel] = raw
	}
	return stdjson.Marshal(values)
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
// If a link with the same Relation (Rel) already exists, it is converted to a
// slice of links as per HAL relation multiplicity.
//
// AddLink is a pointer method. On static or precomputed envelopes it does not
// add the link and defers ErrAddLinkPrecomputed to marshaling. An empty or
// whitespace-only relation is also deferred as an error. It is not safe to call
// AddLink concurrently with marshaling.
func (e *Envelope) AddLink(l Link) {
	if e.premanaged {
		if e.err == nil {
			e.err = ErrAddLinkPrecomputed
		}
		return
	}
	if e.err != nil {
		return
	}
	if err := e.validateLinkForAdd(l); err != nil {
		e.err = err
		return
	}
	e.addLinkRaw(l.Rel, l)
}

func (e *Envelope) validateLinkForAdd(l Link) error {
	if err := e.validateRelationForAdd(l.Rel, e.links); err != nil {
		return err
	}
	if e.instance != nil && e.instance.strictLinks {
		return ValidateLink(l)
	}
	return nil
}

func (e *Envelope) addLinkRaw(rel string, val any) {
	if e.links == nil {
		e.links = make(map[string]any, defaultLinksCapacity)
	}
	appendRelationValue(e.links, rel, val)
}
