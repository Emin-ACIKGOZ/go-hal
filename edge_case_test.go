// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

const wantUserID1 = `{"id":1}`

func TestMarshal_NilData(t *testing.T) {
	inst := New()
	env := inst.Wrap(context.Background(), nil)

	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != "{}" {
		t.Fatalf("expected {}, got %s", out)
	}
}

func TestMarshal_NilLinks(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	inst := New()
	RegisterInstance(inst, func(_ context.Context, _ *User) []Link {
		return nil
	})

	env := inst.Wrap(context.Background(), &User{ID: 1})
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != wantUserID1 {
		t.Fatalf("expected simple output, got %s", out)
	}
}

func TestMarshal_EmptyLinksSlice(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	inst := New()
	RegisterInstance(inst, func(_ context.Context, _ *User) []Link {
		return []Link{}
	})

	env := inst.Wrap(context.Background(), &User{ID: 1})
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != wantUserID1 {
		t.Fatalf("expected no links for empty slice, got %s", out)
	}
}

func TestAddLink_NilMap(t *testing.T) {
	e := &Envelope{
		Data:     nil,
		instance: New(),
		links:    nil,
	}

	e.AddLink(Link{Rel: "self", Href: "/test"})

	if e.links == nil {
		t.Fatal("links map should have been created")
	}

	if e.links["self"] == nil {
		t.Fatal("self link should exist")
	}
}

func TestAddLink_MultipleRelocations(t *testing.T) {
	e := &Envelope{
		Data:     nil,
		instance: New(),
		links:    make(map[string]any),
	}

	e.AddLink(Link{Rel: "self", Href: "/a"})
	e.AddLink(Link{Rel: "self", Href: "/b"})
	e.AddLink(Link{Rel: "self", Href: "/c"})

	v := e.links["self"]
	slice, ok := v.([]any)
	if !ok {
		t.Fatal("expected slice for multiple same rels")
	}

	if len(slice) != 3 {
		t.Fatalf("expected 3 links, got %d", len(slice))
	}
}

func TestMarshal_NilEmbedded(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	inst := New()
	RegisterInstance(inst, func(_ context.Context, _ *User) []Link {
		return []Link{{Rel: "self", Href: "/users/1"}}
	})

	env := inst.Wrap(context.Background(), &User{ID: 1})
	if env.embedded != nil {
		t.Fatal("embedded should be nil initially")
	}

	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if containsString(string(out), "_embedded") {
		t.Fatalf("expected no _embedded field, got %s", out)
	}
}

func TestMarshal_NilInstance(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	env := &Envelope{
		Data:     &User{ID: 1},
		instance: nil,
		links:    nil,
	}

	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != wantUserID1 {
		t.Fatalf("expected no links without instance, got %s", out)
	}
}

func TestWrap_ContextCancellation(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	inst := New()
	RegisterInstance(inst, func(ctx context.Context, _ *User) []Link {
		select {
		case <-ctx.Done():
			return []Link{{Rel: "canceled", Href: "/canceled"}}
		default:
			return []Link{{Rel: "self", Href: "/users"}}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	env := inst.Wrap(ctx, &User{ID: 1})
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if !containsString(string(out), "canceled") {
		t.Fatalf("expected canceled link, got %s", out)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}