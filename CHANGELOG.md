# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- Minimum Go version is now **1.26**. Go 1.22, 1.24, and 1.25 are end-of-life
  and are no longer tested.
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
- `RegisterCurie` panics for an empty/whitespace prefix or empty href.
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
