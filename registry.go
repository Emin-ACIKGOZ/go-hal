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
	mu              sync.RWMutex
	generators      map[reflect.Type]Generator
	precomputed     map[reflect.Type]*PrecomputedLinks // OPTIMIZATION: static pre-computed
	curies          map[string]string
	strictMode      bool
	strictLinks     bool
	strictRelations bool
	strictCuries    bool
	strictJSON      bool
	linkShape       LinkShape
	relationShapes  map[string]LinkShape
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
// It panics if prefix is empty or contains whitespace, or if href is empty.
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
//
// It panics if prefix is empty or contains whitespace, or if href is empty.
func (i *Instance) RegisterCurie(prefix, href string) {
	if err := validateCurieDeclarations([]Link{{Name: prefix, Href: href, Templated: curieHrefTemplated(href)}}, i.strictCuries); err != nil {
		panic(fmt.Sprintf("hal: RegisterCurie: %v", err))
	}
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
// Links are serialized once at registration time and resolve registered CURIEs
// at wrap time. Best for links that do not change per resource instance.
//
// It panics for a nil instance, a nil or non-pointer target, an empty relation,
// or invalid/duplicate CURIE declarations. A typed-nil pointer target such as
// (*T)(nil) is accepted. Passing no links registers an empty link set that
// suppresses generators and emits no _links.
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
	targetType := validateStaticTarget(i, target)
	pre := compileStaticLinks(links, i)

	i.mu.Lock()
	defer i.mu.Unlock()
	i.precomputed[targetType] = pre
}

func validateStaticTarget(i *Instance, target any) reflect.Type {
	if i == nil {
		panic("hal: RegisterStatic: instance must not be nil")
	}
	targetType := reflect.TypeOf(target)
	if targetType == nil {
		panic("hal: RegisterStatic: target must not be nil")
	}
	if targetType.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("hal: RegisterStatic: target must be a pointer, got %s", targetType))
	}
	return targetType
}

//nolint:cyclop // Registration is a linear validation loop.
func compileStaticLinks(links []Link, inst *Instance) *PrecomputedLinks {
	var curies []Link
	linksMap := make(map[string]any, len(links))
	for _, l := range links {
		if err := validateRelationName(l.Rel, linksMap); err != nil {
			panic(fmt.Sprintf("hal: RegisterStatic: %v", err))
		}
		if inst != nil && inst.strictRelations {
			if err := inst.validateStrictRelation(l.Rel); err != nil {
				panic(fmt.Sprintf("hal: RegisterStatic: %v", err))
			}
		}
		if inst != nil && inst.strictLinks {
			if err := ValidateLink(l); err != nil {
				panic(fmt.Sprintf("hal: RegisterStatic: %v", err))
			}
		}
		if l.Rel == relCuries {
			curies = append(curies, l)
		}
		appendRelationValue(linksMap, l.Rel, l)
	}
	if err := validateCurieDeclarations(curies, inst != nil && inst.strictCuries); err != nil {
		panic(fmt.Sprintf("hal: RegisterStatic: %v", err))
	}
	normalizeStaticCuries(linksMap)
	if inst != nil {
		if err := applyStaticLinkShapes(linksMap, inst); err != nil {
			panic(fmt.Sprintf("hal: RegisterStatic: %v", err))
		}
	}

	pre := &PrecomputedLinks{
		prefixes:     linkRelationPrefixes(links),
		curies:       append([]Link(nil), curies...),
		validated:    true,
		strictLinks:  inst != nil && inst.strictLinks,
		strictCuries: inst != nil && inst.strictCuries,
	}
	if len(links) > 0 {
		pre.JSON = wrapStaticLinks(linksMap)
	}
	return pre
}

func normalizeStaticCuries(linksMap map[string]any) {
	value, ok := linksMap[relCuries]
	if !ok {
		return
	}
	if _, isArray := value.([]any); !isArray {
		linksMap[relCuries] = []any{value}
	}
}

func wrapStaticLinks(linksMap map[string]any) []byte {
	linksJSON, err := json.Marshal(linksMap)
	if err != nil {
		panic(fmt.Sprintf("hal: RegisterStatic: %v", err))
	}
	fullJSON := make([]byte, 0, len(linksJSON)+precomputedLinksWrapperLen+precomputedLinksPrefixLen)
	fullJSON = append(fullJSON, `{`...)
	fullJSON = append(fullJSON, `"`+fieldLinks+`":`...)
	fullJSON = append(fullJSON, linksJSON...)
	fullJSON = append(fullJSON, `}`...)
	return fullJSON
}

