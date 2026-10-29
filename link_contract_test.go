// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

type linkContractUser struct {
	ID int `json:"id"`
}

func marshalLinks(t *testing.T, links ...Link) map[string]any {
	t.Helper()

	inst := New()
	RegisterInstance(inst, func(_ context.Context, _ *linkContractUser) []Link {
		return links
	})

	env := inst.Wrap(context.Background(), &linkContractUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	value := decodeJSONValue(t, b)
	root, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("document has type %T, want object", value)
	}
	return root
}

func assertLinkRelationShapes(t *testing.T, root map[string]any) {
	t.Helper()

	value, ok := root["_links"]
	if !ok {
		return
	}
	links, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("_links has type %T, want object", value)
	}

	for rel, raw := range links {
		switch v := raw.(type) {
		case map[string]any:
			if err := checkLinkHref(v); err != nil {
				t.Fatalf("relation %q: %v", rel, err)
			}
		case []any:
			for i, item := range v {
				link, ok := item.(map[string]any)
				if !ok {
					t.Fatalf("relation %q element %d has type %T, want object", rel, i, item)
				}
				if err := checkLinkHref(link); err != nil {
					t.Fatalf("relation %q element %d: %v", rel, i, err)
				}
			}
		default:
			t.Fatalf("relation %q has type %T, want object or array of objects", rel, raw)
		}
	}
}

func TestLinkContract_AllFields(t *testing.T) {
	root := marshalLinks(t, Link{
		Rel:         "alternate",
		Href:        "https://example.com/orders/1?lang=fr",
		Templated:   true,
		Type:        "application/hal+json",
		Deprecation: "https://example.com/deprecations/v1",
		Name:        "fr",
		Profile:     "https://example.com/profiles/order",
		Title:       "Commande",
		HrefLang:    "fr",
		Method:      "GET",
	})
	assertLinkRelationShapes(t, root)

	links := relationObjects(t, root, "alternate")
	if len(links) != 1 {
		t.Fatalf("got %d alternate links, want 1", len(links))
	}
	link := links[0]
	requireExactKeys(t, rawObject(t, link),
		"href", "templated", "type", "deprecation", "name", "profile", "title", "hreflang", "method",
	)

	want := map[string]any{
		"href":        "https://example.com/orders/1?lang=fr",
		"templated":   true,
		"type":        "application/hal+json",
		"deprecation": "https://example.com/deprecations/v1",
		"name":        "fr",
		"profile":     "https://example.com/profiles/order",
		"title":       "Commande",
		"hreflang":    "fr",
		"method":      "GET",
	}
	for key, wantValue := range want {
		if got := link[key]; got != wantValue {
			t.Errorf("link[%q] = %#v, want %#v", key, got, wantValue)
		}
	}
}

func TestLinkContract_HrefRequiredAndRelOmitted(t *testing.T) {
	root := marshalLinks(t, Link{Rel: "self", Href: ""})
	assertLinkRelationShapes(t, root)

	links := relationObjects(t, root, "self")
	if len(links) != 1 {
		t.Fatalf("got %d self links, want 1", len(links))
	}
	requireKeys(t, rawObject(t, links[0]), "href")
	forbidKeys(t, rawObject(t, links[0]), "Rel", "rel")
}

func TestLinkContract_TemplatedFalseOmitted(t *testing.T) {
	root := marshalLinks(t, Link{Rel: "self", Href: "/users/1", Templated: false})
	assertLinkRelationShapes(t, root)

	links := relationObjects(t, root, "self")
	if len(links) != 1 {
		t.Fatalf("got %d self links, want 1", len(links))
	}
	requireExactKeys(t, rawObject(t, links[0]), "href")
}

func TestLinkContract_TemplatedTruePresent(t *testing.T) {
	root := marshalLinks(t, Link{Rel: "find", Href: "/orders{?id}", Templated: true})
	assertLinkRelationShapes(t, root)

	links := relationObjects(t, root, "find")
	if len(links) != 1 {
		t.Fatalf("got %d find links, want 1", len(links))
	}
	requireKeys(t, rawObject(t, links[0]), "href", "templated")
	if got := links[0]["templated"]; got != true {
		t.Fatalf("templated = %#v, want true", got)
	}
}

func TestLinkContract_RelationMultiplicity(t *testing.T) {
	tests := []struct {
		name      string
		links     []Link
		rel       string
		wantHrefs []string
		wantArray bool
	}{
		{
			name:      "single relation is an object",
			links:     []Link{{Rel: "self", Href: "/users/1"}},
			rel:       "self",
			wantHrefs: []string{"/users/1"},
		},
		{
			name: "two links with same relation use an array",
			links: []Link{
				{Rel: "self", Href: "/users/1"},
				{Rel: "self", Href: "/users/1?page=2"},
			},
			rel:       "self",
			wantHrefs: []string{"/users/1", "/users/1?page=2"},
			wantArray: true,
		},
		{
			name: "three links preserve insertion order",
			links: []Link{
				{Rel: "item", Href: "/items/1"},
				{Rel: "item", Href: "/items/2"},
				{Rel: "item", Href: "/items/3"},
			},
			rel:       "item",
			wantHrefs: []string{"/items/1", "/items/2", "/items/3"},
			wantArray: true,
		},
		{
			name: "distinct relations remain separate objects",
			links: []Link{
				{Rel: "self", Href: "/users/1"},
				{Rel: "edit", Href: "/users/1/edit"},
			},
			rel:       "edit",
			wantHrefs: []string{"/users/1/edit"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := marshalLinks(t, tc.links...)
			assertLinkRelationShapes(t, root)

			linksValue, ok := root["_links"].(map[string]any)
			if !ok {
				t.Fatalf("_links has type %T, want object", root["_links"])
			}
			raw, ok := linksValue[tc.rel]
			if !ok {
				t.Fatalf("missing relation %q", tc.rel)
			}
			_, isArray := raw.([]any)
			if isArray != tc.wantArray {
				t.Fatalf("relation %q array form = %t, want %t", tc.rel, isArray, tc.wantArray)
			}

			links := relationObjects(t, root, tc.rel)
			if len(links) != len(tc.wantHrefs) {
				t.Fatalf("got %d links, want %d", len(links), len(tc.wantHrefs))
			}
			for i, wantHref := range tc.wantHrefs {
				if got := links[i]["href"]; got != wantHref {
					t.Errorf("links[%d].href = %#v, want %q", i, got, wantHref)
				}
			}
		})
	}
}

// rawObject re-encodes a decoded object so the RawMessage-based key helpers can
// be reused.
func rawObject(t *testing.T, obj map[string]any) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return decodeJSONObject(t, b)
}
