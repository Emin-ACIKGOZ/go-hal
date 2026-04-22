# go-hal

A high-performance, strictly compliant [HAL (Hypertext Application Language)](https://stateless.group/hal_specification.html) library for Go. It focuses on zero-reflection runtime execution, type safety, and allocation-efficient HAL augmentation during JSON serialization.

**Target Audience:** This library is designed for senior Go developers building high-throughput microservices where HAL compliance and allocation-efficient JSON serialization are non-negotiable. It is not intended for beginners or those looking for an "all-in-one" web framework.

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
	// 1. Register: Define links for *User (must be pointer type)
	hal.Register(func(ctx context.Context, u *User) []hal.Link {
		return []hal.Link{
			{Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)},
		}
	})
}

func main() {
	// 2. Wrap: Create the HAL envelope
	user := &User{ID: 101, Name: "Alice"}
	response := hal.Wrap(context.Background(), user)

	// 3. Marshal: HAL metadata injection occurs during marshaling
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

## Non-Goals

To maintain speed, predictability, and simplicity, `go-hal` explicitly **does not** support:

* **Client-side Navigation**
  This is a server-side serialization library only.

* **Code Generation**
  No Go structs are generated from OpenAPI, and no OpenAPI is generated from Go code.

* **Router Integration**
  Routes, HTTP methods, and URL construction are your responsibility. No Gin/Chi/Fiber inference.

* **Implicit Auto-Discovery**
  Links are added *only* when you explicitly register a generator for a type.

## Features

* **Zero-Reflection Runtime**
  Generators are compiled into type-safe closures at registration time. No `reflect.Call` in the hot path.

* **Byte-Splicing Performance**
  Injects `_links` and `_embedded` directly into JSON output without allocating intermediate maps.

* **Strict HAL Semantics**
  Correct handling of single vs. multiple links per relation, and object-vs-array polymorphism.

* **Dual-Mode API**
  Use a global singleton for convenience or isolated instances for unit testing.

## Strict Mode

By default, `hal.Wrap` is permissive: if no generator is found for a type, the data is returned as-is.

**Strict Mode** exists to catch developer mistakes early. When enabled, it panics if:

1. You wrap a value type `T` but registered a generator for `*T`.
2. You wrap a type for which no generator exists at all.

Enable it during development or testing:

```go
hal.DefaultInstance = hal.New(hal.WithStrictMode())
```

## OpenAPI Adapter (`hal/openapi`)

The optional `hal/openapi` package bridges HAL with OpenAPI 3.0 schemas using `kin-openapi`.

### What it does

* Injects valid HAL `_links` and `_embedded` schemas into existing OpenAPI components
* Models HAL link polymorphism (`Link | []Link`) correctly using `oneOf`
* Allows contract-first teams to document HAL responses without hand-writing boilerplate

### What it does *not* do

* It does **not** generate OpenAPI specs from Go code
* It does **not** validate runtime responses
* It does **not** infer links from registered generators

### Example

```go
import (
	openapi "github.com/Emin-ACIKGOZ/go-hal/openapi"
)

// Using kin-openapi
adapter := openapi.New(doc)
adapter.InjectLinkSchema()
adapter.MakeResource(userSchema) // Augments schema with HAL fields
```

## Migration Guide

This guide helps migrate from other HAL libraries to go-hal.

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
| Reflection | Required | Zero at runtime |
| Link injection | At creation | At JSON marshal |
| Performance | Baseline | 60%+ faster |
| Dependencies | Multiple | go-json only |

## Performance Tuning

go-hal provides multiple optimization paths. Choose based on your link complexity.

### Performance Comparison

| Method | ns/op (avg) | Improvement | Best For |
|--------|-------------|-------------|---------|
| Wrap + Generator | ~5,500 ns | baseline | Dynamic links |
| RegisterStatic | ~2,100 ns | +62% | Static links |
| WrapPrecomputed | ~1,900 ns | +66% | Precomputed links |

### Optimization Levels

#### Level 1: Baseline (Default)
Use Register() for dynamic links that depend on runtime data.

```go
hal.Register(func(ctx context.Context, u *User) []hal.Link {
    return []hal.Link{{Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)}}
})
env := hal.Wrap(ctx, u)
```

#### Level 2: Static Pre-computed
Use RegisterStatic() for links that are the same per type (just varying IDs).

```go
inst := hal.New()
hal.RegisterStatic(inst, &User{}, []hal.Link{{Rel: "self", Href: "/users"}})

env := inst.Wrap(ctx, &User{ID: 42})
// Automatically uses pre-computed _links
```

#### Level 3: Maximum Performance
Use WrapPrecomputed() when you pre-serialize links yourself.

```go
links := []byte(`{"self":{"href":"/users/42"}}`)
env := inst.WrapPrecomputed(ctx, user, links)
```

This bypasses all link generation for maximum throughput.

### Memory Optimization

go-hal's byte-splicing avoids intermediate allocations:

| Benchmark | Allocations per Op |
|-----------|-----------------|
| Baseline | 14 |
| RegisterStatic | 5 |
| WrapPrecomputed | 5 |

### When to Optimize

1. **RegisterStatic**: Links don't change per request (e.g., `/users` not `/users/123`)
2. **WrapPrecomputed**: You generate links elsewhere (caching, batch jobs)

## Performance & Constraints

This library uses a deliberate **byte-splicing** technique to merge your struct's JSON output with HAL metadata.

### Constraints

1. Your data **must** marshal to a JSON object (`{...}`)
   Root arrays or primitives are not valid HAL resources.

2. Your data **must not** be `nil`
   (unless you intentionally want an empty HAL response).

### Debugging Tip

If you encounter invalid JSON (e.g. `{,"_links":...}`), inspect the raw JSON output of your data struct.
`go-hal` relies on detecting the opening `{` and closing `}` of your payload.


## License

MIT