// PrecomputedLinks holds validated, wrapped pre-serialized links JSON.
//
// The JSON field is read-only by convention and owned by the handle. Do not
// mutate it after compilation. Its zero value means an empty link set with no
// _links field. Use CompilePrecomputedLinks to build a handle from caller JSON.
type PrecomputedLinks struct {
	JSON         []byte   // wrapped {"_links":...}; read-only; owned by handle
	prefixes     []string // sorted unique CURIE prefixes used by relations
	curies       []Link   // decoded declarations in source order
	validated    bool
	strictLinks  bool
	strictCuries bool
}

// CompilePrecomputedLinks validates and copies caller-supplied link JSON once,
// producing a structurally validated handle that can be reused across envelopes
// and instances. It does not opt into strict link or CURIE semantics.
func CompilePrecomputedLinks(linksJSON []byte) (*PrecomputedLinks, error) {
	return compilePrecomputedLinks(linksJSON, false, false)
}

// CompilePrecomputedLinksStrict is CompilePrecomputedLinks plus strict link and
// strict CURIE validation. The resulting handle can be wrapped by strict
// instances without revalidation.
func CompilePrecomputedLinksStrict(linksJSON []byte) (*PrecomputedLinks, error) {
	return compilePrecomputedLinks(linksJSON, true, true)
}

func compilePrecomputedLinks(linksJSON []byte, strictLinks, strictCuries bool) (*PrecomputedLinks, error) {
	normalized, prefixes, curies, err := normalizePrecomputedLinksPolicy(linksJSON, strictLinks, strictCuries)
	if err != nil {
		return nil, err
	}
	return &PrecomputedLinks{
		JSON:         normalized,
		prefixes:     prefixes,
		curies:       curies,
		validated:    true,
		strictLinks:  strictLinks,
		strictCuries: strictCuries,
	}, nil
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
// It first checks for pre-computed links (via RegisterStatic), then falls back
// to generators. If static links were registered for the data's type, those
// links take precedence and generators are not called.
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

// WrapPrecomputed wraps data with caller-supplied links JSON.
// It validates and copies the input and computes the CURIE prefixes on every
// call. For a hot path, validate once with CompilePrecomputedLinks and reuse the
// handle with WrapPrecomputedLinks.
//
// # Input Format
//
// linksJSON may be raw link relations or the wrapped form:
//
//	{"self": {"href": "/users/42"}}
//	{"_links": {"self": {"href": "/users/42"}}}
//
// Invalid input produces a deferred error during marshaling.
//
// # Example
//
//	links := []byte(`{"self":{"href":"/users/42"}}`)
//	env := inst.WrapPrecomputed(ctx, &User{ID: 42}, links)
//	json.Marshal(env)
func (i *Instance) WrapPrecomputed(_ context.Context, data any, linksJSON []byte) *Envelope {
	links, err := compilePrecomputedLinks(linksJSON, i.strictLinks, i.strictCuries)
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

//nolint:cyclop // Validation cases are intentionally explicit and ordered.
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
	case i.strictLinks && !links.strictLinks:
		e.err = fmt.Errorf("%w: precomputed handle was not compiled with strict links", ErrInvalidLink)
	case i.strictCuries && !links.strictCuries:
		e.err = fmt.Errorf("%w: precomputed handle was not compiled with strict CURIEs", ErrInvalidCurie)
	case i.strictRelations && len(links.JSON) > 0:
		if err := i.validatePrecomputedStrictRelations(links.JSON); err != nil {
			e.err = err
			return e
		}
		fallthrough
	default:
		resolved, err := i.resolvePrecomputedLinks(links)
		if err != nil {
			e.err = err
		} else {
			e.precomputedJSON = resolved
		}
	}
	return e
}

// normalizePrecomputedLinks wraps raw link relations in a top-level "_links"
// object. It accepts the already-wrapped form used internally by RegisterStatic
// so callers can pass either representation. Invalid input is reported instead
// of being spliced into the output.
func normalizePrecomputedLinks(linksJSON []byte) ([]byte, []string, error) {
	normalized, prefixes, _, err := normalizePrecomputedLinksPolicy(linksJSON, false, false)
	return normalized, prefixes, err
}

