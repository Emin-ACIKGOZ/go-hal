// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"reflect"
	"strings"
	"sync"
)

type Generator func(ctx context.Context, v any) []Link

type Instance struct {
	mu         sync.RWMutex
	generators map[reflect.Type]Generator
	curies     map[string]string
	strictMode bool
}

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

var DefaultInstance = New()

func Register[T any](gen func(context.Context, *T) []Link) {
	RegisterInstance(DefaultInstance, gen)
}

func RegisterCurie(prefix, href string) {
	DefaultInstance.RegisterCurie(prefix, href)
}

func Wrap(ctx context.Context, data any) *Envelope {
	return DefaultInstance.Wrap(ctx, data)
}

// --- Instance Methods ---

func (i *Instance) RegisterCurie(prefix, href string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.curies[prefix] = href
}

func (i *Instance) RegisterInstance(gen any) {
	// Deprecated: Use RegisterInstance[T]
	i.registerReflect(gen)
}

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
