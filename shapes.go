// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import "errors"

// LinkShape controls whether a relation is serialized as one object or always
// as an array. ShapeAuto keeps the HAL default: one value is an object and
// repeated values form an array.
type LinkShape uint8

const (
	// ShapeAuto emits an object for one link and an array for repeated links.
	ShapeAuto LinkShape = iota
	// ShapeArray always emits an array, even for one link.
	ShapeArray
	// ShapeObject requires at most one link and emits an object.
	ShapeObject
)

func (s LinkShape) String() string {
	switch s {
	case ShapeArray:
		return "array"
	case ShapeObject:
		return "object"
	default:
		return "auto"
	}
}

// ErrLinkShape is returned when a configured shape cannot represent the links
// present, such as ShapeObject with repeated links.
var ErrLinkShape = errors.New("hal: incompatible link shape")

func applyLinkShape(rel string, value any, shape LinkShape) (any, bool, error) {
	if rel == relCuries {
		return value, false, nil // curies is always an array
	}
	switch shape {
	case ShapeArray:
		if _, isArray := value.([]any); isArray {
			return value, false, nil
		}
		return []any{value}, true, nil
	case ShapeObject:
		items, isArray := value.([]any)
		if !isArray {
			return value, false, nil
		}
		switch len(items) {
		case 0:
			return value, false, nil
		case 1:
			return items[0], true, nil
		default:
			return nil, false, ErrLinkShape
		}
	default:
		return value, false, nil
	}
}

func applyLinkShapes(links map[string]any, shapeFor func(string) LinkShape) (map[string]any, error) {
	var shaped map[string]any
	for rel, value := range links {
		next, changed, err := applyLinkShape(rel, value, shapeFor(rel))
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}
		if shaped == nil {
			shaped = make(map[string]any, len(links))
			for name, original := range links {
				shaped[name] = original
			}
		}
		shaped[rel] = next
	}
	if shaped != nil {
		return shaped, nil
	}
	return links, nil
}

func (i *Instance) shapeFor(rel string) LinkShape {
	if i == nil {
		return ShapeAuto
	}
	if shape, ok := i.relationShapes[rel]; ok {
		return shape
	}
	return i.linkShape
}

func applyStaticLinkShapes(links map[string]any, inst *Instance) error {
	for rel, value := range links {
		next, changed, err := applyLinkShape(rel, value, inst.shapeFor(rel))
		if err != nil {
			return err
		}
		if changed {
			links[rel] = next
		}
	}
	return nil
}