func normalizePrecomputedLinksPolicy(linksJSON []byte, strictLinks, strictCuries bool) ([]byte, []string, []Link, error) {
	linksJSON = bytes.TrimSpace(linksJSON)
	if len(linksJSON) == 0 {
		return nil, nil, nil, nil
	}
	fields, err := decodePrecomputedObject(linksJSON)
	if err != nil {
		return nil, nil, nil, err
	}

	if rawLinks, ok := fields[fieldLinks]; ok {
		return normalizeWrappedPrecomputedLinks(linksJSON, fields, rawLinks, strictLinks, strictCuries)
	}

	curies, err := validatePrecomputedLinkRelations(fields, strictLinks, strictCuries)
	if err != nil {
		return nil, nil, nil, err
	}
	return wrapPrecomputedLinks(linksJSON), rawRelationPrefixes(fields), curies, nil
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
	strictLinks, strictCuries bool,
) ([]byte, []string, []Link, error) {
	if len(fields) != 1 {
		return nil, nil, nil, errors.New("hal: wrapped precomputed links must contain only _links")
	}
	var links map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(rawLinks, &links); err != nil || links == nil {
		return nil, nil, nil, errors.New("hal: precomputed _links must be a JSON object")
	}
	curies, err := validatePrecomputedLinkRelations(links, strictLinks, strictCuries)
	if err != nil {
		return nil, nil, nil, err
	}
	// Own the wrapped form too: callers may reuse or mutate their buffer.
	return bytes.Clone(linksJSON), rawRelationPrefixes(links), curies, nil
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
func validatePrecomputedLinkRelations(links map[string]stdjson.RawMessage, strictLinks, strictCuries bool) ([]Link, error) {
	names := make([]string, 0, len(links))
	for rel := range links {
		names = append(names, rel)
	}
	for _, rel := range names {
		if err := validateRelationSyntax(rel); err != nil {
			return nil, fmt.Errorf("hal: precomputed %w", err)
		}
	}
	if err := validateRelationCollisions(names); err != nil {
		return nil, fmt.Errorf("hal: precomputed %w", err)
	}

	var curies []Link
	for rel, raw := range links {
		if rel == relCuries {
			continue // validated below with CURIE-specific rules
		}
		if err := validatePrecomputedRelationValue(rel, raw, strictLinks); err != nil {
			return nil, err
		}
	}
	if raw, ok := links[relCuries]; ok {
		decoded, err := validatePrecomputedCuries(raw, strictLinks, strictCuries)
		if err != nil {
			return nil, fmt.Errorf("hal: precomputed curies: %w", err)
		}
		curies = decoded
	}
	return curies, nil
}

func validatePrecomputedCuries(raw stdjson.RawMessage, strictLinks, strictCuries bool) ([]Link, error) {
	_, declarations, err := decodePrecomputedCurieRaw(raw, strictLinks, strictCuries)
	return declarations, err
}

func decodePrecomputedCurieRaw(raw stdjson.RawMessage, strictLinks, strictCuries bool) ([]stdjson.RawMessage, []Link, error) {
	var rawDeclarations []stdjson.RawMessage
	if err := stdjson.Unmarshal(raw, &rawDeclarations); err != nil {
		return nil, nil, errors.New("must be an array of link objects")
	}
	declarations := make([]Link, 0, len(rawDeclarations))
	for i, item := range rawDeclarations {
		var declaration Link
		if err := stdjson.Unmarshal(item, &declaration); err != nil {
			return nil, nil, fmt.Errorf("element %d: %w", i, err)
		}
		declarations = append(declarations, declaration)
	}
	if err := validateCurieDeclarations(declarations, strictCuries); err != nil {
		return nil, nil, err
	}
	if strictLinks {
		for i, declaration := range declarations {
			if err := ValidateLink(declaration); err != nil {
				return nil, nil, fmt.Errorf("element %d: %w", i, err)
			}
		}
	}
	return rawDeclarations, declarations, nil
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

func validatePrecomputedRelationValue(rel string, raw stdjson.RawMessage, strictLinks bool) error {
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
			if err := validatePrecomputedLinkObject(rel, item, strictLinks); err != nil {
				return fmt.Errorf("hal: precomputed relation %q element %d: %w", rel, i, err)
			}
		}
		return nil
	}

	if rel == relCuries {
		return errors.New("hal: precomputed curies relation must be an array of link objects")
	}
	return validatePrecomputedLinkObject(rel, trimmed, strictLinks)
}

