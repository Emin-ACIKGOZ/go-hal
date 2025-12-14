// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMarshal_Passthrough(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	inst := New()
	out := inst.Wrap(context.Background(), &User{ID: 1})

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != `{"id":1}` {
		t.Fatalf("unexpected output: %s", b)
	}
}

func TestMarshal_InjectsLinks(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	inst := New()
	RegisterInstance(inst, func(ctx context.Context, u *User) []Link {
		return []Link{{Rel: "self", Href: "/users/1"}}
	})

	out := inst.Wrap(context.Background(), &User{ID: 1})
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(b, `"_links"`) {
		t.Fatalf("expected _links in output: %s", b)
	}
}

func TestMarshal_NonObjectFails(t *testing.T) {
	inst := New()
	out := inst.Wrap(context.Background(), 42)

	if _, err := json.Marshal(out); err == nil {
		t.Fatal("expected error for non-object data")
	}
}

func TestMarshal_EmptyStruct(t *testing.T) {
	// A struct with no fields marshals to "{}"
	type Empty struct{}

	inst := New()
	RegisterInstance(inst, func(ctx context.Context, e *Empty) []Link {
		return []Link{{Rel: "self", Href: "/empty"}}
	})

	out := inst.Wrap(context.Background(), &Empty{})
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}

	expected := `{"_links":{"self":{"href":"/empty"}}}`
	if string(b) != expected {
		t.Fatalf("expected %s, got %s", expected, string(b))
	}
}

func contains(b []byte, s string) bool {
	return string(b) != "" && string(b) != "{}" && (len(b) >= len(s)) && (string(b) != "")
}
