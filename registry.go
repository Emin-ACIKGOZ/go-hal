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
	fullJSON = append(fullJSON, `"`+fieldLinks+`":`...)
	fullJSON = append(fullJSON, linksJSON...)
	fullJSON = append(fullJSON, `}`...)

	i.mu.Lock()
	defer i.mu.Unlock()
	// Store as precomputed for this type
	i.precomputed[targetType] = &PrecomputedLinks{
		JSON:      fullJSON,
		prefixes:  linkRelationPrefixes(links),
		validated: true,
	}
}

// PrecomputedLinks holds validated, wrapped pre-serialized links JSON.
//
// The JSON field is read-only and owned by the handle. Its zero value means an
// empty link set with no _links field. Use CompilePrecomputedLinks to build a
// handle from caller-supplied JSON.
type PrecomputedLinks struct {
	JSON      []byte   // wrapped {"_links":...}; read-only; owned by handle
	prefixes  []string // sorted unique CURIE prefixes used by relations
	validated bool
}

// CompilePrecomputedLinks validates and copies caller-supplied link JSON once,
// producing a handle that can be reused across envelopes and instances.
func CompilePrecomputedLinks(linksJSON []byte) (*PrecomputedLinks, error) {
	normalized, prefixes, err := normalizePrecomputedLinks(linksJSON)
	if err != nil {
		return nil, err
	}
	return &PrecomputedLinks{JSON: normalized, prefixes: prefixes, validated: true}, nil
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
		return i.wrapPrecomputedHandle(data, pre)
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
	links, err := CompilePrecomputedLinks(linksJSON)
	if err != nil {
		return &Envelope{Data: data, instance: i, err: err, premanaged: true}
	}
	return i.wrapPrecomputedHandle(data, links)
}

// WrapPrecomputedLinks wraps data with an already-compiled link handle. The
// handle is reusable across envelopes and instances; its JSON is not copied or
// revalidated. A nil handle has WrapRaw semantics, while a non-nil zero handle
// means an empty link set.
func (i *Instance) WrapPrecomputedLinks(data any, links *PrecomputedLinks) *Envelope {
	return i.wrapPrecomputedHandle(data, links)
}

var errPrecomputedUnvalidated = errors.New("hal: unvalidated precomputed links handle")

func (i *Instance) wrapPrecomputedHandle(data any, links *PrecomputedLinks) *Envelope {
	if links == nil {
		return i.WrapRaw(data)
	}
	e := &Envelope{Data: data, instance: i, premanaged: true}
	switch {
	case len(links.JSON) == 0:
		// Empty precomputed set: suppress generators and emit no _links.
	case !links.validated:
		e.err = errPrecomputedUnvalidated
	default:
		e.precomputedJSON = links.JSON
	}
	return e
}

// normalizePrecomputedLinks wraps raw link relations in a top-level "_links"
// object. It accepts the already-wrapped form used internally by RegisterStatic
// so callers can pass either representation. Invalid input is reported instead
// of being spliced into the output.
func normalizePrecomputedLinks(linksJSON []byte) ([]byte, []string, error) {
	linksJSON = bytes.TrimSpace(linksJSON)
	if len(linksJSON) == 0 {
		return nil, nil, nil
	}
	fields, err := decodePrecomputedObject(linksJSON)
	if err != nil {
		return nil, nil, err
	}

	if rawLinks, ok := fields[fieldLinks]; ok {
		return normalizeWrappedPrecomputedLinks(linksJSON, fields, rawLinks)
	}

	if err := validatePrecomputedLinkRelations(fields); err != nil {
		return nil, nil, err
	}
	return wrapPrecomputedLinks(linksJSON), rawRelationPrefixes(fields), nil
}

func decodePrecomputedObject(linksJSON []byte) (map[string]stdjson.RawMessage, error) {
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
	return fields, nil
}

func normalizeWrappedPrecomputedLinks(
	linksJSON []byte,
	fields map[string]stdjson.RawMessage,
	rawLinks stdjson.RawMessage,
) ([]byte, []string, error) {
	if len(fields) != 1 {
		return nil, nil, errors.New("hal: wrapped precomputed links must contain only _links")
	}
	var links map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(rawLinks, &links); err != nil || links == nil {
		return nil, nil, errors.New("hal: precomputed _links must be a JSON object")
	}
	if err := validatePrecomputedLinkRelations(links); err != nil {
		return nil, nil, err
	}
	// Own the wrapped form too: callers may reuse or mutate their buffer.
	return bytes.Clone(linksJSON), rawRelationPrefixes(links), nil
}

