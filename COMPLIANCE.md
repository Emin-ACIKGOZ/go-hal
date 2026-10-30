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
  The constants `MediaTypeHAL` and `MediaTypeVendorHAL` expose both spells;
  content negotiation remains the application's responsibility.
- HAL resource objects allow the reserved `_links` and `_embedded` properties
  (draft §4.1). Link objects and their optional fields are defined in draft §5.
  HAL CURIEs are defined in draft §8.3. The hypertext cache pattern is draft
  §8.4.
- This module requires Go 1.26 or newer; only supported Go releases are tested.
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
| Resource object `_links` | Supported | JSON object roots only. `null` and empty objects are normalized to an empty resource. |
| Resource object `_embedded` | Supported | `AddEmbedded`, `Collection`, and static/precomputed splicing. Arbitrary raw object graphs are caller-owned. |
| Link Object fields | Supported | `href`, `templated`, `type`, `deprecation`, `name`, `profile`, `title`, `hreflang`; `method` is a non-standard extension. |
| Relation multiplicity | Supported | One value is an object; repeated values become an array. |
| Stable link shape | Supported | `ShapeAuto` default, `ShapeArray`, `ShapeObject`; per-relation overrides with `WithRelationShape`. `curies` is always an array. |
| Link relations | Supported | Default hardening rejects blank/control/non-UTF-8 names and bare-token case collisions. `WithStrictRelations` adds grammar checks. |
| HAL CURIEs | Supported | Declarations merge by name; user declarations win over registered prefixes. Embedded scope uses parent fallback and local override. |
| Embedded resources | Supported | `*Envelope`, raw structs/maps/objects, one-level slices; depth 32 and cycle detection. |
| Hypertext cache pattern | Supported | `AddCachedLink` adds a link and the same-relation embedded resource atomically. |
| Static links (`RegisterStatic`) | Supported | Links are serialized once; registered CURIEs resolve at wrap time. Strict policy is applied at registration. |
| Precomputed links | Supported | Validated once with `CompilePrecomputedLinks`; strict compilation with `CompilePrecomputedLinksStrict`; reusable handles. |
| JSON duplicate-key protection | Partial by default | Precomputed link input is always scanned. `WithStrictJSON` scans marshaled state. |
| Reserved-field protection | Supported | Top-level `_links` / `_embedded` in user data, including escaped keys, are rejected with `ErrHALFieldConflict`. |
| Media-type helpers | Supported | `ContentType(profiles...)` formats `application/hal+json` with an RFC 6906 profile parameter. Profiles must be absolute URIs. No negotiation. |
| URI / RFC 6570 template validation | Opt-in | `ValidateLink` and `WithStrictLinks` validate href/templates. Default link output remains permissive. |
| Hard NCName validation for CURIE names | Opt-in | Default rejects colon/control/whitespace/invalid UTF-8; `WithStrictCuries` applies the ASCII NCName subset. |
| Automatic self/profile links | Not provided | Add `Link{Rel: "self"}` or `Link{Rel: "profile"}` explicitly. |
| Client navigation / discovery | Deferred | Optional under draft-11; planned as a separate `halclient` package. Not in this module. |
| OpenAPI generation from Go, runtime response validation | Out of scope | The `openapi` adapter only augments schemas. |

## Validation boundaries

### Embedded resources

`AddEmbedded` accepts an embedded Resource Object and applies HAL multiplicity:

- one value is an object, repeated values form an array;
- a slice is flattened one level, a one-element slice becomes an object, and an
  empty slice is a no-op;
- raw structs, maps, pointers to them, and object `json.RawMessage` values are
  embedded as-is and are not auto-wrapped or link-generated;
- `nil`, typed-nil pointers, scalars, `[]byte`, `Envelope` values, nested
  slices, and non-object `RawMessage` values are rejected with a deferred
  `ErrInvalidEmbedded`;
- relation names follow the default hardening and `WithStrictRelations` policy;
- direct nested envelopes must be `*Envelope` so identity, depth, and cycles can
  be tracked. The maximum depth is 32; cycles return
  `ErrEmbeddedDepthExceeded`.

