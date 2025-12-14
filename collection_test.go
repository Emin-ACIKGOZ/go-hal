// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCollection_Basic(t *testing.T) {
	type User struct {
		ID int
	}

	inst := New()
	items := []*User{{ID: 1}, {ID: 2}}

	page := inst.Collection(
		context.Background(),
		items,
		2,
		Link{Rel: "self", Href: "/users"},
	)

	if page.Count != 2 {
		t.Fatalf("expected count 2, got %d", page.Count)
	}

	if _, ok := page.Embedded["items"]; !ok {
		t.Fatal("expected embedded items")
	}

	if _, ok := page.Links["self"]; !ok {
		t.Fatal("expected self link")
	}
}

func TestCollection_NonSlicePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for non-slice input")
		}
	}()

	inst := New()
	inst.Collection(context.Background(), 123, 0, Link{})
}

func TestCollection_MarshalJSON_Recursive(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	inst := New()
	// Register generator for User so we expect links on individual items
	RegisterInstance(inst, func(ctx context.Context, u *User) []Link {
		return []Link{{Rel: "self", Href: "/user"}}
	})

	items := []*User{{ID: 1}}
	page := inst.Collection(context.Background(), items, 1, Link{Rel: "next", Href: "/next"})

	b, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Check for Collection Link
	if !contains(b, `"/next"`) {
		t.Fatal("collection missing 'next' link")
	}

	// Check for Item Link (Recursive Envelope)
	// We expect the item inside _embedded to have its own _links
	if !contains(b, `"/user"`) {
		t.Fatal("embedded item missing 'self' link (recursive marshaling failed)")
	}
}
