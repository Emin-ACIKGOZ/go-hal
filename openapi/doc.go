// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

// Package openapi provides an adapter to augment OpenAPI 3.0 documents with HAL semantics.
//
// It injects standard HAL schemas (Link, _links, _embedded) into existing
// OpenAPI definitions, ensuring that generated documentation accurately reflects
// the hypermedia controls added by the hal package.
package openapi
