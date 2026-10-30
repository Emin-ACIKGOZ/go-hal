// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestAddLink_DefaultRelationHardening(t *testing.T) {
	t.Parallel()

	invalid := []string{"", "   ", "a b", "a\tb", "a\x00b", "a\xffb"}
	for _, rel := range invalid {
		t.Run(rel, func(t *testing.T) {
			env := New().WrapRaw(&embeddedTestUser{ID: 1})
			env.AddLink(Link{Rel: rel, Href: "/x"})

			b, err := json.Marshal(env)
			if !errors.Is(err, ErrInvalidRelation) {
				t.Fatalf("error = %v (output %s), want ErrInvalidRelation", err, b)
			}
			if len(b) != 0 {
				t.Fatalf("error returned partial output: %s", b)
			}
			if len(env.links) != 0 {
				t.Fatalf("invalid relation was added: %v", env.links)
			}
		})
	}
}

func TestAddLink_BareTokenCaseCollisionRejected(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: "/a"})
	env.AddLink(Link{Rel: "Self", Href: "/b"})

	_, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("error = %v, want ErrInvalidRelation", err)
	}
	if _, exists := env.links["Self"]; exists {
		t.Fatal("colliding relation was added")
	}
}

func TestAddLink_ExactDuplicateTokenIsMultiplicity(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: "/a"})
	env.AddLink(Link{Rel: "self", Href: "/b"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if got := relationObjects(t, root, "self"); len(got) != 2 {
		t.Fatalf("self = %#v, want 2 links", got)
	}
}

func TestAddLink_CuriesAndURIsAreCaseSensitive(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "acme:widget", Href: "/a"})
	env.AddLink(Link{Rel: "ACME:widget", Href: "/b"})
	env.AddLink(Link{Rel: "https://example.com/Rel", Href: "/c"})
	env.AddLink(Link{Rel: "https://example.com/rel", Href: "/d"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	links := root["_links"].(map[string]any)
	for _, rel := range []string{"acme:widget", "ACME:widget", "https://example.com/Rel", "https://example.com/rel"} {
		if _, exists := links[rel]; !exists {
			t.Fatalf("relation %q missing from %v", rel, links)
		}
	}
}

func TestRegisterStatic_RejectsRelationCollisions(t *testing.T) {
	t.Parallel()

	assertPanics(t, "hal: RegisterStatic:", func() {
		RegisterStatic(New(), &embeddedTestUser{}, []Link{
			{Rel: "self", Href: "/a"},
			{Rel: "SELF", Href: "/b"},
		})
	})
}

func TestRegisterStatic_RejectsControlRelation(t *testing.T) {
	t.Parallel()

	assertPanics(t, "hal: RegisterStatic:", func() {
		RegisterStatic(New(), &embeddedTestUser{}, []Link{
			{Rel: "a\x00b", Href: "/x"},
		})
	})
}

func TestPrecomputed_RejectsRelationCollisions(t *testing.T) {
	t.Parallel()

	invalid := [][]byte{
		[]byte(`{"self":{"href":"/a"},"Self":{"href":"/b"}}`),
		[]byte(`{"a\u0000b":{"href":"/x"}}`),
		[]byte(`{"":{"href":"/x"}}`),
		[]byte(`{"   ":{"href":"/x"}}`),
	}
	for _, links := range invalid {
		if _, err := CompilePrecomputedLinks(links); !errors.Is(err, ErrInvalidRelation) {
			t.Errorf("CompilePrecomputedLinks(%s) = %v, want ErrInvalidRelation", links, err)
		}
	}

	valid := []byte(`{"acme:x":{"href":"/a"},"ACME:x":{"href":"/b"}}`)
	if _, err := CompilePrecomputedLinks(valid); err != nil {
		t.Fatalf("case-sensitive CURIE relations rejected: %v", err)
	}
}
