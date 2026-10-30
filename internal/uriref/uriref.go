// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

// Package uriref validates RFC 3986 URI references conservatively.
package uriref

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

// ErrInvalidURIReference identifies a rejected URI reference.
var ErrInvalidURIReference = errors.New("invalid URI reference")

// Validate reports whether ref is a safe absolute or relative URI reference.
// It accepts the reference forms used by HAL href, deprecation, and profile
// fields and rejects controls, spaces, non-ASCII bytes, malformed percent
// escapes, backslashes, and malformed authorities. It is intentionally a
// conservative component scan layered on net/url.Parse, not a full RFC 3986
// parser.
func Validate(ref string) error {
	if ref == "" {
		return fmt.Errorf("%w: empty reference", ErrInvalidURIReference)
	}
	if !utf8.ValidString(ref) {
		return fmt.Errorf("%w: invalid UTF-8", ErrInvalidURIReference)
	}
	if err := validateBytes(ref); err != nil {
		return err
	}
	if err := validateBracketsAndFragment(ref); err != nil {
		return err
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURIReference, err)
	}
	if err := validateAuthority(ref, parsed); err != nil {
		return err
	}
	return nil
}

// ValidateAbsolute reports whether ref is an absolute URI: a valid URI
// reference with a scheme. urn:, mailto:, tag:, and uppercase schemes are
// accepted; bare names, relative paths, network-path references, query-only and
// fragment-only references are rejected.
func ValidateAbsolute(ref string) error {
	if err := Validate(ref); err != nil {
		return err
	}
	if _, ok := schemeOf(ref); !ok {
		return fmt.Errorf("%w: not an absolute URI", ErrInvalidURIReference)
	}
	return nil
}

func validateBytes(ref string) error {
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		if c <= 0x20 || c >= 0x7f {
			return fmt.Errorf("%w: contains a control, space, or non-ASCII byte", ErrInvalidURIReference)
		}
		switch c {
		case '\\', '"', '<', '>', '^', '`', '{', '}', '|':
			return fmt.Errorf("%w: contains disallowed byte %q", ErrInvalidURIReference, c)
		case '%':
			if i+2 >= len(ref) || !isHex(ref[i+1]) || !isHex(ref[i+2]) {
				return fmt.Errorf("%w: malformed percent escape", ErrInvalidURIReference)
			}
			i += 2
		}
	}
	return nil
}

//nolint:cyclop // Explicit bracket/fragment checks keep the policy auditable.
func validateBracketsAndFragment(ref string) error {
	if strings.Count(ref, "#") > 1 {
		return fmt.Errorf("%w: multiple fragment delimiters", ErrInvalidURIReference)
	}
	start, end, hasAuthority := authorityBounds(ref)
	for i := 0; i < len(ref); i++ {
		if ref[i] != '[' && ref[i] != ']' {
			continue
		}
		if !hasAuthority || i < start || i >= end {
			return fmt.Errorf("%w: bracket outside authority", ErrInvalidURIReference)
		}
	}
	if hasAuthority {
		authority := ref[start:end]
		if strings.Count(authority, "[") > 1 || strings.Count(authority, "]") > 1 {
			return fmt.Errorf("%w: malformed IPv6 literal", ErrInvalidURIReference)
		}
		open := strings.IndexByte(authority, '[')
		closeIdx := strings.IndexByte(authority, ']')
		if (open >= 0) != (closeIdx >= 0) || open > closeIdx {
			return fmt.Errorf("%w: unmatched bracket in authority", ErrInvalidURIReference)
		}
	}
	return nil
}

func authorityBounds(ref string) (start, end int, ok bool) {
	if strings.HasPrefix(ref, "//") {
		start = len("//")
	} else {
		scheme, schemeOK := schemeOf(ref)
		if !schemeOK || !strings.HasPrefix(ref[len(scheme)+1:], "//") {
			return 0, 0, false
		}
		start = len(scheme) + len("://")
	}
	for i := start; i < len(ref); i++ {
		switch ref[i] {
		case '/', '?', '#':
			return start, i, true
		}
	}
	return start, len(ref), true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' ||
		c >= 'a' && c <= 'f' ||
		c >= 'A' && c <= 'F'
}

func validateAuthority(ref string, parsed *url.URL) error {
	if !hasAuthority(ref) {
		return nil
	}
	if parsed.Host == "" {
		return fmt.Errorf("%w: authority has no host", ErrInvalidURIReference)
	}
	return nil
}

func hasAuthority(ref string) bool {
	if strings.HasPrefix(ref, "//") {
		return true
	}
	scheme, ok := schemeOf(ref)
	if !ok {
		return false
	}
	return strings.HasPrefix(ref[len(scheme)+1:], "//")
}

//nolint:cyclop // Flat byte scanner is easier to audit than a table.
func schemeOf(ref string) (string, bool) {
	idx := strings.IndexByte(ref, ':')
	if idx <= 0 {
		return "", false
	}
	for i := 0; i < idx; i++ {
		c := ref[i]
		valid := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' ||
			(i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'))
		if !valid {
			return "", false
		}
	}
	return ref[:idx], true
}
