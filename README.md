# go-hal

A HAL (Hypertext Application Language) serializer for Go. It wraps Go values in
an envelope and injects `_links` during JSON marshaling, with typed link
generators, reusable precomputed links, CURIEs, and an optional OpenAPI
adapter. `AddEmbedded` emits arbitrary embedded resources, and `CollectionPage`
also emits a project-specific `_embedded.items` convention.

The implementation targets the supported subset of
`draft-kelly-json-hal-11` (an expired IETF Informational draft, not an RFC).
Compliance is deliberately scoped: see [COMPLIANCE.md](COMPLIANCE.md) for the
support matrix, validation boundaries, and the list of features that are out of
scope. In particular, `application/hal+json` is not an IANA-registered media
type; the registered vendor type is `application/vnd.hal+json`.

## Requirements

Go **1.26** or newer. Go 1.22, 1.24, and 1.25 are end-of-life and are no longer
tested. The previous v1.1.0 release remains available for older toolchains.

## Minimal Example

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	// Alias the import to ensure the `hal` prefix is stable
	hal "github.com/Emin-ACIKGOZ/go-hal"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func init() {
	// Register a generator for *User.
	hal.Register(func(ctx context.Context, u *User) []hal.Link {
		return []hal.Link{
			{Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)},
		}
	})
}

