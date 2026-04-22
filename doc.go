// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

// Package hal implements the Hypertext Application Language (HAL) for JSON.
//
// It provides a mechanism to wrap existing Go structs in an envelope that
// injects standard HAL "_links" and "_embedded" fields without requiring
// changes to the underlying data models.
//
// The package relies on a central or instance-based registry to map Go types
// to link generation functions. It supports CURIEs (Compact URIs) and strict
// mode validation to ensure type safety during link generation.
//
// All methods on Instance are safe for concurrent use.
package hal
