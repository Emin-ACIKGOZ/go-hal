// Copyright (c) 2025-2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

// Package hal provides a high-performance HAL (Hypertext Application Language) implementation.
//
// # Overview
//
// go-hal is a zero-reflection, allocation-efficient HAL serialization library for Go. It wraps your
// Go structs in HAL Envelopes, injecting _links and _embedded metadata during JSON
// serialization without intermediate allocations.
//
// # Key Features
//
//   - Zero reflection at runtime (type-safe closures)
//   - Pre-computed links for maximum performance
//   - Full HAL spec compliance (draft-kelly-json-hal)
//   - Dual-mode: global singleton or isolated instances
//
// # Quick Start
//
//	hal.Register(func(ctx context.Context, u *User) []hal.Link {
//	    return []hal.Link{{Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)}}
//	})
//
//	env := hal.Wrap(ctx, &User{ID: 1, Name: "Alice"})
//	json.Marshal(env) // => {"id":1,"name":"Alice","_links":{"self":{"href":"/users/1"}}}
package hal

const (
	jsonTrailingChars          = 2  // } to remove from JSON
	precomputedLinksPrefixLen  = 10 // len(`{"_links":`)
	precomputedLinksWrapperLen = 2  // {}
)

// Link represents a HAL Hypermedia link.
// Implements draft-kelly-json-hal Section 5 Link Object.
//
// All fields (except Rel and Method) are compliant with the HAL specification.
// Rel is used internally as the map key and is not serialized.
// Method is a non-standard extension for HTTP method hints.
//
// # Example
//
//	Link{
//	    Rel:       "self",
//	    Href:      "/users/123",
//	    Templated: false,
//	    Title:     "Current User",
//	}
type Link struct {
	Rel         string `json:"-"` // Used for map keys, not serialized directly
	Href        string `json:"href"`
	Templated   bool   `json:"templated,omitempty"`
	Type        string `json:"type,omitempty"`
	Deprecation string `json:"deprecation,omitempty"` // OPTIONAL: link to deprecation info
	Name        string `json:"name,omitempty"`
	Profile     string `json:"profile,omitempty"` // OPTIONAL: URI hint about target
	Title       string `json:"title,omitempty"`
	HrefLang    string `json:"hreflang,omitempty"` // OPTIONAL: language of target
	Method      string `json:"method,omitempty"`   // Non-standard: common hint for HTTP methods
}

// Envelope is the container for your data with HAL metadata.
// It wraps your Go struct and injects _links and _embedded during JSON serialization.
//
// To create an Envelope:
//
//	// Using global instance
//	env := hal.Wrap(ctx, &MyStruct{...})
//
//	// Using isolated instance
//	inst := hal.New()
//	env := inst.Wrap(ctx, &MyStruct{...})
//
// The Envelope implements json.Marshaler and will inject HAL metadata automatically.
type Envelope struct {
	Data            any            // The user's struct
	instance        *Instance      // The registry instance to use
	links           map[string]any // Computed during marshal
	embedded        map[string]any // Computed during marshal
	precomputedJSON []byte         // OPTIMIZATION: pre-serialized links JSON
}

// InstanceOption configures a new HAL Instance.
// Apply options when creating a new Instance:
//
//	inst := hal.New(hal.WithStrictMode())
type InstanceOption func(*Instance)

// WithStrictMode enables panic on generator mismatches.
// When enabled, hal.Wrap panics if:
//
//  1. You pass a value type T but registered a generator for *T
//  2. You pass a type with no registered generator
//
// This helps catch type mismatch errors during development.
//
// # Example
//
//	inst := hal.New(hal.WithStrictMode()) // panics on mismatches
func WithStrictMode() InstanceOption {
	return func(i *Instance) {
		i.strictMode = true
	}
}
