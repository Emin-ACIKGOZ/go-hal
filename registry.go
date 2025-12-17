// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"reflect"
	"strings"
	"sync"
)

// Generator defines a function capable of producing HAL links for a specific value.
type Generator func(ctx context.Context, v any) []Link

// Instance maintains a registry of link generators and CURIE definitions.
// It is safe for concurrent use.
type Instance struct {
	mu         sync.RWMutex
	generators map[reflect.Type]Generator
	curies     map[string]string
	strictMode bool
}

// New creates a new HAL Instance with the provided options.
func New(opts ...InstanceOption) *Instance {
	i := &Instance{
		generators: make(map[reflect.Type]Generator),
		curies:     make(map[string]string),
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

// DefaultInstance is the global singleton registry used by package-level functions.
var DefaultInstance = New()

// Register binds a generator function to type T on the DefaultInstance.
func Register[T any](gen func(context.Context, *T) []Link) {
	RegisterInstance(DefaultInstance, gen)
}

// RegisterCurie registers a Compact URI prefix and href on the DefaultInstance.
func RegisterCurie(prefix, href string) {
	DefaultInstance.RegisterCurie(prefix, href)
}

// Wrap wraps the provided data in a HAL envelope using the DefaultInstance.
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

// Wrap creates a new Envelope containing the data and computed links.
// It resolves the appropriate generator for the data's type and invokes it.
func (i *Instance) Wrap(ctx context.Context, data any) *Envelope {
	e := &Envelope{
		Data:     data,
		instance: i,
		links:    make(map[string]any),
		embedded: make(map[string]any),
	}
	e.computeLinks(ctx)
	return e
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

	var used []Link
	for rel := range links {
		if idx := strings.IndexByte(rel, ':'); idx > 0 {
			prefix := rel[:idx]
			if href, ok := i.curies[prefix]; ok {
				used = append(used, Link{
					Rel:       "curies",
					Name:      prefix,
					Href:      href,
					Templated: true,
				})
			}
		}
	}
	return used
}