Raw embedded graphs are caller-owned and are not scanned for cycles. The depth
and ancestor scope are marshal-scoped and are not stored on `Envelope`, so an
envelope remains safe to marshal repeatedly and concurrently while not mutated.

Static and precomputed envelopes accept `AddEmbedded`; `_links` bytes are
preserved and `_embedded` is spliced into the cached JSON. `AddLink` and
`AddCachedLink` on those envelopes add nothing and defer
`ErrAddLinkPrecomputed`.

### Links

By default Link Objects are permissive. `ValidateLink` and `WithStrictLinks`
apply this matrix:

| href | templated | result |
| --- | --- | --- |
| URI / relative reference | absent or false | valid |
| URI / relative reference | true | `ErrInvalidLink` |
| RFC 6570 template | true | valid |
| RFC 6570 template | absent or false | `ErrInvalidLink` |
| empty / whitespace | either | `ErrInvalidLink` |

The URI-reference validator accepts absolute and relative references, including
the draft §6 examples. It conservatively rejects controls, spaces, non-ASCII
bytes, malformed percent escapes, backslashes, braces in plain URI references,
raw brackets outside an authority, multiple raw `#` fragment delimiters, and
authorities without a host. Raw IPv6 brackets are accepted inside an authority;
percent-encoded `%5B` and `%23` remain data. It uses `net/url.Parse` plus a component scan,
not a full RFC 3986 parser. The template parser covers operators `+ # . / ; ? &`;
reserved operators `= , ! @ |` parse but are rejected in strict link and CURIE
contexts. `%7B` is data, not an expression.

Optional fields: `type` is checked with `mime.ParseMediaType` and must be a
concrete media type (wildcards/ranges such as `*/*` and `application/*` are
rejected); `deprecation` is a URI-reference (relative values are accepted
because the base is caller-controlled); `profile` is an absolute URI per draft
§5.6/RFC 6906; `hreflang` uses a reduced language-tag syntax. Template literals
reject control/space characters and the unsafe bytes `"`, `\`, `<`, `>`, `^`,
`` ` ``, and `|`. `name`, `title`, `method`, and `Rel` are not validated.

These link checks run through `ValidateLink` and `WithStrictLinks`; default
link output remains permissive.

Strict link policy is applied to `AddLink`, generated links, `RegisterStatic`,
`Collection` self links, and precomputed wrapping. Static registration panics
with `ErrInvalidLink`; deferred AddLink errors surface at marshal time.

### Relations and shape

Default hardening rejects blank, whitespace-bearing, control-bearing, or
non-UTF-8 relation names with `ErrInvalidRelation`, and rejects bare tokens that
collide case-insensitively (`self` vs `Self`). Exact duplicate tokens are normal
multiplicity. CURIEs and URIs remain case-sensitive.

`WithStrictRelations` additionally accepts lowercase registered tokens, any
absolute URI scheme (`https`, `urn`, `mailto`, `tag`, ...), and declared CURIEs.
It rejects relative references, `:x`, and mixed-case bare tokens. Because a
non-authority scheme form is lexically indistinguishable from a CURIE without
declaration context, an undeclared `acme:widget` is accepted as an opaque URI.
RFC 8288 `reg-rel-type` does not include underscore, so strict mode rejects
`a_b`; default mode still accepts it and does not fold it as a bare-token
collision. CURIE prefixes and strict NCName declarations still allow `_`.

`ShapeAuto` keeps the default one-object/many-array behavior. `ShapeArray`
always emits an array, including for one link. `ShapeObject` requires at most one
link per relation and returns/panics with `ErrLinkShape` for repeats. Static
links and Collection apply the instance policy; `curies` always stays an array.
Shape policy never rewrites precomputed JSON.

### CURIE declarations

Default declaration names must be non-empty valid UTF-8 and must not contain a
colon, whitespace, or control characters. `WithStrictCuries` restricts names to
the ASCII NCName subset `[A-Za-z_][A-Za-z0-9._-]*`.

Strict declarations require exactly one expression using `{rel}` or `{+rel}`,
no prefix or explode modifier, valid literal syntax, and `templated: true`.
A template without `templated: true`, a non-template with `templated: true`, or
`%7Brel%7D` (data) is rejected with `ErrInvalidCurie`. Generated declarations
derive `templated` from the registered href template so they never claim
`templated: true` for a literal.

