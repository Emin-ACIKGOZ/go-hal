// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"fmt"
	"mime"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MediaTypeHAL is the media type named by draft-kelly-json-hal-11. It is not
// registered with IANA.
const MediaTypeHAL = "application/hal+json"

// MediaTypeVendorHAL is the IANA-registered vendor HAL media type. It is
// informative and does not define optional media type parameters.
const MediaTypeVendorHAL = "application/vnd.hal+json"

// ContentType builds an application/hal+json content type with an optional
// RFC 6906 profile parameter. Zero profiles returns the bare media type; each
// non-empty profile is joined into one space-separated profile value. Profiles
// are checked for header safety (empty, whitespace, control, non-ASCII,
// invalid UTF-8) but are not otherwise URI-validated.
func ContentType(profiles ...string) (string, error) {
	if len(profiles) == 0 {
		return MediaTypeHAL, nil
	}
	for _, profile := range profiles {
		if !validProfile(profile) {
			return "", fmt.Errorf("hal: invalid profile %q", profile)
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

func validProfile(profile string) bool {
	if profile == "" || !utf8.ValidString(profile) {
		return false
	}
	for _, r := range profile {
		if r > unicode.MaxASCII || unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}
