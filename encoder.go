// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// MarshalJSON implements the json.Marshaler interface.
// It serializes the wrapped Data and splices in the HAL "_links" and "_embedded"
// fields into the resulting JSON object.
func (e *Envelope) MarshalJSON() ([]byte, error) {
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

func (e *Envelope) marshalData() ([]byte, error) {
	if e.Data == nil {
		return nil, nil
	}
	b, err := json.Marshal(e.Data)
	if err != nil {
		return nil, err
	}
	return bytes.TrimSpace(b), nil
}

// checkJSONStructure returns (isNull, isEmptyObject, error).
func checkJSONStructure(b []byte) (bool, bool, error) {
	if len(b) == 0 || string(b) == "null" {
		return true, false, nil
	}

	// Must start with '{'
	if b[0] != '{' {
		return false, false, errors.New("hal: data must be a JSON object to splice")
	}

	// Check for empty object "{}"
	// Since we trimmed in marshalData, "{}" should be exactly length 2.
	if len(b) == 2 && b[1] == '}' {
		return false, true, nil
	}

	return false, false, nil
}

func (e *Envelope) marshalMeta() ([]byte, error) {
	if curies := e.instance.resolveCuries(e.links); len(curies) > 0 {
		e.addLinkRaw("curies", curies)
	}

	meta := make(map[string]any)
	if len(e.links) > 0 {
		meta["_links"] = e.links
	}
	if len(e.embedded) > 0 {
		meta["_embedded"] = e.embedded
	}

	if len(meta) == 0 {
		return nil, nil
	}
	return json.Marshal(meta)
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

	// Splicing: remove closing brace of data, add comma, remove opening brace of meta
	totalLen := (len(data) - 1) + 1 + (len(meta) - 1)
	var buf bytes.Buffer
	buf.Grow(totalLen)

	buf.Write(data[:len(data)-1])
	buf.WriteByte(',')
	buf.Write(meta[1:])

	return buf.Bytes()
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
