// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizeLinkDerivationMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		href       string
		templated  bool
		wantFlag   bool
		wantStrict bool // ValidateLink accepts the normalized link
	}{
		{name: "query expression", href: "/orders{?id}", wantFlag: true, wantStrict: true},
		{name: "reserved expression", href: "{+path}", wantFlag: true, wantStrict: true},
		{name: "two expressions absolute", href: "https://h/{a}{b}", wantFlag: true, wantStrict: true},
		{name: "embedded expression", href: "/a{b}c", wantFlag: true, wantStrict: true},
		{name: "fragment expression", href: "/orders{#frag}", wantFlag: true, wantStrict: true},
		{name: "percent braces are data", href: "/orders/%7Bid%7D", wantStrict: true},
		{name: "lowercase percent braces are data", href: "/orders/%7bid%7d", wantStrict: true},
		{name: "unclosed expression", href: "/orders{?id"},
		{name: "reserved operator", href: "/orders{=id}"},
		{name: "empty expression", href: "/orders{}"},
		{name: "malformed expression", href: "/orders{%zz}"},
		{name: "literal with flag", href: "/orders", templated: true, wantFlag: true},
		{name: "empty href", href: ""},
		{name: "whitespace href", href: "   "},
		{name: "literal space", href: "not a uri at all"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			in := Link{Rel: "find", Href: tc.href, Templated: tc.templated}
			got := normalizeLink(in)
			if got.Templated != tc.wantFlag {
				t.Fatalf("normalizeLink(%q, templated=%t).Templated = %t, want %t",
					tc.href, tc.templated, got.Templated, tc.wantFlag)
			}
			if in.Templated != tc.templated {
				t.Fatalf("normalizeLink mutated its input: %#v", in)
			}

			err := ValidateLink(in)
			if tc.wantStrict && err != nil {
				t.Fatalf("ValidateLink(%q, templated=%t) = %v, want nil", tc.href, tc.templated, err)
			}
			if !tc.wantStrict && !errors.Is(err, ErrInvalidLink) {
				t.Fatalf("ValidateLink(%q, templated=%t) = %v, want ErrInvalidLink", tc.href, tc.templated, err)
			}
		})
	}
}

func TestNormalizeLinkIsIdempotent(t *testing.T) {
	t.Parallel()

	for _, href := range []string{"/orders{?id}", "/orders", "/orders/%7Bid%7D", "/orders{=id}"} {
		first := normalizeLink(Link{Href: href})
		second := normalizeLink(first)
		if first != second {
			t.Errorf("normalizeLink(%q) is not idempotent: %#v then %#v", href, first, second)
		}
	}
}

func TestAddLinkDerivesTemplated(t *testing.T) {
	t.Parallel()

	for _, inst := range []*Instance{New(), New(WithStrictLinks())} {
		original := Link{Rel: "find", Href: "/orders{?id}"}
		env := inst.WrapRaw(&embeddedTestUser{ID: 1})
		env.AddLink(original)
		if original.Templated {
			t.Fatal("AddLink mutated caller's Link")
		}

		b, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("template link rejected: %v", err)
		}
		root := decodeJSONValue(t, b).(map[string]any)
		find := relationObjects(t, root, "find")
		if len(find) != 1 || find[0]["templated"] != true {
			t.Fatalf("find = %#v, want templated true", find)
		}
	}
}

func TestAddCachedLinkDerivesTemplated(t *testing.T) {
	t.Parallel()

	link := Link{Rel: "author", Href: "/people/{id}"}
	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddCachedLink(link, &embeddedTestUser{ID: 2})
	if link.Templated {
		t.Fatal("AddCachedLink mutated caller's Link")
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	author := relationObjects(t, root, "author")
	if len(author) != 1 || author[0]["templated"] != true {
		t.Fatalf("author = %#v, want templated true", author)
	}
}

func TestPrecomputedTemplateDerivationIsNotRewritten(t *testing.T) {
	t.Parallel()

	links := []byte(`{"find":{"href":"/x{?id}"}}`)
	handle, err := CompilePrecomputedLinks(links)
	if err != nil {
		t.Fatal(err)
	}
	env := New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, links) {
		t.Fatalf("precomputed bytes were rewritten: %s", b)
	}
	if bytes.Contains(b, []byte(`"templated"`)) {
		t.Fatalf("precomputed bytes gained templated: %s", b)
	}
}

func TestStrictPrecomputedTemplateStillRequiresExplicitFlag(t *testing.T) {
	t.Parallel()

	invalid := [][]byte{
		[]byte(`{"find":{"href":"/x{?id}"}}`),
	}
	for _, links := range invalid {
		if _, err := CompilePrecomputedLinksStrict(links); !errors.Is(err, ErrInvalidLink) {
			t.Errorf("CompilePrecomputedLinksStrict(%s) = %v, want ErrInvalidLink", links, err)
		}
	}

	curies := []byte(`{"curies":[{"name":"acme","href":"https://docs.example/{rel}"}]}`)
	env := New(WithStrictLinks()).WrapPrecomputed(context.Background(), &embeddedTestUser{ID: 1}, curies)
	if _, err := json.Marshal(env); !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("strict links accepted an underived precomputed curie template: %v", err)
	}

	env = New(WithStrictLinks()).WrapPrecomputed(
		context.Background(),
		&embeddedTestUser{ID: 1},
		[]byte(`{"find":{"href":"/x{?id}"}}`),
	)
	if _, err := json.Marshal(env); !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("strict instance accepted an underived precomputed template: %v", err)
	}
}

func BenchmarkAddLink_Plain(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		env := New().WrapRaw(&embeddedTestUser{ID: 1})
		env.AddLink(Link{Rel: "self", Href: "/users/42"})
	}
}

func BenchmarkAddLink_Templated(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		env := New().WrapRaw(&embeddedTestUser{ID: 1})
		env.AddLink(Link{Rel: "find", Href: "/orders{?id}"})
	}
}
