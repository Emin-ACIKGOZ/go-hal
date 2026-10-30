// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestLinkPolicy_DefaultEmitsPermissiveValues(t *testing.T) {
	t.Parallel()

	links := []Link{
		{Rel: "empty", Href: ""},
		{Rel: "space", Href: "   "},
		{Rel: "invalid", Href: "not a uri at all"},
		{Rel: "bad-type", Href: "/x", Type: "not a media type"},
		{Rel: "range", Href: "/x", Type: "application/*"},
		{Rel: "crlf", Href: "/x", Type: "application/json\n"},
		{Rel: "relative-deprecation", Href: "/x", Deprecation: "/relative"},
		{Rel: "relative-profile", Href: "/x", Profile: "relative/path"},
		{Rel: "bad-language", Href: "/x", HrefLang: "en_US"},
	}
	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	for _, link := range links {
		env.AddLink(link)
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("default mode rejected a permissive link: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	for _, link := range links {
		objects := relationObjects(t, root, link.Rel)
		if len(objects) != 1 {
			t.Fatalf("%s = %#v, want one link", link.Rel, objects)
		}
		if got := objects[0]["href"]; got != link.Href {
			t.Fatalf("%s href = %#v, want %q", link.Rel, got, link.Href)
		}
	}
}

func TestLinkPolicy_StrictRejectsHrefAuthoringBugs(t *testing.T) {
	t.Parallel()

	links := []Link{
		{Rel: "empty", Href: ""},
		{Rel: "space", Href: "   "},
		{Rel: "invalid", Href: "not a uri at all"},
		{Rel: "malformed-template", Href: "/x{?id"},
		{Rel: "reserved-operator", Href: "/x{=id}"},
		{Rel: "template-flag-on-literal", Href: "/x", Templated: true},
	}
	for _, link := range links {
		env := New(WithStrictLinks()).WrapRaw(&embeddedTestUser{ID: 1})
		env.AddLink(link)
		out, err := json.Marshal(env)
		if !errors.Is(err, ErrInvalidLink) {
			t.Fatalf("%s error = %v (output %s), want ErrInvalidLink", link.Rel, err, out)
		}
		if len(out) != 0 {
			t.Fatalf("%s returned partial output: %s", link.Rel, out)
		}
		if len(env.links) != 0 {
			t.Fatalf("%s mutated links: %v", link.Rel, env.links)
		}
	}
}

func TestLinkPolicy_StrictMediaTypes(t *testing.T) {
	t.Parallel()

	valid := []Link{
		{Href: "/x", Type: "application/json"},
		{Href: "/x", Type: "application/hal+json"},
		{Href: "/x", Type: "text/plain; charset=utf-8"},
	}
	for _, link := range valid {
		if err := ValidateLink(link); err != nil {
			t.Errorf("valid type %q rejected: %v", link.Type, err)
		}
	}

	invalid := []Link{
		{Href: "/x", Type: "*/*"},
		{Href: "/x", Type: "application/*"},
		{Href: "/x", Type: "*/json"},
		{Href: "/x", Type: "application"},
		{Href: "/x", Type: "application/json\n"},
		{Href: "/x", Type: "not a media type"},
	}
	for _, link := range invalid {
		if err := ValidateLink(link); !errors.Is(err, ErrInvalidLink) {
			t.Errorf("invalid type %q = %v, want ErrInvalidLink", link.Type, err)
		}
	}
}

func TestLinkPolicy_StrictDeprecationAndProfile(t *testing.T) {
	t.Parallel()

	valid := []Link{
		{Href: "/x", Deprecation: "/relative"},
		{Href: "/x", Deprecation: "https://example.com/deprecations/v1"},
		{Href: "/x", Profile: "https://example.com/profiles/order"},
		{Href: "/x", Profile: "urn:example:profile"},
		{Href: "/x", Profile: "mailto:user@example.com"},
		{Href: "/x", Profile: "tag:example.com,2026:profile"},
	}
	for _, link := range valid {
		if err := ValidateLink(link); err != nil {
			t.Errorf("valid deprecation/profile rejected: %#v: %v", link, err)
		}
	}

	invalid := []Link{
		{Href: "/x", Deprecation: "/foo bar"},
		{Href: "/x", Deprecation: "/foo\\bar"},
		{Href: "/x", Profile: "relative/path"},
		{Href: "/x", Profile: "/abs/path"},
		{Href: "/x", Profile: "//example.com/path"},
		{Href: "/x", Profile: "?query=only"},
		{Href: "/x", Profile: "#fragment"},
	}
	for _, link := range invalid {
		if err := ValidateLink(link); !errors.Is(err, ErrInvalidLink) {
			t.Errorf("invalid deprecation/profile %#v = %v, want ErrInvalidLink", link, err)
		}
	}
}

func TestLinkPolicy_StrictHreflangBoundaries(t *testing.T) {
	t.Parallel()

	valid := []string{"en", "en-US", "zh-Hant-TW", "a-b", "en-1", "x"}
	for _, tag := range valid {
		if err := ValidateLink(Link{Href: "/x", HrefLang: tag}); err != nil {
			t.Errorf("valid hreflang %q rejected: %v", tag, err)
		}
	}

	invalid := []string{"-en", "en-", "en_US", "englishlanguage", "en--US", ""}
	for _, tag := range invalid {
		err := ValidateLink(Link{Href: "/x", HrefLang: tag})
		if tag == "" {
			if err != nil {
				t.Errorf("absent hreflang rejected: %v", err)
			}
			continue
		}
		if !errors.Is(err, ErrInvalidLink) {
			t.Errorf("invalid hreflang %q = %v, want ErrInvalidLink", tag, err)
		}
	}
}

func TestLinkPolicy_DefaultEmptyHrefIsRFC3986Reference(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: ""})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("default empty href rejected: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if got := relationObjects(t, root, "self")[0]["href"]; got != "" {
		t.Fatalf("href = %#v, want empty string", got)
	}
}
