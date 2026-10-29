// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

type nilDataUser struct {
	ID int `json:"id"`
}

func nilDataCases() []struct {
	name string
	data any
} {
	return []struct {
		name string
		data any
	}{
		{name: "nil interface", data: nil},
		{name: "typed nil pointer", data: (*nilDataUser)(nil)},
		{name: "nil map", data: map[string]any(nil)},
		{name: "nil slice", data: []int(nil)},
		{name: "raw null", data: json.RawMessage("null")},
	}
}

func TestNilDataContract_WithoutLinks(t *testing.T) {
	for _, tc := range nilDataCases() {
		t.Run(tc.name, func(t *testing.T) {
			env := New().WrapRaw(tc.data)
			b, err := json.Marshal(env)
			if err != nil {
				t.Fatal(err)
			}
			assertNoDuplicateKeys(t, b)
			assertJSONEqual(t, b, []byte(`{}`))
		})
	}
}

func TestNilDataContract_WithLinks(t *testing.T) {
	for _, tc := range nilDataCases() {
		t.Run(tc.name, func(t *testing.T) {
			env := New().WrapRaw(tc.data)
			env.AddLink(Link{Rel: "self", Href: "/x"})

			b, err := json.Marshal(env)
			if err != nil {
				t.Fatal(err)
			}
			assertNoDuplicateKeys(t, b)
			assertJSONEqual(t, b, []byte(`{"_links":{"self":{"href":"/x"}}}`))
		})
	}
}

func TestNilDataContract_Precomputed(t *testing.T) {
	links := []byte(`{"self":{"href":"/x"}}`)
	want := []byte(`{"_links":{"self":{"href":"/x"}}}`)

	for _, tc := range nilDataCases() {
		t.Run(tc.name, func(t *testing.T) {
			env := New().WrapPrecomputed(context.Background(), tc.data, links)
			b, err := json.Marshal(env)
			if err != nil {
				t.Fatal(err)
			}
			assertNoDuplicateKeys(t, b)
			assertJSONEqual(t, b, want)
		})
	}
}

func TestNilDataContract_EmptyObject(t *testing.T) {
	env := New().WrapRaw(map[string]any{})
	env.AddLink(Link{Rel: "self", Href: "/x"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, b, []byte(`{"_links":{"self":{"href":"/x"}}}`))
}
