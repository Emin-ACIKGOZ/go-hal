// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"errors"
	"fmt"

	"github.com/Emin-ACIKGOZ/go-hal/internal/relation"
)

// ErrInvalidRelation is returned for a blank, control-bearing, non-UTF-8, or
// otherwise unusable link relation.
var ErrInvalidRelation = errors.New("hal: invalid link relation")

// appendRelationValue adds a relation value with the HAL multiplicity rules:
// one value is an object, repeated values form an array. The same helper is
// used for _links and _embedded values so their shapes cannot drift.
func appendRelationValue(values map[string]any, rel string, val any) {
	if existing, ok := values[rel]; ok {
		if slice, isSlice := existing.([]any); isSlice {
			values[rel] = append(slice, val)
		} else {
			values[rel] = []any{existing, val}
		}
	} else {
		values[rel] = val
	}
}

// validateRelationName enforces the default relation hardening and rejects
// bare-token case collisions within one relation namespace.
func validateRelationName(rel string, existing map[string]any) error {
	if relation.Classify(rel) == relation.Invalid {
		return fmt.Errorf("%w: %q", ErrInvalidRelation, rel)
	}
	if relation.Classify(rel) != relation.Token {
		return nil
	}
	folded := relation.FoldToken(rel)
	for name := range existing {
		if name == rel || relation.Classify(name) != relation.Token {
			continue
		}
		if relation.FoldToken(name) == folded {
			return fmt.Errorf("%w: bare token %q collides with %q", ErrInvalidRelation, rel, name)
		}
	}
	return nil
}
