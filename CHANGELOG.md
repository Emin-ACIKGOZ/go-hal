# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.4.0] - 2026-10-30

### Added

- Automatic `templated: true` derivation for RFC 6570 template hrefs on dynamic
  `AddLink`/`AddCachedLink` calls, static links, `Collection` self links, and
  struct-supplied `curies` declarations. `%7B`/`%7D` remain data, and
  malformed or reserved-operator templates are not advertised as templates.
- Generated `curies` declarations for registered relation prefixes that appear
  only in `_embedded` keys, including byte-splicing into precomputed parents
  that already carry a `curies` array.
- `ErrInvalidProfile` and absolute URI validation for `ContentType` profiles via
  the shared internal `uriref.ValidateAbsolute` helper.
- Strict `Link.Profile` validation now requires an absolute URI.
- Strict relation validation honors resource-local and inherited CURIE
  declarations, including declarations added after their first use and
  declarations inherited from a precomputed parent.
- Document-wide `WithStrictJSON`: a strict parent policy is inherited by
  embedded child envelopes transitively and raw embedded resources are scanned
  for duplicate keys at any depth.

### Changed

- Relation validation now uses a shared collision index; default relation
  grammar, case-variant collision behavior, and link-shape defaults are
  unchanged.
- `ContentType` now rejects profiles that are not absolute URIs, wrapping
  `ErrInvalidProfile` and `uriref.ErrInvalidURIReference`.
- Document client-side navigation as out of scope for this server-side module
  instead of promising a future `halclient` package. Draft-11 has no
  client-side MUST; §8.4 makes cache-pattern support OPTIONAL and §5.4 applies
  to clients that traverse a deprecated link.

### Fixed

- Precomputed parent `curies` declarations are preserved when embedded-only
  prefixes are spliced in; only the `curies` member is rewritten.
- Strict CURIE decoding and strict link compilation continue to reject
  precomputed template declarations that omit `templated: true`, while
  struct-supplied declarations derive it during normalization.

### Migration

Default output now gains `templated: true` on template links (dynamic, static,
Collection, and struct-supplied curies) and generated `curies` declarations for
embedded-only registered prefixes. `ContentType` now rejects non-absolute
profiles. Strict link validation accepts underived template structs because
normalization supplies the flag, but precomputed JSON still requires an explicit
`templated: true`. Strict relations now honor resource-local and inherited CURIE
declarations, and strict JSON applies document-wide. Default relation grammar,
default case-variant collision behavior, and link-shape defaults are unchanged.

## [1.3.0] - 2026-10-30

### Added

- `(*Envelope).AddEmbedded` for arbitrary embedded Resource Objects, with
  object/array multiplicity, one-level slice flattening, static/precomputed
  splicing, depth-32 and cycle protection, and nested `*Envelope` support.
- `(*Envelope).AddCachedLink` for the draft §8.4 hypertext cache pattern.
- `ValidateLink`, `WithStrictLinks`, `WithStrictRelations`, and
  `WithStrictCuries` for opt-in link, relation, and CURIE validation.
- `WithStrictJSON`, `ErrDuplicateJSONKey`, and `DuplicateKeyError{Key, Offset}`
  for opt-in duplicate-key scanning of marshaled state.
- `LinkShape` (`ShapeAuto`, `ShapeArray`, `ShapeObject`), `WithLinkShape`, and
  `WithRelationShape` for stable object/array relation shapes.
- `CompilePrecomputedLinksStrict` for reusable handles that strict instances
  accept.
- `MediaTypeHAL`, `MediaTypeVendorHAL`, and `ContentType(profiles...)`.
- `ErrInvalidEmbedded`, `ErrEmbeddedDepthExceeded`, `ErrInvalidLink`,
  `ErrInvalidRelation`, and `ErrLinkShape`.
- Embedded CURIE scope with parent fallback and child-local override.

### Changed

- Relationship hardening now rejects blank/control/non-UTF-8 relations,
  bare-token case collisions, and CURIE names containing a colon or control
  character. Default link output is otherwise unchanged.
- Generated CURIE declarations derive `templated` from the href template.
- Docs describe the full server-side draft-11 scope and defer client support to
  a planned separate `halclient` package.

### Fixed

- Repeated multi-element embedded slices flatten into one resource array instead
  of nesting arrays.
- Strict relations accept declared non-scheme CURIEs and reject RFC 8288 bare
  tokens containing underscores.
- Default collision detection folds only actual bare tokens, not relative
  references.
- Precomputed CURIE merging byte-splices only the `curies` member, preserving
  unknown fields and user declarations.
- Strict link validation rejects media ranges and unsafe template literals;
  URI-reference validation rejects misplaced brackets and multiple fragment
  delimiters.

## [1.2.0] - 2026-10-30

### Changed

- Minimum Go version is now **1.26**. Go 1.22–1.25 are end-of-life and are no
  longer tested.
- Upgraded `github.com/goccy/go-json` to v0.12.0.
- Upgraded `github.com/getkin/kin-openapi` to v0.152.0 and refreshed the
  security-relevant transitive modules, including `golang.org/x/text` to
  v0.43.0.
- `govulncheck` reports no known vulnerabilities in the final module graph.

## [1.1.0] - 2026-10-30

### Added

- `CompilePrecomputedLinks` and `(*Instance).WrapPrecomputedLinks` for validated
  precomputed links that can be reused across envelopes and instances.
- `ErrDuplicateCurie` and `ErrInvalidCurie` for CURIE declaration validation.
- `ErrAddLinkPrecomputed` for attempts to mutate static or precomputed envelopes.
- `COMPLIANCE.md` with the HAL draft scope, support matrix, validation
  boundaries, concurrency notes, and primary sources.

### Changed

- `Envelope.MarshalJSON` now has a value receiver, so both `Envelope` and
  `*Envelope` marshal as HAL resources.
- `RegisterStatic` rejects invalid registration input with a panic and treats
  empty links as an empty, generator-suppressing link set. Registered CURIEs
  resolve at wrap time.
- User CURIE declarations win over registered declarations with the same name.
  Unknown fields on user `curies` declarations survive merging.
- `RegisterCurie` panics for unusable prefixes or hrefs; strict CURIE
  validation additionally requires an NCName and a `rel` template.
- Static and precomputed envelopes reject `AddLink` at marshaling time.
- `CollectionPage` omits empty `_links` / `_embedded` containers and always
  emits `count` and `total`.
- `WrapPrecomputed` copies caller-owned link bytes and compiles on every call;
  use the compiled API for hot paths. Compiled handles are read-only by
  convention, and marshaling cached links returns an owned copy.
- CI runs Go 1.22.5, 1.24.11, and stable with vet, shuffle, race, coverage, and
  fuzz smoke tests. Benchmark comparison is advisory `benchstat`.

### Fixed

- Precomputed links reject `null` and mistyped `href`, `templated`, string
  fields, and CURIE declarations.
- Escaped reserved HAL keys such as `\u005flinks` are rejected.
- Duplicate-key scanning preserves large numbers and rejects empty, truncated,
  and trailing JSON.
- Whitespace-only objects are treated as empty instead of corrupting spliced
  output.
- OpenAPI helpers tolerate nil documents, schemas, and item refs, preserve a
  custom Link schema, and mark HAL collection fields read-only.

### Notes

- Released as **v1.1.0** on 2026-10-30. `version.go` reports `v1.1.0`.
- HAL target: `draft-kelly-json-hal-11`, an expired IETF Informational draft;
  `application/hal+json` is not an IANA-registered media type.
