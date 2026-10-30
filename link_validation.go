// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"errors"
	"fmt"
	"mime"
	"strings"

	"github.com/Emin-ACIKGOZ/go-hal/internal/uriref"
	"github.com/Emin-ACIKGOZ/go-hal/internal/uritemplate"
)

// ErrInvalidLink is returned when a Link fails validation. By default links are
// permissive; ValidateLink and instances configured with WithStrictLinks apply
// the full checks.
var ErrInvalidLink = errors.New("hal: invalid link")

// ValidateLink validates a Link Object without requiring a relation namespace:
// href URI/template consistency and the optional type, deprecation, profile,
// and hreflang fields. Rel, name, title, and method are not validated.
func ValidateLink(l Link) error {
	if err := validateLinkHref(l); err != nil {
		return err
	}
	return validateLinkOptional(l)
}

func validateLinkHref(l Link) error {
	href := l.Href
	if strings.TrimSpace(href) == "" {
		return fmt.Errorf("%w: href must not be empty", ErrInvalidLink)
	}
	if strings.ContainsRune(href, '{') {
		return validateTemplatedHref(l)
	}
	if l.Templated {
		return fmt.Errorf("%w: templated is true for a non-template href", ErrInvalidLink)
	}
	if err := uriref.Validate(href); err != nil {
		return fmt.Errorf("%w: href: %v", ErrInvalidLink, err)
	}
	return nil
}

func validateTemplatedHref(l Link) error {
	tmpl, err := uritemplate.Parse(l.Href)
	if err != nil {
		return fmt.Errorf("%w: href template: %v", ErrInvalidLink, err)
	}
	if !tmpl.HasExpressions() {
		return fmt.Errorf("%w: href template has no expression", ErrInvalidLink)
	}
	if tmpl.HasUnsupportedOperator() {
		return fmt.Errorf("%w: href template uses an unsupported operator", ErrInvalidLink)
	}
	if !l.Templated {
		return fmt.Errorf("%w: template href requires templated true", ErrInvalidLink)
	}
	return nil
}

func validateLinkOptional(l Link) error {
	if l.Type != "" {
		if err := validateLinkMediaType(l.Type); err != nil {
			return err
		}
	}
	if l.Deprecation != "" {
		if err := uriref.Validate(l.Deprecation); err != nil {
			return fmt.Errorf("%w: deprecation: %v", ErrInvalidLink, err)
		}
	}
	if l.Profile != "" {
		if err := uriref.Validate(l.Profile); err != nil {
			return fmt.Errorf("%w: profile: %v", ErrInvalidLink, err)
		}
	}
	if l.HrefLang != "" && !isLanguageTag(l.HrefLang) {
		return fmt.Errorf("%w: hreflang %q is not a language tag", ErrInvalidLink, l.HrefLang)
	}
	return nil
}

func validateLinkMediaType(value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%w: type %q contains a line break", ErrInvalidLink, value)
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return fmt.Errorf("%w: type %q is not a media type", ErrInvalidLink, value)
	}
	slash := strings.IndexByte(mediaType, '/')
	if slash <= 0 || slash == len(mediaType)-1 || strings.ContainsRune(mediaType, '*') {
		return fmt.Errorf("%w: type %q is not a concrete media type", ErrInvalidLink, value)
	}
	return nil
}

//nolint:cyclop // Small character-class scanner kept in one pass.
func isLanguageTag(tag string) bool {
	parts := strings.Split(tag, "-")
	if len(parts) == 0 {
		return false
	}
	for i, part := range parts {
		if len(part) == 0 || len(part) > 8 {
			return false
		}
		for j := 0; j < len(part); j++ {
			c := part[j]
			switch {
			case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			case i > 0 && c >= '0' && c <= '9':
			default:
				return false
			}
		}
	}
	return true
}