Full draft §8.3 CURIE conformance requires `WithStrictCuries`: default mode keeps
accepting non-template declarations (and emits them unchanged) for backward
compatibility. This is an opt-in policy, not a default rejection.

Duplicate names in one source return `ErrDuplicateCurie`. User declarations win
over registered declarations. `RegisterCurie` derives `templated` and panics on
invalid names, empty hrefs, or strict href/templated violations;
`RegisterStatic` panics likewise.

Embedded `*Envelope` children inherit parent declarations. A child-local
declaration overrides the parent for that child and its descendants. Children
emit declarations only for prefixes they use that are not already inherited, so
redundant child declarations are avoided. Raw embedded values do not
participate in CURIE scope.

### Precomputed links

`CompilePrecomputedLinks` and `WrapPrecomputed` validate that the input is:

- valid UTF-8 JSON and a JSON object;
- free of duplicate object keys at any depth (typed `ErrDuplicateJSONKey`);
- a relation map whose names pass default relation hardening and collision
  checks;
- link values shaped as objects or arrays of objects;
- `href` present and a string; known optional fields have the correct JSON type;
- `curies` an array of Link Objects with valid, non-duplicate names.

Unknown link fields pass through unchanged. Empty relation arrays are allowed.
Caller bytes are copied, so later mutation cannot corrupt a handle or envelope.

`CompilePrecomputedLinksStrict` sets both strict link and strict CURIE flags and
performs those validations. `WrapPrecomputed` compiles using the instance's
strict policy. A strict instance rejects a non-empty reusable handle that lacks
the matching strict flag with deferred `ErrInvalidLink` or `ErrInvalidCurie`.
A nil handle has `WrapRaw` semantics; the zero handle is an empty link set and
is always valid.

Precomputed CURIE declarations are cached on the handle and used when
registered declarations are merged. Registered CURIEs still resolve at wrap
time, preserving cross-instance reuse. Cached bytes are preserved except for
deliberate `_embedded` splicing and a byte-splice of only the `curies` member
when registered declarations are added; all other cached bytes and unknown
fields remain untouched.

### Static links

`RegisterStatic` panics for a nil instance, a nil or non-pointer target, an
invalid relation, an invalid/duplicate CURIE declaration, or a strict
link/relation/CURIE violation. A typed-nil pointer target is accepted. Empty
links suppress generators and emit no `_links`. Registered CURIEs resolve at
wrap time, not registration time. Static links apply the instance shape policy
at registration.

### Generated links

Generator output is serialized through the same relation-multiplicity, shape,
relation-policy, and strict-link rules as `AddLink`. User-provided CURIE
declarations are normalized to an array and merged with registered declarations;
duplicate names within one source are rejected with `ErrDuplicateCurie`, and
invalid declarations return `ErrInvalidCurie`. Generated declarations derive
`templated` from the parser.

### Strict JSON

`WithStrictJSON` scans each envelope's marshaled non-empty Data object and each
raw embedded resource for duplicate member names at any depth. It applies to
generated, raw, static, precomputed, and compiled envelope paths, and the policy
is inherited by embedded child envelopes transitively (the effective policy is
parent OR child). A strict parent therefore covers a permissive child, while a
strict child stays strict under a permissive parent. It does not rescan final
output, precomputed links (those are scanned at compile time), or a child's
already-scanned output. Default marshaling stays permissive apart from
precomputed input scanning.

RFC 8259 §4 makes unique names a SHOULD, not a HAL MUST: duplicate names are
valid JSON but non-interoperable, so this is opt-in hardening. Errors are
`ErrDuplicateJSONKey` and `*DuplicateKeyError{Key, Offset}`; `Offset` is relative
to the scanned value (the root Data object or that embedded resource), not the
final document. The key display is capped and errors remain visible through
`errors.Is`/`errors.As`, including through `json.MarshalerError`.

### Envelope mutation

