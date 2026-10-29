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

	expected := `{"id":42,"name":"Alice","_links":{"self":{"href":"/users/42"}}}`
	if string(b) != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}
}

func TestWrapPrecomputed_AlreadyWrapped(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	inst := New()
	linksJSON := []byte(`{"_links":{"self":{"href":"/users/42"}}}`)

	env := inst.WrapPrecomputed(context.Background(), &User{ID: 42}, linksJSON)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	expected := `{"id":42,"_links":{"self":{"href":"/users/42"}}}`
	if string(b) != expected {
		t.Fatalf("expected %s, got %s", expected, b)
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

	expected := `{"_links":{"self":{"href":"/users/42"}}}`
	if string(b) != expected {
		t.Fatalf("expected %s, got %s", expected, b)
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

	if string(b) != `{"id":1}` {
		t.Fatalf("expected data in output, got %s", b)
	}
}
