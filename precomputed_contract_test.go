// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func marshalPrecomputed(t *testing.T, data any, links []byte) ([]byte, error) {
	t.Helper()
	env := New().WrapPrecomputed(context.Background(), data, links)
	return json.Marshal(env)
}

func TestPrecomputedContract_AcceptsRawAndWrappedForms(t *testing.T) {
	data := map[string]any{"id": 1}
	want := []byte(`{"id":1,"_links":{"self":{"href":"/x"}}}`)

	forms := [][]byte{
		[]byte(`{"self":{"href":"/x"}}`),
		[]byte(`{"_links":{"self":{"href":"/x"}}}`),
	}
	for _, links := range forms {
		t.Run(string(links), func(t *testing.T) {
			b, err := marshalPrecomputed(t, data, links)
			if err != nil {
				t.Fatal(err)
			}
			assertNoDuplicateKeys(t, b)
			assertJSONEqual(t, b, want)
		})
	}
}

func TestPrecomputedContract_EmptyObjectLinks(t *testing.T) {
	b, err := marshalPrecomputed(t, map[string]any{"id": 1}, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("document is not an object")
	}
	links, ok := root["_links"].(map[string]any)
	if !ok {
		t.Fatalf("_links has type %T, want empty object", root["_links"])
	}
	if len(links) != 0 {
		t.Fatalf("_links has %d entries, want 0", len(links))
	}
}

func TestPrecomputedContract_RejectsInvalidLinks(t *testing.T) {
	invalid := [][]byte{
		[]byte(`not json`),
		[]byte(`[]`),
		[]byte(`null`),
		[]byte(`"self"`),
		[]byte(`42`),
		[]byte(`true`),
		[]byte(`{"_links":[]}`),
		[]byte(`{"_links":null}`),
		[]byte(`{"_links":"self"}`),
		[]byte(`{"_links":42}`),
	}

	for _, links := range invalid {
		t.Run(string(links), func(t *testing.T) {
			b, err := marshalPrecomputed(t, map[string]any{"id": 1}, links)
			if err == nil {
				t.Fatalf("expected error for links %s, got %s", links, b)
			}
			if len(b) != 0 {
				t.Fatalf("error returned partial output: %s", b)
			}
			if !strings.Contains(err.Error(), "precomputed") {
				t.Fatalf("error %q does not identify precomputed links", err)
			}

			// Invalid links must fail even when there is no data to splice.
			b, err = marshalPrecomputed(t, nil, links)
			if err == nil {
				t.Fatalf("expected error for links %s with nil data, got %s", links, b)
			}
			if len(b) != 0 {
				t.Fatalf("error returned partial output: %s", b)
			}
		})
	}
}

func TestPrecomputedContract_RejectsNonObjectData(t *testing.T) {
	links := []byte(`{"self":{"href":"/x"}}`)
	dataCases := []struct {
		name string
		data any
	}{
		{name: "array", data: []int{1, 2}},
		{name: "string", data: "resource"},
		{name: "number", data: 42},
		{name: "bool", data: true},
	}

	for _, tc := range dataCases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := marshalPrecomputed(t, tc.data, links)
			if err == nil {
				t.Fatalf("expected error for %T data, got %s", tc.data, b)
			}
			if len(b) != 0 {
				t.Fatalf("error returned partial output: %s", b)
			}
		})
	}
}

func TestPrecomputedContract_NullDataUsesLinksOnly(t *testing.T) {
	links := []byte(`{"self":{"href":"/x"}}`)
	want := []byte(`{"_links":{"self":{"href":"/x"}}}`)

	dataCases := []struct {
		name string
		data any
	}{
		{name: "nil interface", data: nil},
		{name: "typed nil pointer", data: (*nilDataUser)(nil)},
		{name: "raw null", data: json.RawMessage("null")},
	}

	for _, tc := range dataCases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := marshalPrecomputed(t, tc.data, links)
			if err != nil {
				t.Fatal(err)
			}
			assertNoDuplicateKeys(t, b)
			assertJSONEqual(t, b, want)
		})
	}
}
