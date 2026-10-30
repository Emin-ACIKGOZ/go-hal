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
		if err := validateLinkHref(href); err != nil {
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
		if err := validateLinkHref(href); !errors.Is(err, ErrInvalidLink) {
			t.Errorf("validateLinkHref(%q) = %v, want ErrInvalidLink", href, err)
		}
	}
}
