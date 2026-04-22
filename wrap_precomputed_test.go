// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

func TestWrapPrecomputed_Basic(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	inst := New()
	linksJSON := []byte(`{"self":{"href":"/users/42"}}`)

	env := inst.WrapPrecomputed(context.Background(), &User{ID: 42, Name: "Alice"}, linksJSON)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(b, `"id":42`) {
		t.Fatalf("expected id in output: %s", b)
	}
	if !contains(b, `"_links"`) {
		t.Fatalf("expected _links in output: %s", b)
	}
	if !contains(b, `"self"`) {
		t.Fatalf("expected self link in output: %s", b)
	}
}

func TestWrapPrecomputed_NilData(t *testing.T) {
	inst := New()
	linksJSON := []byte(`{"self":{"href":"/users/42"}}`)

	env := inst.WrapPrecomputed(context.Background(), nil, linksJSON)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != string(linksJSON) {
		t.Fatalf("expected just links for nil data: %s", b)
	}
}

func TestWrapPrecomputed_EmptyLinks(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	inst := New()

	env := inst.WrapPrecomputed(context.Background(), &User{ID: 1}, nil)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(b, `"id":1`) {
		t.Fatalf("expected data in output: %s", b)
	}
}
