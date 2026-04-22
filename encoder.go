// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	json "github.com/goccy/go-json"
)

// MarshalJSON implements the json.Marshaler interface.
// It serializes the wrapped Data and splices in the HAL "_links" and "_embedded"
// fields into the resulting JSON object.
func (e *Envelope) MarshalJSON() ([]byte, error) {
	// OPTIMIZATION: Fast path for pre-computed JSON
	if e.precomputedJSON != nil {
		if e.Data == nil {
			return e.precomputedJSON, nil
		}
		dataBytes, err := json.Marshal(e.Data)
		if err != nil {
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

	// 3. Prepare HAL metadata (_links, _embedded)
	metaBytes, err := e.marshalMeta()
	if err != nil {
		return nil, err
	}

	// 4. Combine
	return spliceJSON(dataBytes, metaBytes, isDataNull, isEmptyObj), nil
}

func splicePrecomputed(data, linksJSON []byte) []byte {
	// linksJSON is already {"_links":{...}}
	// We need to combine data and linksJSON
	if len(data) == 0 || string(data) == "{}" {
		return linksJSON
	}
	if len(linksJSON) == 0 {
		return data
	}
	// Remove trailing } from data and , from linksJSON
	result := make([]byte, 0, len(data)+len(linksJSON)-jsonTrailingChars)
	result = append(result, data[:len(data)-1]...)
	result = append(result, ',')
	result = append(result, linksJSON[1:]...)
	return result
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
	if len(b) == 0 {
		return true, false, nil
	}

	// Must start with '{'
	if b[0] != '{' {
		return false, false, errors.New("hal: data must be a JSON object to splice")
	}

	// Check for empty object "{}"
	if len(b) == 2 && b[1] == '}' {
		return false, true, nil
	}

	// Check for null - bytes comparison (no allocation)
	if len(b) >= 4 && b[0] == 'n' && b[1] == 'u' && b[2] == 'l' && b[3] == 'l' {
		return true, false, nil
	}

	return false, false, nil
}

func (e *Envelope) marshalMeta() ([]byte, error) {
	if e.instance != nil {
		if curies := e.instance.resolveCuries(e.links); len(curies) > 0 {
			e.addLinkRaw("curies", curies)
		}
	}

	// Fast path: no links
	if len(e.links) == 0 {
		if e.embedded != nil && len(e.embedded) > 0 {
			return json.Marshal(map[string]any{"_embedded": e.embedded})
		}
		return nil, nil
	}

	// Only create map if needed
	if e.embedded != nil && len(e.embedded) > 0 {
		return json.Marshal(map[string]any{
			"_links":    e.links,
			"_embedded": e.embedded,
		})
	}

	return json.Marshal(map[string]any{"_links": e.links})
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
	if existing, ok := e.links[rel]; ok {
		if slice, isSlice := existing.([]any); isSlice {
			e.links[rel] = append(slice, val)
		} else {
			e.links[rel] = []any{existing, val}
		}
	} else {
		e.links[rel] = val
	}
}
