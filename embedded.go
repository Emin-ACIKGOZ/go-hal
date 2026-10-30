// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"reflect"
	"unicode/utf8"

	json "github.com/goccy/go-json"
)

// ErrInvalidEmbedded is returned when an embedded value cannot be represented
// as a HAL Resource Object.
var ErrInvalidEmbedded = errors.New("hal: invalid embedded resource")

// ErrEmbeddedDepthExceeded is returned when embedded envelopes form a cycle or
// exceed the maximum nesting depth during marshaling.
var ErrEmbeddedDepthExceeded = errors.New("hal: embedded resource depth exceeded")

const maxEmbeddedDepth = 32

// AddEmbedded appends an embedded resource under rel, following HAL §4.1.2.
//
// One value is emitted as an object; repeated values form an array. A slice is
// flattened one level, a one-element slice is emitted as an object, and an
// empty slice is a no-op. Nested envelopes must be *Envelope values so the
// marshaller can bound depth and detect cycles without copying identity.
//
// Raw structs, maps, and object json.RawMessage values are embedded as-is; they
// are not automatically wrapped or link-generated. Nil, typed-nil, scalar,
// []byte, Envelope values, and non-object RawMessage values are rejected with a
// deferred ErrInvalidEmbedded.
//
// AddEmbedded is a pointer method and is not safe to call concurrently with
// marshaling. It works on generated, raw, static, and precomputed envelopes.
func (e *Envelope) AddEmbedded(rel string, value any) {
	if e.err != nil {
		return
	}
	if err := e.validateRelationForAdd(rel, e.embedded); err != nil {
		e.err = err
		return
	}
	normalized, ok, err := normalizeEmbeddedValue(value)
	if err != nil {
		e.err = err
		return
	}
	if !ok {
		return
	}
	if e.embedded == nil {
		e.embedded = make(map[string]any, defaultLinksCapacity)
	}
	appendRelationValue(e.embedded, rel, normalized)
}

//nolint:cyclop // Type normalization is a deliberate flat switch.
func normalizeEmbeddedValue(value any) (any, bool, error) {
	if value == nil {
		return nil, false, fmt.Errorf("%w: nil value", ErrInvalidEmbedded)
	}

	switch v := value.(type) {
	case Envelope:
		return nil, false, fmt.Errorf("%w: embed *Envelope, not Envelope value", ErrInvalidEmbedded)
	case *Envelope:
		if v == nil {
			return nil, false, fmt.Errorf("%w: nil *Envelope", ErrInvalidEmbedded)
		}
		return v, true, nil
	case stdjson.RawMessage:
		if err := validateRawEmbeddedObject(v); err != nil {
			return nil, false, err
		}
		return v, true, nil
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return nil, false, fmt.Errorf("%w: []byte is not a resource object", ErrInvalidEmbedded)
		}
		if rv.Len() == 0 {
			return nil, false, nil
		}
		items := make([]any, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			item, err := normalizeEmbeddedSingle(rv.Index(i).Interface())
			if err != nil {
				return nil, false, err
			}
			items = append(items, item)
		}
		if len(items) == 1 {
			return items[0], true, nil
		}
		return items, true, nil
	}
	single, err := normalizeEmbeddedSingle(value)
	if err != nil {
		return nil, false, err
	}
	return single, true, nil
}

//nolint:cyclop // Type normalization is a deliberate flat switch.
func normalizeEmbeddedSingle(value any) (any, error) {
	if value == nil {
		return nil, fmt.Errorf("%w: nil value", ErrInvalidEmbedded)
	}
	switch v := value.(type) {
	case Envelope:
		return nil, fmt.Errorf("%w: embed *Envelope, not Envelope value", ErrInvalidEmbedded)
	case *Envelope:
		if v == nil {
			return nil, fmt.Errorf("%w: nil *Envelope", ErrInvalidEmbedded)
		}
		return v, nil
	case stdjson.RawMessage:
		if err := validateRawEmbeddedObject(v); err != nil {
			return nil, err
		}
		return v, nil
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		return nil, fmt.Errorf("%w: nested slices are not flattened", ErrInvalidEmbedded)
	}
	if (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) && rv.IsNil() {
		return nil, fmt.Errorf("%w: nil value", ErrInvalidEmbedded)
	}
	if !isEmbeddedObjectKind(rv.Kind()) {
		return nil, fmt.Errorf("%w: %T is not a resource object", ErrInvalidEmbedded, value)
	}
	return value, nil
}

func isEmbeddedObjectKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Map, reflect.Struct, reflect.Pointer, reflect.Interface:
		return true
	default:
		return false
	}
}

func validateRawEmbeddedObject(raw stdjson.RawMessage) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("%w: RawMessage is not valid UTF-8", ErrInvalidEmbedded)
	}
	trimmed := bytes.TrimSpace(raw)
	if !hasJSONObjectDelimiters(trimmed) {
		return fmt.Errorf("%w: RawMessage must be a JSON object", ErrInvalidEmbedded)
	}
	if !stdjson.Valid(trimmed) {
		return fmt.Errorf("%w: RawMessage is not valid JSON", ErrInvalidEmbedded)
	}
	return nil
}

// marshalEmbeddedResource marshals one embedded value. Direct *Envelope values
// are marshaled through the scope-aware envelope path; raw values are encoded
// and required to be non-null JSON objects.
func marshalEmbeddedResource(value any, depth int, ancestors map[*Envelope]struct{}, scope *curieScope) ([]byte, error) {
	switch v := value.(type) {
	case *Envelope:
		return marshalEnvelopeChild(v, depth, ancestors, scope)
	case stdjson.RawMessage:
		return marshalRawEmbeddedValue(v)
	case []any:
		return marshalEmbeddedArray(v, depth, ancestors, scope)
	default:
		return marshalRawEmbeddedValue(value)
	}
}

func marshalEmbeddedArray(values []any, depth int, ancestors map[*Envelope]struct{}, scope *curieScope) ([]byte, error) {
	items := make([]stdjson.RawMessage, 0, len(values))
	for i, value := range values {
		raw, err := marshalEmbeddedResource(value, depth, ancestors, scope)
		if err != nil {
			return nil, fmt.Errorf("embedded element %d: %w", i, err)
		}
		items = append(items, raw)
	}
	return stdjson.Marshal(items)
}

func marshalRawEmbeddedValue(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEmbedded, err)
	}
	trimmed := bytes.TrimSpace(raw)
	if !hasJSONObjectDelimiters(trimmed) {
		return nil, fmt.Errorf("%w: value marshals to a non-object", ErrInvalidEmbedded)
	}
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("%w: value marshals to invalid UTF-8", ErrInvalidEmbedded)
	}
	return raw, nil
}

func marshalEnvelopeChild(child *Envelope, depth int, ancestors map[*Envelope]struct{}, scope *curieScope) ([]byte, error) {
	if child == nil {
		return nil, fmt.Errorf("%w: nil *Envelope", ErrInvalidEmbedded)
	}
	if _, seen := ancestors[child]; seen {
		return nil, fmt.Errorf("%w: cycle detected", ErrEmbeddedDepthExceeded)
	}
	ancestors[child] = struct{}{}
	defer delete(ancestors, child)
	return child.marshalEnvelope(depth, ancestors, scope)
}

// AddCachedLink adds a link and embeds the same relation's resource, the HAL
// hypertext cache pattern from draft §8.4. The operation is atomic: if the
// relation or resource is invalid, neither the link nor the embedded value is
// added. On static or precomputed envelopes it records the deferred
// ErrAddLinkPrecomputed and adds nothing.
func (e *Envelope) AddCachedLink(link Link, resource any) {
	if e.premanaged {
		if e.err == nil {
			e.err = ErrAddLinkPrecomputed
		}
		return
	}
	if e.err != nil {
		return
	}
	if err := e.validateLinkForAdd(link); err != nil {
		e.err = err
		return
	}
	if err := validateRelationName(link.Rel, e.embedded); err != nil {
		e.err = err
		return
	}
	normalized, ok, err := normalizeEmbeddedValue(resource)
	if err != nil {
		e.err = err
		return
	}

	e.addLinkRaw(link.Rel, link)
	if !ok {
		return
	}
	if e.embedded == nil {
		e.embedded = make(map[string]any, defaultLinksCapacity)
	}
	appendRelationValue(e.embedded, link.Rel, normalized)
}
