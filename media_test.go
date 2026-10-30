// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import "testing"

func TestMediaTypeConstants(t *testing.T) {
	t.Parallel()

	if MediaTypeHAL != "application/hal+json" {
		t.Fatalf("MediaTypeHAL = %q", MediaTypeHAL)
	}
	if MediaTypeVendorHAL != "application/vnd.hal+json" {
		t.Fatalf("MediaTypeVendorHAL = %q", MediaTypeVendorHAL)
	}
}
