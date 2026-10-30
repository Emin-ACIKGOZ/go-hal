// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package uriref

import (
	"errors"
	"testing"
)

func TestValidateAcceptsRFC3986Examples(t *testing.T) {
	t.Parallel()
	valid := []string{
		"http://example.com",
		"http://example.com/",
		"https://example.com/orders/523?page=2#frag",
		"ftp://ftp.is.co.za/rfc/rfc1808.txt",
		"http://www.ietf.org/rfc/rfc2396.txt",
		"ldap://[2001:db8::7]/c=GB?objectClass?one",
		"mailto:John.Doe@example.com",
		"news:comp.infosystems.www.servers.unix",
		"tel:+1-816-555-1212",
		"telnet://192.0.2.16:80/",
		"urn:oasis:names:specification:docbook:dtd:xml:4.1.2",
		"/relative/path",
		"relative/path",
		"./a/b",
		"../a/b",
		"?query=only",
		"#fragment",
		"//example.com/path",
		"http://example.com/a%20b",
		"http://[::1]/",
	}
	for _, ref := range valid {
		if err := Validate(ref); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", ref, err)
		}
	}
}

func TestValidateRejectsUnsafeReferences(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"",
		"http://exa mple.com",
		"/foo bar",
		"/foo\\bar",
		"http://example.com/%zz",
		"http://example.com/%2",
		"http://example.com/a{b}",
		"http://",
		"http:///path",
		"//",
		"http://[::1",
		"\x00",
		"http://example.com/\x7f",
		"http://example.com/受",
		"http://example.com/\"x\"",
		"http://example.com/|",
	}
	for _, ref := range invalid {
		if err := Validate(ref); !errors.Is(err, ErrInvalidURIReference) {
			t.Errorf("Validate(%q) = %v, want ErrInvalidURIReference", ref, err)
		}
	}
}

func TestValidateBracketsAndFragments(t *testing.T) {
	t.Parallel()

	valid := []string{
		"http://[::1]/",
		"//[::1]/x",
		"http://user@[::1]:8080/path",
		"/a%5Bb",
		"/a%23b",
		"/a#b",
		"https://example.com/a%5Bb?q=%23#frag",
	}
	for _, ref := range valid {
		if err := Validate(ref); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", ref, err)
		}
	}

	invalid := []string{
		"/a[b",
		"/a]b",
		"//host/x[",
		"http://[::1]/[x]",
		"/a#b#c",
		"https://example.com/#a#b",
		"http://[::1",
		"http://::1]",
	}
	for _, ref := range invalid {
		if err := Validate(ref); !errors.Is(err, ErrInvalidURIReference) {
			t.Errorf("Validate(%q) = %v, want ErrInvalidURIReference", ref, err)
		}
	}
}
