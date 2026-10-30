// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestAddCachedLink_AddsLinkAndEmbedded(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddCachedLink(
		Link{Rel: "author", Href: "/people/alan-watts"},
		&embeddedTestUser{ID: 2},
	)

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root := decodeJSONValue(t, b).(map[string]any)
	author := relationObjects(t, root, "author")
	if len(author) != 1 || author[0]["href"] != "/people/alan-watts" {
		t.Fatalf("author link = %#v", author)
	}
	if _, ok := root["_embedded"].(map[string]any)["author"].(map[string]any); !ok {
		t.Fatalf("author embedded resource missing from %s", b)
	}
}

func TestAddCachedLink_MatchesDraftCachePattern(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(map[string]any{"name": "The Way of Zen"})
	env.AddLink(Link{Rel: "self", Href: "/books/the-way-of-zen"})
	env.AddCachedLink(
		Link{Rel: "author", Href: "/people/alan-watts"},
		map[string]any{"name": "Alan Watts", "born": "January 6, 1915"},
	)

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)
	assertJSONEqual(t, b, []byte(`{
		"name":"The Way of Zen",
		"_links":{
			"self":{"href":"/books/the-way-of-zen"},
			"author":{"href":"/people/alan-watts"}
		},
		"_embedded":{
			"author":{"name":"Alan Watts","born":"January 6, 1915"}
		}
	}`))
}

func TestAddCachedLink_IsAtomicOnInvalidResource(t *testing.T) {
	t.Parallel()

	for _, resource := range []any{nil, "scalar", []byte(`{}`), json.RawMessage(`[]`)} {
		env := New().WrapRaw(&embeddedTestUser{ID: 1})
		env.AddCachedLink(Link{Rel: "author", Href: "/people/1"}, resource)

		_, err := json.Marshal(env)
		if !errors.Is(err, ErrInvalidEmbedded) {
			t.Fatalf("resource %T: error = %v, want ErrInvalidEmbedded", resource, err)
		}
		if _, exists := env.links["author"]; exists {
			t.Fatalf("resource %T: link was added despite failure", resource)
		}
		if _, exists := env.embedded["author"]; exists {
			t.Fatalf("resource %T: embedded value was added despite failure", resource)
		}
	}
}

func TestAddCachedLink_IsAtomicOnInvalidRelation(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddCachedLink(Link{Rel: " ", Href: "/x"}, &embeddedTestUser{ID: 2})

	_, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("error = %v, want ErrInvalidRelation", err)
	}
	if len(env.links) != 0 || len(env.embedded) != 0 {
		t.Fatalf("invalid relation mutated envelope: links=%v embedded=%v", env.links, env.embedded)
	}
}

func TestAddCachedLink_PrecomputedAddsNothing(t *testing.T) {
	t.Parallel()

	env := New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, &PrecomputedLinks{})
	env.AddCachedLink(Link{Rel: "author", Href: "/people/1"}, &embeddedTestUser{ID: 2})

	_, err := json.Marshal(env)
	if !errors.Is(err, ErrAddLinkPrecomputed) {
		t.Fatalf("error = %v, want ErrAddLinkPrecomputed", err)
	}
	if len(env.links) != 0 || len(env.embedded) != 0 {
		t.Fatalf("premanaged envelope mutated: links=%v embedded=%v", env.links, env.embedded)
	}
}

func TestAddCachedLink_EmptySliceAddsLinkOnly(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddCachedLink(Link{Rel: "item", Href: "/items"}, []*embeddedTestUser{})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if len(relationObjects(t, root, "item")) != 1 {
		t.Fatal("link was not added")
	}
	if _, exists := root["_embedded"]; exists {
		t.Fatal("empty slice produced _embedded")
	}
}
