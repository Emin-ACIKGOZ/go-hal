// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAddLink_Polymorphism(t *testing.T) {
	e := &Envelope{
		links: make(map[string]any),
	}

	e.AddLink(Link{Rel: "self", Href: "/a"})
	e.AddLink(Link{Rel: "self", Href: "/b"})

	v, ok := e.links["self"]
	if !ok {
		t.Fatal("missing self link")
	}

	if _, ok := v.([]any); !ok {
		t.Fatal("expected slice for duplicate rels")
	}
}

func TestAllocationEfficientHALAugmentationDuringJSONSerialization_AutoInjection(t *testing.T) {
	type User struct{}

	inst := New()
	inst.RegisterCurie("acme", "https://docs/{rel}")

	RegisterInstance(inst, func(ctx context.Context, u *User) []Link {
		return []Link{{Rel: "acme:test", Href: "/x"}}
	})

	env := inst.Wrap(context.Background(), &User{})

	// Trigger MarshalJSON to force curie resolution & injection
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	links, ok := decoded["_links"].(map[string]any)
	if !ok {
		t.Fatal("expected _links to exist")
	}

	if _, ok := links["curies"]; !ok {
		t.Fatal("expected curies to be injected")
	}
}
