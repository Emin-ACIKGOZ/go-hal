// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		link  Link
		valid bool
	}{
		{name: "relative href", link: Link{Rel: "self", Href: "/users/1"}, valid: true},
		{name: "absolute href", link: Link{Rel: "self", Href: "https://example.com/1"}, valid: true},
		{name: "template", link: Link{Rel: "find", Href: "/orders{?id}", Templated: true}, valid: true},
		{name: "template without flag", link: Link{Rel: "find", Href: "/orders{?id}"}, valid: false},
		{name: "uri with flag", link: Link{Rel: "find", Href: "/orders", Templated: true}, valid: false},
		{name: "empty href", link: Link{Rel: "self", Href: " "}, valid: false},
		{name: "bad optional field", link: Link{Rel: "self", Href: "/x", Type: "nope"}, valid: false},
		{name: "bad hreflang", link: Link{Rel: "self", Href: "/x", HrefLang: "en_US"}, valid: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLink(tc.link)
			if tc.valid && err != nil {
				t.Fatalf("valid link rejected: %v", err)
			}
			if !tc.valid && !errors.Is(err, ErrInvalidLink) {
				t.Fatalf("error = %v, want ErrInvalidLink", err)
			}
		})
	}
}

func TestWithStrictLinks_AddLinkDefersInvalidLink(t *testing.T) {
	t.Parallel()

	env := New(WithStrictLinks()).WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: ""})

	b, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("error = %v (output %s), want ErrInvalidLink", err, b)
	}
	if len(b) != 0 {
		t.Fatalf("error returned partial output: %s", b)
	}
	if _, exists := env.links["self"]; exists {
		t.Fatal("invalid link was added")
	}
}

func TestWithStrictLinks_GeneratedLinkFailsMarshal(t *testing.T) {
	t.Parallel()

	inst := New(WithStrictLinks())
	RegisterInstance(inst, func(_ context.Context, _ *embeddedTestUser) []Link {
		return []Link{{Rel: "self", Href: "/ok"}, {Rel: "find", Href: "/x{?id}"}}
	})
	env := inst.Wrap(context.Background(), &embeddedTestUser{ID: 1})

	b, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("error = %v (output %s), want ErrInvalidLink", err, b)
	}
	if len(b) != 0 {
		t.Fatalf("error returned partial output: %s", b)
	}
}

func TestWithStrictLinks_AddCachedLinkIsAtomic(t *testing.T) {
	t.Parallel()

	env := New(WithStrictLinks()).WrapRaw(&embeddedTestUser{ID: 1})
	env.AddCachedLink(Link{Rel: "author", Href: "", Type: "bad"}, &embeddedTestUser{ID: 2})

	_, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("error = %v, want ErrInvalidLink", err)
	}
	if len(env.links) != 0 || len(env.embedded) != 0 {
		t.Fatalf("invalid cached link mutated envelope: links=%v embedded=%v", env.links, env.embedded)
	}
}

func TestWithStrictLinks_DoesNotChangeDefaults(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: "", Type: "not a media type"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, b, []byte(`{"id":1,"_links":{"self":{"href":"","type":"not a media type"}}}`))
}
