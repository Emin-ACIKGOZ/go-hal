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

func (e *Envelope) MarshalJSON() ([]byte, error) {
	var dataBytes []byte
	var err error

	if e.Data != nil {
		dataBytes, err = json.Marshal(e.Data)
		if err != nil {
			return nil, err
		}
	}

	dataBytes = bytes.TrimSpace(dataBytes)
	isDataNull := len(dataBytes) == 0 || string(dataBytes) == "null"

	isDataEmptyObj := false
	if !isDataNull && len(dataBytes) >= 2 {
		if dataBytes[0] == '{' {
			rest := bytes.TrimSpace(dataBytes[1:])
			if len(rest) == 1 && rest[0] == '}' {
				isDataEmptyObj = true
			}
		} else {
			return nil, errors.New("hal: data must be a JSON object to splice")
		}
	}

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
		if isDataNull {
			return []byte("{}"), nil
		}
		return dataBytes, nil
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}

	if isDataNull || isDataEmptyObj {
		return metaBytes, nil
	}

	totalLen := (len(dataBytes) - 1) + 1 + (len(metaBytes) - 1)
	var buf bytes.Buffer
	buf.Grow(totalLen)

	buf.Write(dataBytes[:len(dataBytes)-1])
	buf.WriteByte(',')
	buf.Write(metaBytes[1:])

	return buf.Bytes(), nil
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
