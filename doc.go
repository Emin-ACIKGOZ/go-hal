// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

// Package hal implements the Hypertext Application Language (HAL) for JSON.
//
// It provides a mechanism to wrap existing Go structs in an envelope that
// injects standard HAL "_links" and "_embedded" fields without requiring
// changes to the underlying data models.
//
// The package relies on a central or instance-based registry to map Go types
// to link generation functions. It supports CURIEs (Compact URIs), reusable
// precomputed links, and strict-mode checks on the generated path.
//
// The target is the supported subset of draft-kelly-json-hal-11, an expired
// Informational draft rather than an RFC. See COMPLIANCE.md for the support
// matrix, validation boundaries, and out-of-scope features.
//
// All methods on Instance are safe for concurrent use. Envelopes may be
// marshaled concurrently while they are not being mutated.
package hal
