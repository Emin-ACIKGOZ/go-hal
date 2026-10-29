// Copyright (c) 2025-2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	json "github.com/goccy/go-json"
)

// defaultLinksCapacity is the default capacity for the links map.
// It is set to 4 to handle typical use cases of 1-4 links per resource.
const defaultLinksCapacity = 4

// Generator defines a function that produces HAL links for a specific type.
// The function receives the data to generate links for and returns a slice of Link objects.
//
// # Example
//
//	Generator: func(ctx context.Context, u *User) []Link {
//	    return []Link{{Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)}}
//	}
type Generator func(ctx context.Context, v any) []Link

// Instance maintains a registry of link generators and CURIE definitions.
// It is safe for concurrent use.
//
// # Creating an Instance
//
//	// Basic instance
//	inst := hal.New()
//
//	// With strict mode
//	inst := hal.New(hal.WithStrictMode())
type Instance struct {
	mu          sync.RWMutex
	generators  map[reflect.Type]Generator
	precomputed map[reflect.Type]*PrecomputedLinks // OPTIMIZATION: static pre-computed
	curies      map[string]string
	strictMode  bool
}

// New creates a new HAL Instance.
//
// The instance maintains its own registry of generators, allowing for isolated testing.
// Options can be provided to configure the instance:
//
//	inst := hal.New(hal.WithStrictMode())
func New(opts ...InstanceOption) *Instance {
	i := &Instance{
		generators:  make(map[reflect.Type]Generator),
		precomputed: make(map[reflect.Type]*PrecomputedLinks),
		curies:      make(map[string]string),
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

// DefaultInstance is the global singleton registry used by package-level functions.
// If you need isolated registries for testing, create your own with New().
var DefaultInstance = New()

// Register registers a generator function for type T on the DefaultInstance.
// The generator is called at runtime to produce links for wrapped resources.
//
// # Type Requirements
//
//   - T must be a pointer type (e.g., *User not User)
//   - Generators are registered for the pointee type
//
// # Example
//
//	hal.Register(func(ctx context.Context, u *User) []Link {
//	    return []Link{
//	        {Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)},
//	        {Rel: "edit", Href: fmt.Sprintf("/users/%d/edit", u.ID)},
//	    }
//	})
func Register[T any](gen func(context.Context, *T) []Link) {
	RegisterInstance(DefaultInstance, gen)
}

// RegisterCurie registers a CURIE (Compact URI) prefix on the DefaultInstance.
// CURIEs allow shortened link relations like "acme:widget" that expand to full URIs.
//
// # Example
//
//	hal.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")
func RegisterCurie(prefix, href string) {
	DefaultInstance.RegisterCurie(prefix, href)
}

// Wrap wraps data in a HAL Envelope using the DefaultInstance.
// The envelope will inject _links during JSON serialization based on registered generators.
//
// # Example
//
//	user := &User{ID: 42, Name: "Alice"}
//	env := hal.Wrap(context.Background(), user)
//	json.Marshal(env)
func Wrap(ctx context.Context, data any) *Envelope {
	return DefaultInstance.Wrap(ctx, data)
}

// --- Instance Methods ---

// RegisterCurie adds a CURIE (Compact URI) mapping to the instance.
// These are used to shorten link relations in the output JSON.
func (i *Instance) RegisterCurie(prefix, href string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.curies[prefix] = href
}

// RegisterInstance registers a generator using reflection.
// Deprecated: Use the generic function RegisterInstance[T] instead.
func (i *Instance) RegisterInstance(gen any) {
	i.registerReflect(gen)
}

// RegisterInstance binds a strongly-typed generator function to the provided Instance.
// The generator will be invoked whenever Wrap is called with a value of type *T.
func RegisterInstance[T any](i *Instance, gen func(context.Context, *T) []Link) {
	targetType := reflect.TypeOf((*T)(nil))
	adapter := func(ctx context.Context, v any) []Link {
		return gen(ctx, v.(*T))
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	i.generators[targetType] = adapter
}

// RegisterStatic registers pre-computed links for a type.
// Links are serialized once at registration time, providing ~60% performance improvement
// over runtime generator calls. Best for static links that don't change per request.
//
// # Performance
//
// Pre-computed links bypass generator invocation and JSON serialization on every request.
// This is optimal for links like "self" hrefs that only depend on the resource ID.
//
// # Example
//
//	inst := hal.New()
//	hal.RegisterStatic(inst, &User{}, []hal.Link{
//	    {Rel: "self", Href: "/users"},
//	})
//
//	// At runtime - uses pre-computed links automatically
//	env := inst.Wrap(ctx, &User{ID: 42})
func RegisterStatic(i *Instance, target any, links []Link) {
	targetType := reflect.TypeOf(target)

	// Pre-serialize links JSON once, preserving repeated relations as arrays.
	linksMap := make(map[string]any, len(links))
	for _, l := range links {
		appendLinkValue(linksMap, l.Rel, l)
	}
	linksJSON, _ := json.Marshal(linksMap)

	// Wrap in _links object for easy splice
	fullJSON := make([]byte, 0, len(linksJSON)+precomputedLinksWrapperLen+precomputedLinksPrefixLen)
	fullJSON = append(fullJSON, `{`...)
	fullJSON = append(fullJSON, `"_links":`...)
	fullJSON = append(fullJSON, linksJSON...)
	fullJSON = append(fullJSON, `}`...)

	i.mu.Lock()
	defer i.mu.Unlock()
	// Store as precomputed for this type
	i.precomputed[targetType] = &PrecomputedLinks{JSON: fullJSON}
}

// PrecomputedLinks stores pre-computed links JSON
type PrecomputedLinks struct {
	JSON []byte
}

// introspection hook for the unsafe method
func (i *Instance) registerReflect(gen any) {
	genVal := reflect.ValueOf(gen)
	targetType := genVal.Type().In(1)

	adapter := func(ctx context.Context, v any) []Link {
		in := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(v)}
		out := genVal.Call(in)
		return out[0].Interface().([]Link)
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	i.generators[targetType] = adapter
}

// Wrap creates a HAL Envelope with computed links.
// It first checks for pre-computed links (via RegisterStatic), then falls back to generators.
// This is the primary method for wrapping data in HAL envelopes.
//
// # Automatic Optimization
//
// If RegisterStatic was called for the data's type, links are pre-computed and this method
// automatically uses them, providing ~60% better performance.
func (i *Instance) Wrap(ctx context.Context, data any) *Envelope {
	t := reflect.TypeOf(data)

	// OPTIMIZATION: Check for precomputed first
	i.mu.RLock()
	pre, hasPre := i.precomputed[t]
	i.mu.RUnlock()
	if hasPre && pre != nil {
		return &Envelope{
			Data:            data,
			instance:        i,
			precomputedJSON: pre.JSON,
		}
	}

	e := &Envelope{
		Data:     data,
		instance: i,
		links:    make(map[string]any, defaultLinksCapacity),
	}
	e.computeLinks(ctx)
	return e
}

// WrapPrecomputed wraps data with pre-serialized links JSON.
// This is the absolute fastest option - bypasses all generator calls.
//
// # Performance
//
// Provides ~65% better performance than Wrap by eliminating:
//   - Generator invocation
//   - JSON serialization of links
//
// # Input Format
//
// linksJSON must be a JSON object containing link relations:
//
//	{"self": {"href": "/users/42"}}
//
// # Example
//
//	links := []byte(`{"self":{"href":"/users/42"}})
//	env := inst.WrapPrecomputed(ctx, &User{ID: 42}, links)
//	json.Marshal(env)
func (i *Instance) WrapPrecomputed(_ context.Context, data any, linksJSON []byte) *Envelope {
	normalized, err := normalizePrecomputedLinks(linksJSON)
	return &Envelope{
		Data:            data,
		instance:        i,
		precomputedJSON: normalized,
		precomputedErr:  err,
	}
}

// normalizePrecomputedLinks wraps raw link relations in a top-level "_links"
// object. It accepts the already-wrapped form used internally by RegisterStatic
// so callers can pass either representation. Invalid input is reported instead
// of being spliced into the output.
func normalizePrecomputedLinks(linksJSON []byte) ([]byte, error) {
	linksJSON = bytes.TrimSpace(linksJSON)
	if len(linksJSON) == 0 {
		return nil, nil
	}
	if !utf8.Valid(linksJSON) || !stdjson.Valid(linksJSON) {
		return nil, errors.New("hal: precomputed links must be valid UTF-8 JSON")
	}
	if err := findDuplicateJSONKeys(linksJSON); err != nil {
		return nil, fmt.Errorf("hal: precomputed links contain duplicate JSON keys: %w", err)
	}

	var fields map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(linksJSON, &fields); err != nil {
		return nil, fmt.Errorf("hal: precomputed links must be a JSON object: %w", err)
	}
	if fields == nil {
		return nil, errors.New("hal: precomputed links must be a JSON object")
	}

	if rawLinks, ok := fields["_links"]; ok {
		if len(fields) != 1 {
			return nil, errors.New("hal: wrapped precomputed links must contain only _links")
		}
		var links map[string]stdjson.RawMessage
		if err := stdjson.Unmarshal(rawLinks, &links); err != nil || links == nil {
			return nil, errors.New("hal: precomputed _links must be a JSON object")
		}
		if err := validatePrecomputedLinkRelations(links); err != nil {
			return nil, err
		}
		return linksJSON, nil
	}

	if err := validatePrecomputedLinkRelations(fields); err != nil {
		return nil, err
	}

	wrapped := make([]byte, 0, len(linksJSON)+precomputedLinksPrefixLen+precomputedLinksWrapperLen)
	wrapped = append(wrapped, `{"_links":`...)
	wrapped = append(wrapped, linksJSON...)
	wrapped = append(wrapped, '}')
	return wrapped, nil
}

// validatePrecomputedLinkRelations checks the HAL link relation shape before
// pre-serialized links are trusted for byte splicing.
func validatePrecomputedLinkRelations(links map[string]stdjson.RawMessage) error {
	for rel, raw := range links {
		if rel == "" {
			return errors.New("hal: precomputed link relation must not be empty")
		}
		if err := validatePrecomputedRelationValue(rel, raw); err != nil {
			return err
		}
	}
	return nil
}

func validatePrecomputedRelationValue(rel string, raw stdjson.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return fmt.Errorf("hal: precomputed relation %q must be a link object or array of link objects", rel)
	}

	if trimmed[0] == '[' {
		var items []stdjson.RawMessage
		if err := stdjson.Unmarshal(trimmed, &items); err != nil {
			return fmt.Errorf("hal: precomputed relation %q must be an array of link objects: %w", rel, err)
		}
		for i, item := range items {
			if err := validatePrecomputedLinkObject(rel, item); err != nil {
				return fmt.Errorf("hal: precomputed relation %q element %d: %w", rel, i, err)
			}
			if rel == "curies" {
				if err := requireStringField(item, "name"); err != nil {
					return fmt.Errorf("hal: precomputed curie %q element %d: %w", rel, i, err)
				}
			}
		}
		return nil
	}

	if rel == "curies" {
		return errors.New("hal: precomputed curies relation must be an array of link objects")
	}
	return validatePrecomputedLinkObject(rel, trimmed)
}

func validatePrecomputedLinkObject(rel string, raw stdjson.RawMessage) error {
	var obj map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(raw, &obj); err != nil || obj == nil {
		return fmt.Errorf("hal: precomputed relation %q must contain link objects", rel)
	}
	hrefRaw, ok := obj["href"]
	if !ok {
		return fmt.Errorf("hal: precomputed relation %q link is missing href", rel)
	}
	var href string
	if err := stdjson.Unmarshal(hrefRaw, &href); err != nil {
		return fmt.Errorf("hal: precomputed relation %q href must be a string", rel)
	}
	return nil
}

func requireStringField(raw stdjson.RawMessage, field string) error {
	var obj map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(raw, &obj); err != nil || obj == nil {
		return errors.New("must be a link object")
	}
	value, ok := obj[field]
	if !ok {
		return fmt.Errorf("link object is missing %s", field)
	}
	var str string
	if err := stdjson.Unmarshal(value, &str); err != nil {
		return fmt.Errorf("link object %s must be a string", field)
	}
	return nil
}

// WrapRaw creates an envelope without computing links.
// Use this for cases where context-based link generation is not needed.
func (i *Instance) WrapRaw(data any) *Envelope {
	return &Envelope{
		Data:     data,
		instance: i,
		links:    make(map[string]any, defaultLinksCapacity),
	}
}

func (i *Instance) lookupGenerator(t reflect.Type) (Generator, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	gen, ok := i.generators[t]
	return gen, ok
}

// RegisteredTypes returns a list of all Go types that have a generator registered.
// This is a Read-Only introspection hook useful for adapters (like OpenAPI).
func (i *Instance) RegisteredTypes() []reflect.Type {
	i.mu.RLock()
	defer i.mu.RUnlock()
	types := make([]reflect.Type, 0, len(i.generators))
	for t := range i.generators {
		types = append(types, t)
	}
	return types
}

func (i *Instance) resolveCuries(links map[string]any) []Link {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if len(i.curies) == 0 || len(links) == 0 {
		return nil
	}

	used := make(map[string]struct{})
	for rel := range links {
		if idx := strings.IndexByte(rel, ':'); idx > 0 {
			prefix := rel[:idx]
			if _, ok := i.curies[prefix]; ok {
				used[prefix] = struct{}{}
			}
		}
	}
	if len(used) == 0 {
		return nil
	}

	names := make([]string, 0, len(used))
	for name := range used {
		names = append(names, name)
	}
	sort.Strings(names)

	curies := make([]Link, 0, len(names))
	for _, name := range names {
		curies = append(curies, Link{
			Rel:       "curies",
			Name:      name,
			Href:      i.curies[name],
			Templated: true,
		})
	}
	return curies
}
