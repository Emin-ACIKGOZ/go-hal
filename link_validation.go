// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Emin-ACIKGOZ/go-hal/internal/uriref"
)

// ErrInvalidLink is returned when a Link fails validation. By default links are
// permissive; ValidateLink and instances configured with WithStrictLinks apply
// the full checks.
var ErrInvalidLink = errors.New("hal: invalid link")

func validateLinkHref(href string) error {
	if strings.TrimSpace(href) == "" {
		return fmt.Errorf("%w: href must not be empty", ErrInvalidLink)
	}
	if err := uriref.Validate(href); err != nil {
		return fmt.Errorf("%w: href: %v", ErrInvalidLink, err)
	}
	return nil
}