Static and precomputed envelopes are immutable for links: `AddLink` and
`AddCachedLink` record a deferred `ErrAddLinkPrecomputed`, returned by
marshaling, and add nothing. Generated and raw envelopes continue to accept
links. `AddEmbedded` is allowed on all envelope kinds. Invalid relations are
recorded as deferred `ErrInvalidRelation`.

## Media types and profiles

`MediaTypeHAL` is `application/hal+json`; `MediaTypeVendorHAL` is
`application/vnd.hal+json` and carries no optional parameters.

```go
ct, err := hal.ContentType(
    "https://example.com/profiles/order",
    "https://example.com/profiles/order-v2",
)
// application/hal+json; profile="... ..."
```

`ContentType` returns the bare media type for zero profiles and joins multiple
profiles into one space-separated RFC 6906 profile value. Each profile must be an
absolute URI with a scheme; `https:`, `urn:`, `mailto:`, and `tag:` forms are
accepted, while bare names, relative paths, network-path references, query-only
and fragment-only references, and non-ASCII IRIs are rejected with
`ErrInvalidProfile` wrapping `uriref.ErrInvalidURIReference`. Profile bytes are
preserved as supplied and content is not negotiated.

The vendor type `application/vnd.hal+json` has no optional parameters in its IANA
registration, so the `profile` parameter belongs to `application/hal+json` only;
`ContentType` never emits the vendor type.

The serializer never adds a `self` or `profile` link automatically. Add them
explicitly when required by draft §8.1 or §7.1.

## Collections

`CollectionPage` remains caller-owned. `Collection` wraps items through the
instance, applies link/shape policy to the self link, and emits the project
`_embedded.items` convention. There is no `CollectionPage.Validate` and no
`CollectionPage.MarshalJSON`; callers that construct a page manually are
responsible for its contents.

## Client support (deferred)

Client-side HAL navigation is **deferred**, not rejected. Draft-11 makes client
cache support optional (§5.4/§8.4), and bundling it would force premature shared
internals. A future separate `halclient` package is planned to provide a pure
`Decode` function, shared RFC 6570/relation/CURIE handling, and no HTTP
transport. This release deliberately does not include `halclient`.

## Concurrency

`Instance` registration is guarded by a read/write mutex. Compiled
`PrecomputedLinks` handles are read-only by convention after compilation. An
`Envelope` may be marshaled repeatedly and concurrently as long as it is not
mutated; `AddLink`, `AddEmbedded`, and `AddCachedLink` are not safe to call
concurrently with marshaling. Passing an envelope by value copies its internal
map references, so link maps are shared unless the envelope is reconstructed.

Embedded depth and ancestor tracking are allocated per marshal, so repeated and
concurrent marshals of the same graph do not share traversal state.

## OpenAPI adapter

The optional `hal/openapi` adapter:

- tolerates nil documents, components, schemas, and item references;
- injects a standard Link schema unless the document already defines one;
- adds `_links`, `_embedded`, `count`, and `total` shapes as read-only;
- models `Link | []Link` with `oneOf` and normalizes missing item refs.

It neither generates OpenAPI from Go code nor validates runtime responses, and
it does not model the new strict options or shape policy.

## Sources

- [draft-kelly-json-hal-11](https://datatracker.ietf.org/doc/html/draft-kelly-json-hal-11)
  — §4.1 reserved properties, §5 Link Objects, §6 examples, §8.1 self links,
  §8.3 HAL curies, §8.4 hypertext cache pattern.
- [draft-kelly-json-hal-08](https://datatracker.ietf.org/doc/html/draft-kelly-json-hal-08)
  — older revision used by some clients.
- [IANA media types: application/vnd.hal+json](https://www.iana.org/assignments/media-types/application/vnd.hal+json)
  — registered vendor media type. `application/hal+json` is not registered.
- [RFC 3986](https://www.rfc-editor.org/rfc/rfc3986) — URI generic syntax.
- [RFC 6570](https://www.rfc-editor.org/rfc/rfc6570) — URI Template syntax.
- [RFC 6906](https://www.rfc-editor.org/rfc/rfc6906) — the `profile` media type
  parameter and link relation.
- [RFC 8288](https://www.rfc-editor.org/rfc/rfc8288) — Web Linking.
- [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) — JSON.
