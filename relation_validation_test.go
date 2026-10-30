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

func TestAddLink_RelativeReferencesDoNotCollide(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	relations := []string{"a/b", "A/B", "?query", "?QUERY", "#frag", "#FRAG"}
	for _, rel := range relations {
		env.AddLink(Link{Rel: rel, Href: "/x"})
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("relative reference collision: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	links := root["_links"].(map[string]any)
	for _, rel := range relations {
		if _, exists := links[rel]; !exists {
			t.Fatalf("relation %q missing from %v", rel, links)
		}
	}
}

func TestPrecomputed_RelativeReferencesDoNotCollide(t *testing.T) {
	t.Parallel()

	links := []byte(`{
		"a/b":{"href":"/a"},
		"A/B":{"href":"/b"},
		"?query":{"href":"/q"},
		"?QUERY":{"href":"/Q"},
		"#frag":{"href":"/f"},
		"#FRAG":{"href":"/F"}
	}`)
	handle, err := CompilePrecomputedLinks(links)
	if err != nil {
		t.Fatalf("relative reference collision: %v", err)
	}
	b, err := json.Marshal(New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) {
		t.Fatalf("invalid JSON: %s", b)
	}
}

func TestAddEmbedded_RelativeReferencesDoNotCollide(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("a/b", &embeddedTestUser{ID: 1})
	env.AddEmbedded("A/B", &embeddedTestUser{ID: 2})
	env.AddEmbedded("?query", &embeddedTestUser{ID: 3})
	env.AddEmbedded("?QUERY", &embeddedTestUser{ID: 4})
	env.AddEmbedded("#frag", &embeddedTestUser{ID: 5})
	env.AddEmbedded("#FRAG", &embeddedTestUser{ID: 6})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("relative reference collision: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	embedded := root["_embedded"].(map[string]any)
	for _, rel := range []string{"a/b", "A/B", "?query", "?QUERY", "#frag", "#FRAG"} {
		if _, exists := embedded[rel]; !exists {
			t.Fatalf("embedded relation %q missing from %v", rel, embedded)
		}
	}
}

func TestAddLink_UnderscoreTokensDoNotCollide(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "a_b", Href: "/a"})
	env.AddLink(Link{Rel: "A_B", Href: "/b"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("underscore relations collided: %v", err)
	}
	links := decodeJSONValue(t, b).(map[string]any)["_links"].(map[string]any)
	for _, rel := range []string{"a_b", "A_B"} {
		if _, exists := links[rel]; !exists {
			t.Fatalf("relation %q missing from %v", rel, links)
		}
	}
}

func TestAddLink_BareTokenCollisionStillRejected(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: "/a"})
	env.AddLink(Link{Rel: "Self", Href: "/b"})

	if _, err := json.Marshal(env); !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("error = %v, want ErrInvalidRelation", err)
	}
}

func TestAddLink_LargeRelationSetUsesIndexedCollisionChecks(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	for i := 0; i < 12; i++ {
		env.AddLink(Link{Rel: "rel" + string(rune('a'+i)), Href: "/x"})
	}
	env.AddLink(Link{Rel: "SELF", Href: "/a"})
	env.AddLink(Link{Rel: "self", Href: "/b"})

	if _, err := json.Marshal(env); !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("error = %v, want ErrInvalidRelation", err)
	}
}

func TestAddLink_LargeDistinctRelationSetAccepted(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	for i := 0; i < 16; i++ {
		env.AddLink(Link{Rel: "rel" + string(rune('a'+i)), Href: "/x"})
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("distinct relations rejected: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if got := len(root["_links"].(map[string]any)); got != 16 {
		t.Fatalf("_links has %d entries, want 16", got)
	}
}
