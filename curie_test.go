// Copyright (c) 2025-2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCurie_BasicRegistration(t *testing.T) {
	type CurieUser struct {
		ID int `json:"id"`
	}

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")
	RegisterInstance(inst, func(_ context.Context, _ *CurieUser) []Link {
		return []Link{{Rel: "acme:widget", Href: "/x"}}
	})

	env := inst.Wrap(context.Background(), &CurieUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	links := result["_links"].(map[string]any)

	_, ok := links["curies"]
	if !ok {
		t.Fatal("curies not found")
	}
}

func TestCurie_MultiplePrefixes(t *testing.T) {
	type CurieUser struct {
		ID int `json:"id"`
	}

	inst := New()
	inst.RegisterCurie("v1", "https://api.example.com/v1/{rel}")
	inst.RegisterCurie("v2", "https://api.example.com/v2/{rel}")

	RegisterInstance(inst, func(_ context.Context, _ *CurieUser) []Link {
		return []Link{
			{Rel: "v1:order", Href: "/orders"},
			{Rel: "v2:order", Href: "/orders"},
		}
	})

	env := inst.Wrap(context.Background(), &CurieUser{})
	b, _ := json.Marshal(env)

	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	links := result["_links"].(map[string]any)

	curies := links["curies"].([]any)
	if len(curies) != 2 {
		t.Fatalf("expected 2 curies, got %d", len(curies))
	}
}