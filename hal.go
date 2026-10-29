// Copyright (c) 2025-2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

const (
	jsonTrailingChars          = 2  // } to remove from JSON
	precomputedLinksPrefixLen  = 10 // len(`{"_links":`)
	precomputedLinksWrapperLen = 2  // {}

	fieldLinks    = "_links"
	fieldEmbedded = "_embedded"
	relCuries     = "curies"
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
// It wraps your Go value and injects _links during JSON serialization.
// CollectionPage separately emits the project-specific _embedded.items shape.
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
// Envelope implements json.Marshaler with a value receiver, so both Envelope and
// *Envelope marshal as HAL resources. AddLink remains pointer-only. Static and
// precomputed envelopes reject AddLink with ErrAddLinkPrecomputed. An Envelope
// may be marshaled repeatedly or concurrently while it is not being mutated.
type Envelope struct {
	Data            any            // The user's struct
	instance        *Instance      // The registry instance to use
	links           map[string]any // Computed during marshal
	embedded        map[string]any // Computed during marshal
	precomputedJSON []byte         // OPTIMIZATION: pre-serialized links JSON
	err             error          // Deferred error surfaced during marshaling
	premanaged      bool           // Links are static/precomputed and must not be added to
}

// InstanceOption configures a new HAL Instance.
// Apply options when creating a new Instance:
//
//	inst := hal.New(hal.WithStrictMode())
type InstanceOption func(*Instance)

// WithStrictMode enables panic on generator mismatches.
//
// It applies only to Instance.Wrap on the generated path. When Data is a
// non-nil struct or pointer-to-struct, Wrap panics if:
//
//  1. You pass a value type T but registered a generator for *T
//  2. No generator is registered for the exact type
//
// Static links, WrapRaw, WrapPrecomputed, WrapPrecomputedLinks, nil data, and
// non-struct values bypass these checks. Strict mode does not validate links
// or JSON.
//
// # Example
//
//	inst := hal.New(hal.WithStrictMode()) // panics on generator mismatches
func WithStrictMode() InstanceOption {
	return func(i *Instance) {
		i.strictMode = true
	}
}
