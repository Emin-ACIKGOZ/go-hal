# go-hal

A HAL (Hypertext Application Language) serializer for Go. It wraps Go values in
an envelope and injects `_links` and `_embedded` during JSON marshaling, with
typed link generators, reusable precomputed links, CURIEs, and an optional
OpenAPI adapter.

The implementation targets the supported subset of
`draft-kelly-json-hal-11` (an expired IETF Informational draft, not an RFC).
Compliance is deliberately scoped: see [COMPLIANCE.md](COMPLIANCE.md) for the
support matrix, validation boundaries, and the list of features that are out of
scope. In particular, `application/hal+json` is not an IANA-registered media
type; the registered vendor type is `application/vnd.hal+json`.

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

* **Client-side Navigation** — this is a server-side serialization library.
* **Code Generation** — no OpenAPI-to-Go or Go-to-OpenAPI generation.
* **Router Integration** — URL construction and HTTP methods remain yours.
* **Implicit Auto-Discovery** — links are generated only for registered types.
* **`AddEmbedded` Helpers** — arbitrary embedded resources are not exposed.
  `Collection` emits the project-specific `_embedded.items` convention below;
  top-level `_embedded` in wrapped user data is rejected.
* **Media-Type Negotiation** — choose and negotiate the media type in your
  handlers.

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
* **Defensive JSON Handling** — reserved HAL keys, escaped reserved keys,
  duplicate keys in precomputed input, invalid UTF-8, and non-object roots are
  rejected before output is produced.

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

// Validate once, then reuse. The handle is immutable and instance-independent.
compiled, err := hal.CompilePrecomputedLinks(linksJSON)
if err != nil {
	return err
}
env := inst.WrapPrecomputedLinks(user, compiled)
```

`CompilePrecomputedLinks` owns its bytes: mutating the input slice afterward does
not affect the handle. A nil handle has `WrapRaw` semantics, while a non-nil zero
handle means an empty link set. A hand-built handle whose JSON was never
validated is rejected at marshal time. Static and precomputed envelopes reject
`AddLink` with `ErrAddLinkPrecomputed`; use a generated or raw envelope if you
need to add links dynamically.

`RegisterStatic` serializes links once per type. It panics for a nil instance, a
nil or non-pointer target, empty relations, or invalid/duplicate CURIE
declarations. Registered CURIEs resolve at wrap time, so late registrations and
cross-instance reuse work as expected.

## CURIEs

```go
inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")
```

When a relation such as `acme:widget` is present, the matching registered prefix
is emitted in a `curies` array. Unknown prefixes keep the relation but produce
no declaration. User-supplied `curies` declarations win over registered
declarations with the same name. Duplicate names within one source return
`ErrDuplicateCurie`; empty or whitespace-bearing names and empty hrefs return
`ErrInvalidCurie`.

Generated declarations set `templated: true`. User declarations are not required
to set it, and `href` template syntax is not validated (see
[RFC 6570](https://www.rfc-editor.org/rfc/rfc6570); validation is out of scope).

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
4. `href` is required on Link Objects. URI syntax, RFC 6570 templates, and link
   relation registries are not validated.
5. Static and precomputed envelopes are immutable. `AddLink` on them returns
   `ErrAddLinkPrecomputed` from marshaling. An empty relation is also a deferred
   error.
6. `Instance` registration is concurrency-safe. An `Envelope` may be marshaled
   concurrently only if it is not mutated.

## License

MIT