func main() {
	user := &User{ID: 101, Name: "Alice"}
	response := hal.Wrap(context.Background(), user)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(response); err != nil {
		panic(err)
	}

	// Output:
	// {
	//   "id": 101,
	//   "name": "Alice",
	//   "_links": {
	//     "self": { "href": "/users/101" }
	//   }
	// }
}
```

Both `Envelope` values and `*Envelope` pointers marshal as HAL resources.
`AddLink` remains a pointer method. A nil `*Envelope` marshals as JSON `null`,
following `encoding/json` semantics.

## Non-Goals

`go-hal` does not provide:

* **Client-side Navigation** — deferred to a planned separate `halclient`
  package; this module is a server-side serializer.
* **Code Generation** — no OpenAPI-to-Go or Go-to-OpenAPI generation.
* **Router Integration** — URL construction and HTTP methods remain yours.
* **Implicit Auto-Discovery** — links are generated only for registered types.
  No self or profile link is added automatically.
* **Media-Type Negotiation** — `ContentType` formats a value; choosing and
  negotiating the media type in handlers remains yours.

## Features

* **Typed Generators** — the generic `Register` / `RegisterInstance` API binds
  generators without `reflect.Call` at runtime. The deprecated
  `Instance.RegisterInstance(any)` form still uses reflection.
* **HAL Link Semantics** — one relation value is an object; repeated values
  become an array of objects.
* **Dual-Mode API** — a global singleton for convenience or isolated instances
  for unit testing.
* **Validated Precomputed Links** — compile once, reuse across envelopes and
  instances.
* **CURIE Support** — registered prefixes and user declarations are merged with
  user-wins precedence; invalid or duplicate declarations are rejected.
* **Embedded Resources** — `AddEmbedded` for raw objects, maps, slices, and
  nested `*Envelope` values, with depth/cycle protection and the §8.4 cache
  pattern via `AddCachedLink`.
* **Defensive JSON Handling** — reserved HAL keys, escaped reserved keys,
  duplicate keys in precomputed input, invalid UTF-8, and non-object roots are
  rejected before output is produced. `WithStrictJSON` extends duplicate-key
  checks to each marshaled Data object and raw embedded resource, inherited
  transitively by embedded child envelopes.
* **Opt-in Strict Modes** — `WithStrictLinks`, `WithStrictRelations`,
  `WithStrictCuries`, and `WithStrictJSON` are independent; defaults stay
  permissive apart from the documented invalid-name/collision hardening.
* **Media Helpers** — `MediaTypeHAL`, `MediaTypeVendorHAL`, and
  `ContentType(profiles...)`.

## Embedded Resources

```go
parent := inst.Wrap(ctx, &Book{Title: "The Way of Zen"})
parent.AddEmbedded("author", &Person{Name: "Alan Watts"})
parent.AddCachedLink(
    hal.Link{Rel: "author", Href: "/people/alan-watts"},
    &Person{Name: "Alan Watts"},
)
```

An embedded value is a Resource Object. One value emits an object; repeated
values emit an array. Slices flatten one level, one-element slices emit an
object, and empty slices are a no-op. Raw structs, maps, and object
`json.RawMessage` values are embedded as-is and are **not** auto-wrapped or
link-generated; use `inst.Wrap` when the child needs its own links. Direct
children must be `*Envelope` (not `Envelope` values) so depth and cycles can be
bounded. The maximum embedded depth is 32; cycles return
`ErrEmbeddedDepthExceeded`.

Static and precomputed envelopes also accept `AddEmbedded`; the cached
`_links` bytes are preserved and `_embedded` is spliced in. They still reject
`AddLink` and `AddCachedLink` with `ErrAddLinkPrecomputed`.

## Strict Mode

By default, `hal.Wrap` is permissive: if no generator is found for a type, the
data is returned without generated links.

**Strict Mode** catches generator mismatches on the generated path. It panics
when:

1. You wrap a value type `T` but registered a generator for `*T`.
2. You wrap a struct value or pointer with no registered generator.

Static and precomputed envelopes do not invoke generators, so they bypass these
checks:

```go
hal.DefaultInstance = hal.New(hal.WithStrictMode())
```

### Link, relation, CURIE, and JSON strictness

These options are independent of `WithStrictMode` and of each other:

* `WithStrictLinks()` validates href URI references and RFC 6570 templates,
  enforces `templated` consistency, and validates `type`, `deprecation`,
  `profile`, and `hreflang`. Media ranges such as `*/*` and unsafe template
  literal bytes are rejected; brackets and multiple fragments are checked by
  the URI-reference validator.
* `WithStrictRelations()` accepts lowercase RFC 8288 `reg-rel-type` tokens,
  absolute URI schemes, and declared CURIEs; it checks grammar only (IANA
  registration is not consulted) and rejects relative references, `:x`,
  mixed-case bare tokens, and RFC 8288-invalid underscores such as `a_b`. An
  undeclared `acme:widget` remains an opaque URI. Default mode accepts `a_b`
  and does not fold it as a bare-token collision; CURIE prefixes may use `_`.
  Enforcing the full grammar and case-variant collisions by default is reserved
  for v2.0.0.
* `WithStrictCuries()` requires ASCII NCName-subset names and a single
  `{rel}`/`{+rel}` template with `templated: true`.
* `WithStrictJSON()` scans each non-empty marshaled Data object and raw embedded
  resource, inherits the policy into child envelopes, and returns
  `ErrDuplicateJSONKey` / `*DuplicateKeyError`.

```go
inst := hal.New(
    hal.WithStrictLinks(),
    hal.WithStrictRelations(),
    hal.WithStrictCuries(),
    hal.WithStrictJSON(),
)
```

`ValidateLink` performs the link checks for one-off validation. Strict
link/relation/CURIE policy is applied to `AddLink`, generated links,
`RegisterStatic`, `Collection`, and precomputed wrapping. Use
`CompilePrecomputedLinksStrict` for reusable handles; strict instances reject
non-strict handles and the zero handle remains valid.

Default hardening still rejects blank/control/invalid-UTF-8 relation names,
bare-token case collisions, and CURIE names containing a colon or control
character.

## Collections

`CollectionPage` and `Collection` are a project convention, not part of HAL:

```go
page := inst.Collection(ctx, users, total, hal.Link{Rel: "self", Href: "/users"})
```

`count` and `total` are always serialized, so `total: 0` stays observable. Empty
`_links` and `_embedded` maps are omitted. Embedded items are wrapped through
the instance, so they keep their own links.

## Precomputed Links

There are two entry points:

```go
// Validates and copies on every call. Convenient for one-off input.
env := inst.WrapPrecomputed(ctx, user, linksJSON)

// Validate once, then reuse. Treat the handle as read-only and instance-independent.
compiled, err := hal.CompilePrecomputedLinks(linksJSON)
if err != nil {
	return err
}
env := inst.WrapPrecomputedLinks(user, compiled)
```

`CompilePrecomputedLinks` owns its bytes: mutating the input slice afterward does
not affect the handle. `CompilePrecomputedLinksStrict` additionally validates
strict links and CURIEs. A strict instance rejects a non-empty handle compiled
without the matching strict flags; an empty (zero) handle is always valid. A nil handle has `WrapRaw` semantics, while a non-nil zero
handle means an empty link set. A hand-built handle whose JSON was never
validated is rejected at marshal time. Static and precomputed envelopes reject
`AddLink` with `ErrAddLinkPrecomputed`; use a generated or raw envelope if you
need to add links dynamically.

`RegisterStatic` serializes links once per instance and type. It panics for a
nil instance, a nil or non-pointer target, empty relations, or invalid/duplicate
CURIE declarations. Registered CURIEs resolve at wrap time, so a late
registration on the same instance is reflected. Only compiled
`PrecomputedLinks` handles are reusable across instances.

## CURIEs

```go
inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")
```

When a relation such as `acme:widget` is present, the matching registered prefix
is emitted in a `curies` array. Unknown prefixes keep the relation but produce
no declaration. User-supplied `curies` declarations win over registered
declarations with the same name. Duplicate names within one source return
`ErrDuplicateCurie`; names that are empty, whitespace/control-bearing,
non-UTF-8, or contain a colon and empty hrefs return `ErrInvalidCurie`.
`WithStrictCuries` additionally requires the ASCII NCName subset.
`RegisterCurie` panics for invalid names, empty hrefs, or strict
href/templated violations.

Generated declarations derive `templated` from the registered href template, so
a declaration never claims `templated: true` for a non-template. Default mode
does not require user declarations to set it and does not validate href
templates. `WithStrictCuries()` requires a single `{rel}` or `{+rel}` expression
and `templated: true`.

Embedded `*Envelope` children inherit parent CURIE declarations. A child-local
declaration overrides the parent for that child and its descendants; children do
not emit redundant declarations for inherited prefixes.

## Self and profile links

Draft §8.1 recommends a `self` link on each Resource Object, and §7.1 recommends
a root `profile` link when the media type carries a `profile` parameter. Both are
authored explicitly; the serializer never injects or mutates links during
marshaling. `Collection` already emits its own `self` link and never adds a
`profile`.

```go
profiles := []string{"https://example.com/profiles/order"}
ct, err := hal.ContentType(profiles...)
if err != nil {
	return err
}

env := inst.Wrap(ctx, order)
env.AddLink(hal.Link{Rel: "self", Href: "/orders/523"}) // relative self is fine
for _, profile := range profiles {
	env.AddLink(hal.Link{Rel: "profile", Href: profile}) // absolute profile URIs
}
_ = ct // serve as the response Content-Type
```

`self` applies to every Resource Object, including embedded ones. The root
`profile` link applies to the document root; embedded resources do not inherit
it. A manually built `CollectionPage` sets its own root profile in `Links`.

## OpenAPI Adapter (`hal/openapi`)

The optional adapter augments OpenAPI 3.0 schemas using `kin-openapi`.

### What it does

* Injects a HAL `_links` / `_embedded` schema into existing components
* Models `Link | []Link` with `oneOf`
* Preserves an existing custom `Link` schema
* Marks HAL collection fields read-only and tolerates nil documents/schemas
* Models the `_embedded.items` collection convention

### What it does *not* do

* It does **not** generate OpenAPI specs from Go code
* It does **not** validate runtime responses
* It does **not** infer links from registered generators

```go
import (
	openapi "github.com/Emin-ACIKGOZ/go-hal/openapi"
)

adapter := openapi.New(doc)
adapter.InjectLinkSchema()
adapter.MakeResource(userSchema)
```

## Migration Guide

### From raff/halgo (Go)

**Before (halgo):**

```go
type User struct {
    halgo.Links
    Name string
}

user := User{
    Links: halgo.Links{}.Self("/users/1"),
    Name: "Alice",
}
json.Marshal(user)
```

**After (go-hal):**

```go
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

hal.Register(func(ctx context.Context, u *User) []hal.Link {
    return []hal.Link{{Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)}}
})

env := hal.Wrap(ctx, &User{ID: 1, Name: "Alice"})
json.Marshal(env)
```

### From pyhalboy (Python)

**Before (pyhalboy):**

```python
resource = halboy.Resource()
resource.set_link('self', '/users/1')
resource.set_property('name', 'Alice')
```

**After (go-hal):**

```go
hal.Register(func(ctx context.Context, u *User) []hal.Link {
    return []hal.Link{{Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)}}
})

env := hal.Wrap(ctx, &User{ID: 1, Name: "Alice"})
json.Marshal(env)
```

### From Spring HATEOAS (Java)

**Before (Spring):**

```java
User user = new User();
user.setId(1);
ControllerLinkBuilder.linkTo(UserController.class)
    .withRel("self")
    .add(user);
```

**After (go-hal):**

```go
hal.Register(func(ctx context.Context, u *User) []hal.Link {
    return []hal.Link{{Rel: "self", Href: "/users/1"}}
})

env := hal.Wrap(ctx, &User{ID: 1})
json.Marshal(env)
```

### Key Differences

| Feature | Other Libraries | go-hal |
|---------|----------------|--------|
| Reflection | Often required | Typed generators avoid `reflect.Call`; reflection is used for registration and the deprecated API |
| Link injection | At creation | At JSON marshal |
| Dependencies | Varies | `goccy/go-json` for encoding; optional `kin-openapi` adapter |
| Type Safety | Dynamic | Compile-time checked typed generators |

## Performance

HAL injection necessarily serializes your data and builds link metadata. The
exact cost depends on the payload, encoder, link count, and whether CURIEs are
resolved. Do not rely on fixed numbers from another machine: measure your own
path.

```sh
go test -run='^$' -bench=. -benchmem -count=10 ./...
```

The three approaches are:

1. **Generated links** (`Register` / `Wrap`): flexible, per-request links.
2. **Static links** (`RegisterStatic`): serialized once per type; best for links
   that do not vary per instance.
3. **Compiled precomputed links** (`CompilePrecomputedLinks` +
   `WrapPrecomputedLinks`): validate once and reuse; best when links are produced
   in a separate pass or cache. `WrapPrecomputed` is the safe one-shot form and
   pays validation plus a copy on every call.

CURIE resolution and reserved-field checks still run at wrap/marshal time, so
precomputed links are not a blanket "zero-cost" path. The pull-request workflow
publishes an advisory `benchstat` comparison; it does not fail builds.

## Constraints and Validation

1. Data must marshal to a JSON object (`{...}`) or `null`. `nil`, typed-nil
   pointers, and empty objects are normalized to an empty resource. Root arrays
   and scalars are rejected.
2. User data must not contain top-level `_links` or `_embedded`; this includes
   escaped spellings such as `\u005flinks`. Conflicts return
   `ErrHALFieldConflict`.
3. Precomputed link input is scanned for duplicate JSON keys. Arbitrary user
   state is not re-scanned for duplicate keys after the encoder produces it;
   use a strict encoder if that matters.
4. `href` is required on Link Objects. By default URI syntax, RFC 6570
   templates, and relation registries are not validated; enable
   `WithStrictLinks`/`WithStrictRelations` or call `ValidateLink`.
5. Static and precomputed envelopes are immutable for links. `AddLink` and
   `AddCachedLink` record a deferred `ErrAddLinkPrecomputed`; `AddEmbedded` is
   allowed and spliced into the output. Invalid relations are deferred as
   `ErrInvalidRelation`.
6. Embedded envelopes are limited to depth 32 and checked for cycles on each
   marshal. Raw embedded graphs are caller-owned.
7. `Instance` registration is concurrency-safe. An `Envelope` may be marshaled
   concurrently only if it is not mutated.

## License

MIT
