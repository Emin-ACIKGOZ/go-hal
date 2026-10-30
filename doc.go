// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

// Package hal implements the Hypertext Application Language (HAL) for JSON.
//
// It provides a mechanism to wrap existing Go structs in an envelope that
// injects HAL "_links" and "_embedded" fields without requiring changes to the
// underlying data models. AddEmbedded supports arbitrary Resource Objects and
// the §8.4 cache pattern; CollectionPage also emits the project-specific
// "_embedded.items" convention.
//
// The package relies on a central or instance-based registry to map Go types
// to link generation functions. It supports CURIEs (Compact URIs), reusable
// precomputed links, stable link shapes, media-type helpers, and independent
// opt-in strict policies for links, relations, CURIEs, and JSON. Template hrefs
// gain templated=true on emission, and generated documents declare CURIEs for
// embedded-only relation prefixes. Client-side navigation is deferred to a
// planned separate halclient package.
//
// The target is the supported subset of draft-kelly-json-hal-11, an expired
// Informational draft rather than an RFC. See COMPLIANCE.md for the support
// matrix, validation boundaries, and out-of-scope features.
//
// All methods on Instance are safe for concurrent use. Envelopes may be
// marshaled concurrently while they are not being mutated.
package hal
