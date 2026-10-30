// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"errors"
	"fmt"
	"mime"
	"strings"

	"github.com/Emin-ACIKGOZ/go-hal/internal/uriref"
)

// MediaTypeHAL is the media type named by draft-kelly-json-hal-11. It is not
// registered with IANA.
const MediaTypeHAL = "application/hal+json"

// MediaTypeVendorHAL is the IANA-registered vendor HAL media type. Its 2011
// IANA registration defines no optional parameters.
const MediaTypeVendorHAL = "application/vnd.hal+json"

// ErrInvalidProfile is returned when a ContentType profile is not an absolute
// URI. It is specific to ContentType; strict Link.Profile validation wraps
// ErrInvalidLink instead.
var ErrInvalidProfile = errors.New("hal: invalid profile")

// ContentType builds an application/hal+json content type with an optional
// RFC 6906 profile parameter. Zero profiles returns the bare media type; each
// non-empty profile must be an absolute URI and is joined into one space-
// separated profile value. The bytes are preserved as supplied; schemes other
// than http/https (urn:, mailto:, tag:) are accepted.
func ContentType(profiles ...string) (string, error) {
	if len(profiles) == 0 {
		return MediaTypeHAL, nil
	}
	for _, profile := range profiles {
		if err := uriref.ValidateAbsolute(profile); err != nil {
			return "", fmt.Errorf("%w %q: %w", ErrInvalidProfile, profile, err)
		}
	}
	formatted := mime.FormatMediaType(MediaTypeHAL, map[string]string{
		"profile": strings.Join(profiles, " "),
	})
	if formatted == "" {
		return "", fmt.Errorf("hal: cannot format content type for %d profiles", len(profiles))
	}
	return formatted, nil
}
