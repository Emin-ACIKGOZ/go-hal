// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"encoding/json"
	"testing"
)

func embeddedRelationItems(t *testing.T, root map[string]any, rel string) []map[string]any {
	t.Helper()
	embedded, ok := root["_embedded"].(map[string]any)
	if !ok {
		t.Fatalf("_embedded has type %T, want object", root["_embedded"])
	}
	switch value := embedded[rel].(type) {
	case map[string]any:
		return []map[string]any{value}
	case []any:
		items := make([]map[string]any, 0, len(value))
		for i, item := range value {
			object, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("%s[%d] has type %T, want object", rel, i, item)
			}
			items = append(items, object)
		}
		return items
	default:
		t.Fatalf("%s has type %T, want object or array", rel, embedded[rel])
		return nil
	}
}

func assertEmbeddedIDs(t *testing.T, root map[string]any, rel string, want ...string) {
	t.Helper()
	items := embeddedRelationItems(t, root, rel)
	if len(items) != len(want) {
		t.Fatalf("%s has %d items, want %d: %#v", rel, len(items), len(want), items)
	}
	for i, item := range items {
		if got := item["id"]; got != want[i] {
			t.Errorf("%s[%d].id = %#v, want %q", rel, i, got, want[i])
		}
	}
}

type stringIDEmbedded struct {
	ID string `json:"id"`
}

func TestAddEmbedded_FlattensRepeatedMultiSlices(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("item", []*stringIDEmbedded{{ID: "a"}, {ID: "b"}})
	env.AddEmbedded("item", []*stringIDEmbedded{{ID: "c"}, {ID: "d"}})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)
	root := decodeJSONValue(t, b).(map[string]any)
	assertEmbeddedIDs(t, root, "item", "a", "b", "c", "d")
}

func TestAddEmbedded_FlattensObjectThenSlice(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("item", &stringIDEmbedded{ID: "a"})
	env.AddEmbedded("item", []*stringIDEmbedded{{ID: "b"}, {ID: "c"}})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	assertEmbeddedIDs(t, root, "item", "a", "b", "c")
}

func TestAddEmbedded_FlattensSliceThenOneElementSlice(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("item", []*stringIDEmbedded{{ID: "a"}, {ID: "b"}})
	env.AddEmbedded("item", []*stringIDEmbedded{{ID: "c"}})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	assertEmbeddedIDs(t, root, "item", "a", "b", "c")
}

func TestAddEmbedded_EmptySliceLeavesExistingValues(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("item", []*stringIDEmbedded{{ID: "a"}, {ID: "b"}})
	env.AddEmbedded("item", []*stringIDEmbedded{})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	assertEmbeddedIDs(t, root, "item", "a", "b")
}

func TestAddCachedLink_FlattensRepeatedMultiSlices(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddCachedLink(Link{Rel: "item", Href: "/items"}, []*stringIDEmbedded{{ID: "a"}, {ID: "b"}})
	env.AddCachedLink(Link{Rel: "item", Href: "/items"}, []*stringIDEmbedded{{ID: "c"}, {ID: "d"}})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	assertEmbeddedIDs(t, root, "item", "a", "b", "c", "d")
	links := relationObjects(t, root, "item")
	if len(links) != 2 {
		t.Fatalf("item links = %d, want 2 cached links", len(links))
	}
}
