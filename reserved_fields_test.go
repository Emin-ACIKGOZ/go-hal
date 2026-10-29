// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type reservedLinksUser struct {
	ID    int            `json:"id"`
	Links map[string]any `json:"_links"`
}

type reservedEmbeddedUser struct {
	ID       int            `json:"id"`
	Embedded map[string]any `json:"_embedded"`
}

func TestReservedFieldsContract_RejectsTopLevelConflict(t *testing.T) {
	cases := []struct {
		name string
		data any
	}{
		{
			name: "struct _links",
			data: &reservedLinksUser{
				ID:    1,
				Links: map[string]any{"self": map[string]any{"href": "/x"}},
			},
		},
		{
			name: "struct _embedded",
			data: &reservedEmbeddedUser{
				ID:       1,
				Embedded: map[string]any{"items": []any{}},
			},
		},
		{
			name: "map _links",
			data: map[string]any{
				"id":     1,
				"_links": map[string]any{"self": map[string]any{"href": "/x"}},
			},
		},
		{
			name: "map _embedded",
			data: map[string]any{
				"id":        1,
				"_embedded": map[string]any{"items": []any{}},
			},
		},
		{
			name: "raw _links",
			data: json.RawMessage(`{"id":1,"_links":{"self":{"href":"/x"}}}`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(New().WrapRaw(tc.data))
			if err == nil {
				t.Fatalf("expected conflict error, got %s", b)
			}
			if !errors.Is(err, ErrHALFieldConflict) {
				t.Fatalf("error %v does not wrap ErrHALFieldConflict", err)
			}
			if len(b) != 0 {
				t.Fatalf("conflict returned partial output: %s", b)
			}
		})
	}
}

func TestReservedFieldsContract_RejectsEscapedKeys(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{name: "lowercase hex", data: `{"\u005flinks":{}}`},
		{name: "uppercase hex", data: `{"\u005Flinks":{}}`},
		{name: "escaped embedded", data: `{"\u005fembedded":{}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkReservedHALFields([]byte(tc.data)); !errors.Is(err, ErrHALFieldConflict) {
				t.Fatalf("checkReservedHALFields() = %v, want ErrHALFieldConflict", err)
			}

			b, err := json.Marshal(New().WrapRaw(json.RawMessage(tc.data)))
			if !errors.Is(err, ErrHALFieldConflict) {
				t.Fatalf("marshal error = %v (output %s), want ErrHALFieldConflict", err, b)
			}
			if len(b) != 0 {
				t.Fatalf("conflict returned partial output: %s", b)
			}
		})
	}
}

func TestReservedFieldsContract_AllowsNonConflictingEscapes(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{name: "nested reserved", data: `{"nested":{"\u005flinks":{}}}`},
		{name: "escaped backslash", data: `{"\\u005flinks":1}`},
		{name: "prefixed name", data: `{"x_links":1}`},
		{name: "string value", data: `{"value":"_links"}`},
		{name: "escaped string value", data: `{"value":"\u005flinks"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkReservedHALFields([]byte(tc.data)); err != nil {
				t.Fatalf("unexpected conflict: %v", err)
			}

			b, err := json.Marshal(New().WrapRaw(json.RawMessage(tc.data)))
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			assertJSONEqual(t, b, []byte(tc.data))
		})
	}
}

func TestReservedFieldsContract_RejectsTrailingData(t *testing.T) {
	err := checkReservedHALFields([]byte(`{"id":1}{"_links":{}}`))
	if err == nil {
		t.Fatal("expected trailing data error")
	}
	if errors.Is(err, ErrHALFieldConflict) {
		t.Fatalf("trailing data reported as field conflict: %v", err)
	}
}

func TestReservedFieldsContract_RejectsConflictWithPrecomputedLinks(t *testing.T) {
	data := map[string]any{
		"id":     1,
		"_links": map[string]any{"self": map[string]any{"href": "/data"}},
	}
	env := New().WrapPrecomputed(context.Background(), data, []byte(`{"self":{"href":"/links"}}`))

	b, err := json.Marshal(env)
	if err == nil {
		t.Fatalf("expected conflict error, got %s", b)
	}
	if !errors.Is(err, ErrHALFieldConflict) {
		t.Fatalf("error %v does not wrap ErrHALFieldConflict", err)
	}
	if len(b) != 0 {
		t.Fatalf("conflict returned partial output: %s", b)
	}
}

func TestReservedFieldsContract_AllowsNestedHALNames(t *testing.T) {
	data := map[string]any{
		"_foo": "ordinary state",
		"nested": map[string]any{
			"_links":    map[string]any{"self": map[string]any{"href": "/nested"}},
			"_embedded": map[string]any{"items": []any{}},
		},
	}

	env := New().WrapRaw(data)
	env.AddLink(Link{Rel: "self", Href: "/root"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("document is not an object")
	}
	if got := root["_foo"]; got != "ordinary state" {
		t.Errorf("_foo = %#v, want ordinary state", got)
	}
	nested, ok := root["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested has type %T, want object", root["nested"])
	}
	if _, ok := nested["_links"]; !ok {
		t.Error("nested _links state was dropped")
	}
	if _, ok := nested["_embedded"]; !ok {
		t.Error("nested _embedded state was dropped")
	}
	if _, ok := root["_links"].(map[string]any); !ok {
		t.Error("top-level _links relation was not injected")
	}
}
