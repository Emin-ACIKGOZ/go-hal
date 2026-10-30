// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"errors"
	"mime"
	"strings"
	"testing"

	"github.com/Emin-ACIKGOZ/go-hal/internal/uriref"
)

func TestMediaTypeConstants(t *testing.T) {
	t.Parallel()

	if MediaTypeHAL != "application/hal+json" {
		t.Fatalf("MediaTypeHAL = %q", MediaTypeHAL)
	}
	if MediaTypeVendorHAL != "application/vnd.hal+json" {
		t.Fatalf("MediaTypeVendorHAL = %q", MediaTypeVendorHAL)
	}
}

func TestContentType_ZeroProfiles(t *testing.T) {
	t.Parallel()

	got, err := ContentType()
	if err != nil {
		t.Fatal(err)
	}
	if got != MediaTypeHAL {
		t.Fatalf("ContentType() = %q, want %q", got, MediaTypeHAL)
	}
}

func TestContentType_OneProfile(t *testing.T) {
	t.Parallel()

	got, err := ContentType("https://example.com/profiles/order")
	if err != nil {
		t.Fatal(err)
	}
	mediaType, params, err := mime.ParseMediaType(got)
	if err != nil {
		t.Fatalf("ParseMediaType(%q): %v", got, err)
	}
	if mediaType != MediaTypeHAL {
		t.Fatalf("media type = %q, want %q", mediaType, MediaTypeHAL)
	}
	if params["profile"] != "https://example.com/profiles/order" {
		t.Fatalf("profile = %q", params["profile"])
	}
}

func TestContentType_MultipleProfiles(t *testing.T) {
	t.Parallel()

	profiles := []string{
		"https://example.com/profiles/order",
		"https://example.com/profiles/v2",
	}
	got, err := ContentType(profiles...)
	if err != nil {
		t.Fatal(err)
	}
	_, params, err := mime.ParseMediaType(got)
	if err != nil {
		t.Fatal(err)
	}
	if params["profile"] != strings.Join(profiles, " ") {
		t.Fatalf("profile = %q, want %q", params["profile"], strings.Join(profiles, " "))
	}
	if _, exists := params["profile*"]; exists {
		t.Fatal("ContentType emitted an RFC 2231 profile* parameter")
	}
}

func TestContentType_RejectsUnsafeProfiles(t *testing.T) {
	t.Parallel()

	invalid := [][]string{
		{""},
		{"   "},
		{"https://example.com/a b"},
		{"https://example.com/a\nb"},
		{"https://example.com/a\tb"},
		{"https://example.com/受"},
		{"https://example.com/\xff"},
		{"https://example.com/ok", ""},
	}
	for _, profiles := range invalid {
		if _, err := ContentType(profiles...); err == nil {
			t.Errorf("ContentType(%q) accepted unsafe profile", profiles)
		}
	}
}

func TestContentType_VendorTypeIsParameterless(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(MediaTypeVendorHAL, ';') {
		t.Fatalf("vendor media type carries parameters: %q", MediaTypeVendorHAL)
	}
}

func TestContentType_RequiresAbsoluteURIProfiles(t *testing.T) {
	t.Parallel()

	valid := []string{
		"https://example.com/profiles/order",
		"HTTPS://example.com/profiles/order",
		"urn:example:profile",
		"mailto:user@example.com",
		"tag:example.com,2026:profile",
		"https://example.com/a%20b",
		"https://example.com/a;b",
		"https://example.com/a,b",
		"https://example.com/a=b",
	}
	for _, profile := range valid {
		got, err := ContentType(profile)
		if err != nil {
			t.Errorf("ContentType(%q) = %v, want nil", profile, err)
			continue
		}
		mediaType, params, err := mime.ParseMediaType(got)
		if err != nil {
			t.Errorf("ParseMediaType(%q): %v", got, err)
			continue
		}
		if mediaType != MediaTypeHAL || params["profile"] != profile {
			t.Errorf("round-trip(%q) = %q (%q)", profile, mediaType, params["profile"])
		}
	}

	invalid := []string{
		"notauri",
		"relative/path",
		"/abs/path",
		"//example.com/path",
		"?query=only",
		"#fragment",
		"",
		"   ",
		"https://",
		"http:///x",
		"1a:b",
		"https://example.com/受",
	}
	for _, profile := range invalid {
		got, err := ContentType(profile)
		if err == nil {
			t.Errorf("ContentType(%q) = %q, want error", profile, got)
			continue
		}
		if got != "" {
			t.Errorf("ContentType(%q) returned partial output %q", profile, got)
		}
		if !errors.Is(err, ErrInvalidProfile) {
			t.Errorf("ContentType(%q) error = %v, want ErrInvalidProfile", profile, err)
		}
		if !errors.Is(err, uriref.ErrInvalidURIReference) {
			t.Errorf("ContentType(%q) error = %v, want ErrInvalidURIReference", profile, err)
		}
	}
}
