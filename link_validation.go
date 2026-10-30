// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Emin-ACIKGOZ/go-hal/internal/uriref"
	"github.com/Emin-ACIKGOZ/go-hal/internal/uritemplate"
)

// ErrInvalidLink is returned when a Link fails validation. By default links are
// permissive; ValidateLink and instances configured with WithStrictLinks apply
// the full checks.
var ErrInvalidLink = errors.New("hal: invalid link")

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