//nolint:cyclop // Schema checks are intentionally explicit per field.
func validatePrecomputedLinkObject(rel string, raw stdjson.RawMessage, strictLinks bool) error {
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
	if strictLinks {
		var link Link
		if err := stdjson.Unmarshal(raw, &link); err != nil {
			return fmt.Errorf("hal: precomputed relation %q link: %w", rel, err)
		}
		if err := ValidateLink(link); err != nil {
			return fmt.Errorf("hal: precomputed relation %q: %w", rel, err)
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

func (i *Instance) resolveCuriesExcept(links map[string]any, inherited *curieScope) []Link {
	prefixes := anyRelationPrefixes(links)
	if len(prefixes) > 0 && inherited != nil {
		filtered := prefixes[:0]
		for _, prefix := range prefixes {
			if !inherited.has(prefix) {
				filtered = append(filtered, prefix)
			}
		}
		prefixes = filtered
	}
	return i.curieDeclarations(prefixes)
}

// curieDeclarations returns generated declarations for registered prefixes.
// The returned order follows the sorted input prefixes.
func (i *Instance) curieDeclarations(prefixes []string) []Link {
	if i == nil || len(prefixes) == 0 {
		return nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	declarations := make([]Link, 0, len(prefixes))
	for _, name := range prefixes {
		href, ok := i.curies[name]
		if !ok {
			continue
		}
		declarations = append(declarations, Link{
			Rel:       relCuries,
			Name:      name,
			Href:      href,
			Templated: curieHrefTemplated(href),
		})
	}
	return declarations
}

// resolvePrecomputedLinks merges the registered declarations for the handle's
// relation prefixes at wrap time. Handles stay instance-independent so a late
// registration or a different instance is reflected on the next wrap.
func (i *Instance) resolvePrecomputedLinks(links *PrecomputedLinks) ([]byte, error) {
	if len(links.JSON) == 0 {
		return nil, nil
	}
	declarations := i.curieDeclarations(links.prefixes)
	if len(declarations) == 0 {
		return links.JSON, nil
	}
	return mergePrecomputedCuries(links.JSON, declarations)
}

func mergePrecomputedCuries(linksJSON []byte, registered []Link) ([]byte, error) {
	var document map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(linksJSON, &document); err != nil {
		return nil, fmt.Errorf("hal: precomputed links must be a JSON object: %w", err)
	}
	var links map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(document[fieldLinks], &links); err != nil || links == nil {
		return nil, errors.New("hal: precomputed _links must be a JSON object")
	}

	var userRaw []stdjson.RawMessage
	var user []Link
	if raw, ok := links[relCuries]; ok {
		var err error
		userRaw, user, err = decodePrecomputedCurieRaw(raw, false, false)
		if err != nil {
			return nil, fmt.Errorf("hal: precomputed curies: %w", err)
		}
	}

	merged, err := appendMissingCuries(userRaw, user, registered)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	links[relCuries] = encoded

	encodedLinks, err := json.Marshal(links)
	if err != nil {
		return nil, err
	}
	document[fieldLinks] = encodedLinks
	return json.Marshal(document)
}

func appendMissingCuries(userRaw []stdjson.RawMessage, user, registered []Link) ([]stdjson.RawMessage, error) {
	seen := make(map[string]struct{}, len(user)+len(registered))
	for _, declaration := range user {
		seen[declaration.Name] = struct{}{}
	}

	merged := append([]stdjson.RawMessage(nil), userRaw...)
	for _, declaration := range registered {
		if _, exists := seen[declaration.Name]; exists {
			continue
		}
		encoded, err := json.Marshal(declaration)
		if err != nil {
			return nil, err
		}
		merged = append(merged, encoded)
		seen[declaration.Name] = struct{}{}
	}
	return merged, nil
}

// validatePrecomputedStrictRelations re-checks relation keys on a reusable
// handle when the wrapping instance enables WithStrictRelations. Structural
// validation already ran at compile time; this opt-in check keeps the
// PrecomputedLinks fast path free of relation policy otherwise.
func (i *Instance) validatePrecomputedStrictRelations(linksJSON []byte) error {
	fields, err := decodePrecomputedObject(linksJSON)
	if err != nil {
		return err
	}
	links := fields
	if raw, ok := fields[fieldLinks]; ok {
		var wrapped map[string]stdjson.RawMessage
		if err := stdjson.Unmarshal(raw, &wrapped); err != nil || wrapped == nil {
			return errors.New("hal: precomputed _links must be a JSON object")
		}
		links = wrapped
	}
	for rel := range links {
		if err := i.validateStrictRelation(rel); err != nil {
			return fmt.Errorf("hal: precomputed %w", err)
		}
	}
	return nil
}

// precomputedCurieDeclarations reads the declarations already present in a
// wrapped precomputed link object so embedded children can inherit them
// without rewriting the cached bytes.
func precomputedCurieDeclarations(linksJSON []byte) ([]Link, error) {
	var document map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(linksJSON, &document); err != nil {
		return nil, fmt.Errorf("hal: precomputed links must be a JSON object: %w", err)
	}
	rawLinks, ok := document[fieldLinks]
	if !ok {
		return nil, nil
	}
	var links map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(rawLinks, &links); err != nil || links == nil {
		return nil, errors.New("hal: precomputed _links must be a JSON object")
	}
	rawCuries, ok := links[relCuries]
	if !ok {
		return nil, nil
	}
	var declarations []Link
	if err := stdjson.Unmarshal(rawCuries, &declarations); err != nil {
		return nil, fmt.Errorf("hal: precomputed curies: %w", err)
	}
	return declarations, nil
}