func wrapPrecomputedLinks(linksJSON []byte) []byte {
	wrapped := make([]byte, 0, len(linksJSON)+precomputedLinksPrefixLen+precomputedLinksWrapperLen)
	wrapped = append(wrapped, `{"_links":`...)
	wrapped = append(wrapped, linksJSON...)
	wrapped = append(wrapped, '}')
	return wrapped
}

// validatePrecomputedLinkRelations checks the HAL link relation shape before
// pre-serialized links are trusted for byte splicing.
func validatePrecomputedLinkRelations(links map[string]stdjson.RawMessage) error {
	for rel, raw := range links {
		if strings.TrimSpace(rel) == "" {
			return errors.New("hal: precomputed link relation must not be empty")
		}
		if err := validatePrecomputedRelationValue(rel, raw); err != nil {
			return err
		}
	}
	if raw, ok := links[relCuries]; ok {
		if err := validatePrecomputedCuries(raw); err != nil {
			return fmt.Errorf("hal: precomputed curies: %w", err)
		}
	}
	return nil
}

func validatePrecomputedCuries(raw stdjson.RawMessage) error {
	var declarations []Link
	if err := stdjson.Unmarshal(raw, &declarations); err != nil {
		return errors.New("must be an array of link objects")
	}
	return validateCurieDeclarations(declarations)
}

type jsonValueKind uint8

const (
	jsonNull jsonValueKind = iota
	jsonBool
	jsonString
	jsonNumber
	jsonObject
	jsonArray
	jsonInvalid
)

func (k jsonValueKind) String() string {
	switch k {
	case jsonNull:
		return "null"
	case jsonBool:
		return "boolean"
	case jsonString:
		return "string"
	case jsonNumber:
		return "number"
	case jsonObject:
		return "object"
	case jsonArray:
		return "array"
	default:
		return "invalid JSON"
	}
}

// linkValueKind classifies an already-validated raw JSON value by its first
// non-whitespace byte. It avoids decoding values that only need a type check.
func linkValueKind(raw []byte) jsonValueKind {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return jsonInvalid
	}
	switch trimmed[0] {
	case 'n':
		return jsonNull
	case 't', 'f':
		return jsonBool
	case '"':
		return jsonString
	case '{':
		return jsonObject
	case '[':
		return jsonArray
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return jsonNumber
	default:
		return jsonInvalid
	}
}

var precomputedLinkSchema = map[string]jsonValueKind{
	"href":        jsonString,
	"templated":   jsonBool,
	"type":        jsonString,
	"deprecation": jsonString,
	"name":        jsonString,
	"profile":     jsonString,
	"title":       jsonString,
	"hreflang":    jsonString,
	"method":      jsonString,
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
		}
		return nil
	}

	if rel == relCuries {
		return errors.New("hal: precomputed curies relation must be an array of link objects")
	}
	return validatePrecomputedLinkObject(rel, trimmed)
}

func validatePrecomputedLinkObject(rel string, raw stdjson.RawMessage) error {
	if linkValueKind(raw) != jsonObject {
		return fmt.Errorf("hal: precomputed relation %q must contain link objects", rel)
	}
	var obj map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(raw, &obj); err != nil || obj == nil {
		return fmt.Errorf("hal: precomputed relation %q must contain link objects", rel)
	}

	href, ok := obj["href"]
	if !ok {
		return fmt.Errorf("hal: precomputed relation %q link is missing href", rel)
	}
	if err := requireJSONKind(href, jsonString); err != nil {
		return fmt.Errorf("hal: precomputed relation %q href must be a string", rel)
	}

	for field, want := range precomputedLinkSchema {
		if field == "href" {
			continue
		}
		if value, ok := obj[field]; ok {
			if err := requireJSONKind(value, want); err != nil {
				return fmt.Errorf(
					"hal: precomputed relation %q link field %q must be %s",
					rel, field, want,
				)
			}
		}
	}
	return nil
}

func requireJSONKind(raw stdjson.RawMessage, want jsonValueKind) error {
	if got := linkValueKind(raw); got != want {
		return fmt.Errorf("value has type %s, want %s", got, want)
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
			Rel:       relCuries,
			Name:      name,
			Href:      i.curies[name],
			Templated: true,
		})
	}
	return curies
}
