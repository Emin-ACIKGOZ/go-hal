# HAL Compliance Scope

This document describes what `go-hal` implements, what it deliberately leaves
to callers, and the exact validation boundary of each API. It targets
`draft-kelly-json-hal-11`, the latest JSON HAL draft available at the time of
writing.

## Target and status

- Target: `draft-kelly-json-hal-11` (19 October 2023), an IETF Internet-Draft
  with intended status **Informational** and expiry **21 April 2024**. It is not
  an RFC. See [draft-kelly-json-hal-11](https://datatracker.ietf.org/doc/html/draft-kelly-json-hal-11).
- The draft's examples and vocabulary use `application/hal+json`. That media
  type is **not** in the IANA media type registry. The registered vendor type is
  [`application/vnd.hal+json`](https://www.iana.org/assignments/media-types/application/vnd.hal+json).
  This library does not force either value; content negotiation and media type
  selection belong to the application.
- HAL resource objects allow the reserved `_links` and `_embedded` properties
  (draft §4.1). Link objects and their optional fields are defined in draft §5.
  HAL CURIEs are defined in draft §8.3.
- `CollectionPage`, `count`, `total`, and `_embedded.items` are **project
  conventions**, not HAL specification requirements.

### Draft-08 differences

`draft-kelly-json-hal-08` is the older and still widely referenced revision.
The `_links`, `_embedded`, and Link Object shapes targeted here are compatible
with the draft-08 vocabulary, but this project does not claim conformance to a
particular older revision. Errors, CURIEs, and recommendations are interpreted
according to draft-11.

## Support matrix

| Area | Status | Notes |
| --- | --- | --- |
| Resource object with `_links` / `_embedded` | Supported | JSON object roots only. `null` and empty objects are normalized to an empty resource. |
| Link Object fields | Supported | `href`, `templated`, `type`, `deprecation`, `name`, `profile`, `title`, `hreflang`; `method` is a non-standard extension. |
| Relation multiplicity | Supported | One value is an object; repeated values become an array. |
| HAL CURIEs | Supported | Declarations are merged by name; user declarations win over registered prefixes. |
| Static links (`RegisterStatic`) | Supported | Links are serialized once; registered CURIEs resolve at wrap time. |
| Precomputed links | Supported | Validated once with `CompilePrecomputedLinks`; reusable handles with `WrapPrecomputedLinks`. |
| JSON duplicate-key protection | Partial | Checked for precomputed link input and scanner helpers; arbitrary user state is not re-scanned for duplicate keys. |
| Reserved-field protection | Supported | Top-level `_links` / `_embedded` in user data, including escaped keys, are rejected with `ErrHALFieldConflict`. |
| `AddEmbedded` helper | Out of scope | Embed via a struct field or the `_embedded`-aware collection convention. |
| Media-type helpers | Out of scope | Use `application/vnd.hal+json` or your negotiated profile. |
| URI / RFC 6570 template validation | Out of scope | `href` values are strings; template syntax is not validated. |
| Hard NCName validation for CURIE names | Out of scope | Names must be non-empty and whitespace-free, but are not restricted to NCName. |
| Client navigation / discovery | Out of scope | This is a server-side serializer. |
| OpenAPI generation from Go, runtime response validation | Out of scope | The `openapi` adapter only augments schemas. |

## Validation boundaries

### Precomputed links

`CompilePrecomputedLinks` and `WrapPrecomputed` validate that the input is:

- valid UTF-8 JSON and a JSON object;
- free of duplicate object keys at any depth;
- a relation map whose relation names are non-blank;
- link values shaped as objects or arrays of objects;
- `href` present and a string; known optional fields have the correct JSON type;
- `curies` an array of Link Objects with non-empty names and hrefs, and no
  duplicate names.

Unknown link fields pass through unchanged. Empty relation arrays are allowed.
Caller bytes are copied, so later mutation cannot corrupt a handle or envelope.

`PrecomputedLinks` handles are immutable and reusable across envelopes and
instances. The zero value means an empty link set. A hand-built, non-validated
handle with JSON is rejected at marshal time.

### Static links

`RegisterStatic` panics for a nil instance, a nil or non-pointer target, an
empty relation, or invalid/duplicate CURIE declarations. A typed-nil pointer
target is accepted. Empty links suppress generators and emit no `_links`.
Registered CURIEs resolve at wrap time, not registration time, so late
registrations and cross-instance reuse behave predictably.

### Generated links

Generator output is serialized through the same relation-multiplicity rules.
User-provided CURIE declarations are normalized to an array and merged with
registered declarations; duplicate names within one source are rejected with
`ErrDuplicateCurie`, and invalid declarations return `ErrInvalidCurie`.
Generated declarations set `templated: true`; user declarations are not forced
to set it.

### Envelope mutation

Static and precomputed envelopes are immutable: `AddLink` returns a deferred
`ErrAddLinkPrecomputed` from marshaling. Generated and raw envelopes continue to
accept links. An empty or whitespace-only relation is a deferred error.

## Concurrency

`Instance` registration is guarded by a read/write mutex. Compiled
`PrecomputedLinks` handles are read-only after compilation. An `Envelope` may be
marshaled repeatedly and concurrently as long as it is not mutated; `AddLink`
is not safe to call concurrently with marshaling. Passing an envelope by value
copies its internal map references, so link maps are shared unless the envelope
is reconstructed.

## OpenAPI adapter

The optional `hal/openapi` adapter:

- tolerates nil documents, components, schemas, and item references;
- injects a standard Link schema unless the document already defines one;
- adds `_links`, `_embedded`, `count`, and `total` shapes as read-only;
- models `Link | []Link` with `oneOf` and normalizes missing item refs.

It neither generates OpenAPI from Go code nor validates runtime responses.

## Sources

- [draft-kelly-json-hal-11](https://datatracker.ietf.org/doc/html/draft-kelly-json-hal-11)
  — §4.1 reserved properties, §5 Link Objects, §8.3 HAL curies.
- [draft-kelly-json-hal-08](https://datatracker.ietf.org/doc/html/draft-kelly-json-hal-08)
  — older revision used by some clients.
- [IANA media types: application/vnd.hal+json](https://www.iana.org/assignments/media-types/application/vnd.hal+json)
  — registered vendor media type. `application/hal+json` is not registered.
- [RFC 8288](https://www.rfc-editor.org/rfc/rfc8288) — Web Linking.
- [RFC 6570](https://www.rfc-editor.org/rfc/rfc6570) — URI Template syntax
  (not validated by this library).
- [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) — JSON.
- [RFC 6906](https://www.rfc-editor.org/rfc/rfc6906) — the `profile` media type
  parameter.
