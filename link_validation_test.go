// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"errors"
	"testing"
)

func TestValidateLinkHrefAcceptsURIsAndRelativeReferences(t *testing.T) {
	t.Parallel()

	for _, href := range []string{
		"https://example.com/orders/1",
		"/orders/1",
		"orders/1",
		"?page=2",
		"#fragment",
		"//example.com/path",
		"mailto:user@example.com",
		"urn:isbn:0451450523",
	} {
		if err := validateLinkHref(Link{Href: href}); err != nil {
			t.Errorf("validateLinkHref(%q) = %v, want nil", href, err)
		}
	}
}

func TestValidateLinkHrefRejectsUnsafeReferences(t *testing.T) {
	t.Parallel()

	for _, href := range []string{
		"",
		"   ",
		"https://exa mple.com",
		"/foo\\bar",
		"/foo%zz",
		"http://",
		"\x00",
		"http://example.com/\xff",
	} {
		if err := validateLinkHref(Link{Href: href}); !errors.Is(err, ErrInvalidLink) {
			t.Errorf("validateLinkHref(%q) = %v, want ErrInvalidLink", href, err)
		}
	}
}

func TestValidateLinkHrefTemplateMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		link  Link
		valid bool
	}{
		{name: "template true", link: Link{Href: "/orders{?id}", Templated: true}, valid: true},
		{name: "template false", link: Link{Href: "/orders{?id}"}, valid: false},
		{name: "uri true", link: Link{Href: "/orders/1", Templated: true}, valid: false},
		{name: "uri false", link: Link{Href: "/orders/1"}, valid: true},
		{name: "uri true explicit", link: Link{Href: "/orders/1", Templated: true}, valid: false},
		{name: "empty", link: Link{Href: "", Templated: false}, valid: false},
		{name: "whitespace", link: Link{Href: "  "}, valid: false},
		{name: "malformed template", link: Link{Href: "/orders{?id", Templated: true}, valid: false},
		{name: "unsupported operator", link: Link{Href: "/orders{=id}", Templated: true}, valid: false},
		{name: "percent braces are data", link: Link{Href: "/orders/%7Bid%7D"}, valid: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLinkHref(tc.link)
			if tc.valid && err != nil {
				t.Fatalf("valid link rejected: %v", err)
			}
			if !tc.valid && !errors.Is(err, ErrInvalidLink) {
				t.Fatalf("invalid link error = %v, want ErrInvalidLink", err)
			}
		})
	}
}

func TestValidateLinkOptionalFields(t *testing.T) {
	t.Parallel()

	valid := []Link{
		{Type: "application/hal+json"},
		{Type: "text/plain; charset=utf-8"},
		{Deprecation: "https://example.com/deprecations/v1"},
		{Deprecation: "/deprecated"},
		{Profile: "https://example.com/profiles/order"},
		{HrefLang: "en"},
		{HrefLang: "en-US"},
		{HrefLang: "zh-Hant-TW"},
		{Name: "anything goes", Title: "anything goes", Method: "WHATEVER"},
	}
	for i, link := range valid {
		if err := validateLinkOptional(link); err != nil {
			t.Errorf("valid optional fields %d rejected: %v", i, err)
		}
	}

	invalid := []Link{
		{Type: "not a media type"},
		{Type: "application"},
		{Type: "application/hal+json; bad"},
		{Deprecation: "/foo bar"},
		{Deprecation: "/foo\\bar"},
		{Profile: "http://"},
		{Profile: "/foo%zz"},
		{HrefLang: "-en"},
		{HrefLang: "en-"},
		{HrefLang: "englishlanguage"},
		{HrefLang: "en_US"},
	}
	for i, link := range invalid {
		if err := validateLinkOptional(link); !errors.Is(err, ErrInvalidLink) {
			t.Errorf("invalid optional fields %d = %v, want ErrInvalidLink", i, err)
		}
	}
}
