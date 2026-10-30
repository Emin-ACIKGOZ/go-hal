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
	linkTokens      relationIndex  // folded bare-token index for link collisions
	embeddedTokens  relationIndex  // folded bare-token index for embedded collisions
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

// WithLinkShape sets the default relation shape. ShapeAuto keeps the default
// object-for-one/array-for-many behavior; ShapeArray always emits arrays;
// ShapeObject requires at most one link per relation.
func WithLinkShape(shape LinkShape) InstanceOption {
	return func(i *Instance) {
		i.linkShape = shape
	}
}

// WithRelationShape overrides the default shape for one relation. Later calls
// for the same relation replace the earlier override.
func WithRelationShape(rel string, shape LinkShape) InstanceOption {
	return func(i *Instance) {
		if i.relationShapes == nil {
			i.relationShapes = make(map[string]LinkShape)
		}
		i.relationShapes[rel] = shape
	}
}

// WithStrictJSON enables duplicate-key scanning of marshaled Data. It applies
// to generated, raw, static, precomputed, and compiled envelopes, scans only the
// non-empty Data object, and does not rescan final output or precomputed links
// (those are scanned at compile time). Default marshaling stays permissive.
func WithStrictJSON() InstanceOption {
	return func(i *Instance) {
		i.strictJSON = true
	}
}

// WithStrictCuries enables strict CURIE declaration validation on AddLink,
// merge, RegisterCurie, RegisterStatic, and precomputed wrapping. It is
// independent of WithStrictMode.
//
// Strict names use the ASCII NCName subset [A-Za-z_][A-Za-z0-9._-]*.
func WithStrictCuries() InstanceOption {
	return func(i *Instance) {
		i.strictCuries = true
	}
}

// WithStrictRelations enables strict relation grammar on AddLink, AddEmbedded,
// generated links, static links, and precomputed wrapping. It is independent of
// WithStrictMode.
//
// Strict relations accept lowercase registered tokens, any absolute URI scheme
// (including urn:, mailto:, and tag:), and declared CURIEs. Relative references,
// ":x", and mixed-case bare tokens are rejected. An undeclared "acme:widget" is
// accepted as an opaque URI because a non-authority scheme form is
// indistinguishable from an absolute URI without declaration context.
func WithStrictRelations() InstanceOption {
	return func(i *Instance) {
		i.strictRelations = true
	}
}

// WithStrictLinks enables link validation on AddLink, generated links, static
// links, Collection self links, and precomputed wrapping. It is independent of
// WithStrictMode. By default links are permissive; ValidateLink can be called
// directly for one-off checks.
//
// Strict links accept absolute and relative URI references, require
// templated=true for template hrefs, require templated=false/absent for URI
// hrefs, and validate type, deprecation, profile, and hreflang syntax.
func WithStrictLinks() InstanceOption {
	return func(i *Instance) {
		i.strictLinks = true
	}
}
