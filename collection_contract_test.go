// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

// Collection, count, total, and _embedded.items are a project convention, not
// part of the HAL specification. These tests pin that convention.

package hal

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
)

type collectionContractUser struct {
	ID int `json:"id"`
}

func newCollectionContractInstance(t *testing.T) *Instance {
	t.Helper()
	inst := New()
	RegisterInstance(inst, func(_ context.Context, u *collectionContractUser) []Link {
		return []Link{{Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)}}
	})
	return inst
}

func TestCollectionConvention_ShapeAndRecursiveLinks(t *testing.T) {
	inst := newCollectionContractInstance(t)
	page := inst.Collection(
		context.Background(),
		[]*collectionContractUser{{ID: 1}, {ID: 2}},
		10,
		Link{Rel: "self", Href: "/users"},
	)

	b, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("collection document is not an object")
	}
	requireExactKeys(t, rawObject(t, root), "_links", "_embedded", "count", "total")

	if got := fmt.Sprint(root["count"]); got != "2" {
		t.Errorf("count = %s, want 2", got)
	}
	if got := fmt.Sprint(root["total"]); got != "10" {
		t.Errorf("total = %s, want 10", got)
	}

	links, ok := root["_links"].(map[string]any)
	if !ok {
		t.Fatalf("_links has type %T, want object", root["_links"])
	}
	if _, exists := links["next"]; exists {
		t.Error("unexpected link relation next")
	}
	assertLinkRelationShapes(t, root)
	self := relationObjects(t, root, "self")
	if len(self) != 1 || self[0]["href"] != "/users" {
		t.Fatalf("self = %#v, want href /users", self)
	}

	embedded, ok := root["_embedded"].(map[string]any)
	if !ok {
		t.Fatalf("_embedded has type %T, want object", root["_embedded"])
	}
	items, ok := embedded["items"].([]any)
	if !ok {
		t.Fatalf("_embedded.items has type %T, want array", embedded["items"])
	}
	if len(items) != 2 {
		t.Fatalf("got %d embedded items, want 2", len(items))
	}
	for i, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("item %d has type %T, want object", i, raw)
		}
		itemLinks := relationObjects(t, item, "self")
		wantHref := fmt.Sprintf("/users/%d", i+1)
		if len(itemLinks) != 1 || itemLinks[0]["href"] != wantHref {
			t.Errorf("item %d self = %#v, want %q", i, itemLinks, wantHref)
		}
	}
}

func TestCollectionConvention_SliceVariants(t *testing.T) {
	valueItem := collectionContractUser{ID: 1}
	pointerItem := &collectionContractUser{ID: 1}
	var nilPointers []*collectionContractUser

	tests := []struct {
		name          string
		items         any
		wantCount     int
		wantTotal     bool
		total         int
		wantItemLinks bool
	}{
		{
			name:      "typed nil pointer slice",
			items:     nilPointers,
			wantCount: 0,
		},
		{
			name:      "empty pointer slice",
			items:     []*collectionContractUser{},
			wantCount: 0,
		},
		{
			name:          "pointer slice gets recursive links",
			items:         []*collectionContractUser{pointerItem},
			wantCount:     1,
			wantItemLinks: true,
		},
		{
			name:      "value slice does not match pointer generator",
			items:     []collectionContractUser{valueItem},
			wantCount: 1,
		},
		{
			name:          "zero total is omitted",
			items:         []*collectionContractUser{pointerItem},
			wantCount:     1,
			total:         0,
			wantItemLinks: true,
		},
		{
			name:          "non-zero total is present",
			items:         []*collectionContractUser{pointerItem},
			wantCount:     1,
			total:         5,
			wantTotal:     true,
			wantItemLinks: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inst := newCollectionContractInstance(t)
			page := inst.Collection(context.Background(), tc.items, tc.total, Link{Rel: "self", Href: "/users"})

			b, err := json.Marshal(page)
			if err != nil {
				t.Fatal(err)
			}
			assertNoDuplicateKeys(t, b)

			root, ok := decodeJSONValue(t, b).(map[string]any)
			if !ok {
				t.Fatal("collection document is not an object")
			}
			if got := fmt.Sprint(root["count"]); got != strconv.Itoa(tc.wantCount) {
				t.Errorf("count = %s, want %d", got, tc.wantCount)
			}
			if _, exists := root["total"]; exists != tc.wantTotal {
				t.Errorf("total present = %t, want %t", exists, tc.wantTotal)
			}

			embedded, ok := root["_embedded"].(map[string]any)
			if !ok {
				t.Fatalf("_embedded has type %T, want object", root["_embedded"])
			}
			items, ok := embedded["items"].([]any)
			if !ok {
				t.Fatalf("_embedded.items has type %T, want array", embedded["items"])
			}
			if len(items) != tc.wantCount {
				t.Fatalf("got %d embedded items, want %d", len(items), tc.wantCount)
			}
			if tc.wantCount == 0 {
				return
			}

			item, ok := items[0].(map[string]any)
			if !ok {
				t.Fatalf("item has type %T, want object", items[0])
			}
			_, hasLinks := item["_links"]
			if hasLinks != tc.wantItemLinks {
				t.Errorf("item _links present = %t, want %t", hasLinks, tc.wantItemLinks)
			}
		})
	}
}

func TestCollectionConvention_SelfLinkRelIsIgnored(t *testing.T) {
	inst := newCollectionContractInstance(t)
	page := inst.Collection(
		context.Background(),
		[]*collectionContractUser{{ID: 1}},
		1,
		Link{Rel: "next", Href: "/users?page=1"},
	)

	b, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("collection document is not an object")
	}
	links, ok := root["_links"].(map[string]any)
	if !ok {
		t.Fatalf("_links has type %T, want object", root["_links"])
	}
	if _, exists := links["next"]; exists {
		t.Fatal("selfLink.Rel was used as the link relation")
	}
	self := relationObjects(t, root, "self")
	if len(self) != 1 || self[0]["href"] != "/users?page=1" {
		t.Fatalf("self = %#v, want href /users?page=1", self)
	}
}

func TestCollectionConvention_OmitsEmptyContainers(t *testing.T) {
	tests := []struct {
		name string
		page CollectionPage
		want []byte
	}{
		{
			name: "zero value",
			page: CollectionPage{},
			want: []byte(`{"count":0}`),
		},
		{
			name: "empty maps",
			page: CollectionPage{Links: map[string]any{}, Embedded: map[string]any{}},
			want: []byte(`{"count":0}`),
		},
		{
			name: "links only",
			page: CollectionPage{Links: map[string]any{"self": Link{Rel: "self", Href: "/x"}}},
			want: []byte(`{"_links":{"self":{"href":"/x"}},"count":0}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.page)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, b, tc.want)
		})
	}
}

func TestCollectionConvention_NonSlicePanics(t *testing.T) {
	tests := []struct {
		name  string
		items any
	}{
		{name: "untyped nil", items: nil},
		{name: "integer", items: 123},
		{name: "string", items: "users"},
		{name: "map", items: map[string]any{"id": 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inst := New()
			assertPanics(t, "hal: Collection items must be a slice", func() {
				inst.Collection(context.Background(), tc.items, 0, Link{Rel: "self", Href: "/users"})
			})
		})
	}
}
