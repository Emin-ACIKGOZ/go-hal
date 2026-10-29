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
func (e *Envelope) MarshalJSON() ([]byte, error) {
	// Caller-supplied precomputed links are validated at construction time;
	// surface that error rather than emitting corrupted JSON.
	if e.precomputedErr != nil {
		return nil, e.precomputedErr
	}

	// OPTIMIZATION: Fast path for pre-computed JSON
	if e.precomputedJSON != nil {
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
		if !utf8.Valid(dataBytes) {
			return nil, errors.New("hal: data must be valid UTF-8")
		}
		if err := checkReservedHALFields(dataBytes); err != nil {
			return nil, err
		}
		// Splice data with pre-computed links
		return splicePrecomputed(dataBytes, e.precomputedJSON), nil
	}

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
	if !utf8.Valid(dataBytes) {
		return nil, errors.New("hal: data must be valid UTF-8")
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

func splicePrecomputed(data, linksJSON []byte) []byte {
	data = bytes.TrimSpace(data)
	linksJSON = bytes.TrimSpace(linksJSON)

	// linksJSON is already {"_links":{...}}
	// We need to combine data and linksJSON
	if len(data) == 0 || bytes.Equal(data, []byte("{}")) || bytes.Equal(data, []byte("null")) {
		return linksJSON
	}
	if len(linksJSON) == 0 || isEmptyJSONObject(linksJSON) {
		return data
	}
	// Remove trailing } from data and , from linksJSON
	result := make([]byte, 0, len(data)+len(linksJSON)-jsonTrailingChars)
	result = append(result, data[:len(data)-1]...)
	result = append(result, ',')
	result = append(result, linksJSON[1:]...)
	return result
}

// isEmptyJSONObject reports whether b is an object containing only whitespace.
func isEmptyJSONObject(b []byte) bool {
	if len(b) < 2 || b[0] != '{' || b[len(b)-1] != '}' {
		return false
	}
	return len(bytes.TrimSpace(b[1:len(b)-1])) == 0
}

func (e *Envelope) marshalData() ([]byte, error) {
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

	// Check for empty object "{}"
	if len(trimmed) == 2 && trimmed[1] == '}' {
		return false, true, nil
	}

	return false, false, nil
}

// findDuplicateJSONKeys scans for duplicate object member names at any depth.
// Duplicate names are non-interoperable and cannot be detected after decoding
// into a map.
func findDuplicateJSONKeys(data []byte) error {
	dec := stdjson.NewDecoder(bytes.NewReader(data))

	type frame struct {
		object  bool
		keys    map[string]struct{}
		wantKey bool
	}
	var stack []frame

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			if len(stack) != 0 {
				return errors.New("unexpected EOF")
			}
			return nil
		}
		if err != nil {
			return err
		}

		if delim, ok := tok.(stdjson.Delim); ok {
			switch delim {
			case '{':
				stack = append(stack, frame{
					object:  true,
					keys:    make(map[string]struct{}),
					wantKey: true,
				})
			case '[':
				stack = append(stack, frame{})
			case '}', ']':
				if len(stack) == 0 {
					return errors.New("unbalanced closing delimiter")
				}
				stack = stack[:len(stack)-1]
				if n := len(stack); n > 0 && stack[n-1].object {
					stack[n-1].wantKey = true
				}
			}
			continue
		}

		if n := len(stack); n > 0 && stack[n-1].object {
			top := &stack[n-1]
			if top.wantKey {
				key, ok := tok.(string)
				if !ok {
					return fmt.Errorf("object key has type %T, want string", tok)
				}
				if _, exists := top.keys[key]; exists {
					return fmt.Errorf("duplicate object key %q", key)
				}
				top.keys[key] = struct{}{}
				top.wantKey = false
			} else {
				top.wantKey = true
			}
		}
	}
}

// checkReservedHALFields reports a conflict only when a reserved name appears
// as a top-level object key. Nested state fields remain ordinary JSON.
func checkReservedHALFields(data []byte) error {
	// Cheap pre-check before allocating a token scanner.
	if !bytes.Contains(data, []byte("_links")) && !bytes.Contains(data, []byte("_embedded")) {
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
		keyToken, err := dec.Token()
		if err != nil {
			return fmt.Errorf("hal: inspect data object key: %w", err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("hal: data object key has type %T, want string", keyToken)
		}
		if key == "_links" || key == "_embedded" {
			return fmt.Errorf("%w: %q", ErrHALFieldConflict, key)
		}

		var value stdjson.RawMessage
		if err := dec.Decode(&value); err != nil {
			return fmt.Errorf("hal: inspect data object value: %w", err)
		}
	}
	return nil
}

func (e *Envelope) marshalMeta() ([]byte, error) {
	// Build a local view instead of mutating e.links: an Envelope may be
	// marshaled more than once, including concurrently.
	links := e.links
	if e.instance != nil {
		if curies := e.instance.resolveCuries(e.links); len(curies) > 0 {
			links = make(map[string]any, len(e.links)+1)
			for rel, value := range e.links {
				links[rel] = value
			}
			links["curies"] = mergeCuries(links["curies"], curies)
		}
	}

	// Fast path: no links
	if len(links) == 0 {
		if e.embedded != nil && len(e.embedded) > 0 {
			return json.Marshal(map[string]any{"_embedded": e.embedded})
		}
		return nil, nil
	}

	// Only create map if needed
	if e.embedded != nil && len(e.embedded) > 0 {
		return json.Marshal(map[string]any{
			"_links":    links,
			"_embedded": e.embedded,
		})
	}

	return json.Marshal(map[string]any{"_links": links})
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
		if t.Kind() == reflect.Struct || (t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct) {
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
