// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"errors"
	"fmt"
	"strings"

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
	if err := validateRelationSyntax(rel); err != nil {
		return err
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

func validateRelationSyntax(rel string) error {
	if rel == "" || strings.TrimSpace(rel) == "" {
		return fmt.Errorf("%w: link relation must not be empty", ErrInvalidRelation)
	}
	if relation.Classify(rel) == relation.Invalid {
		return fmt.Errorf("%w: %q is not a valid relation", ErrInvalidRelation, rel)
	}
	return nil
}

func (e *Envelope) validateRelationForAdd(rel string, existing map[string]any) error {
	if err := validateRelationName(rel, existing); err != nil {
		return err
	}
	if e.instance != nil && e.instance.strictRelations {
		return e.instance.validateStrictRelation(rel)
	}
	return nil
}

func (i *Instance) validateStrictRelation(rel string) error {
	if err := validateRelationSyntax(rel); err != nil {
		return err
	}
	kind := relation.Classify(rel)
	if relation.HasScheme(rel) {
		prefix, _, ok := relation.SplitCurie(rel)
		if ok && i.hasCuriePrefix(prefix) {
			return nil
		}
		return nil // undeclared scheme-like relations are opaque URIs
	}
	if kind == relation.Token {
		if !relation.IsToken(rel) {
			return fmt.Errorf("%w: %q is not a lowercase registered token", ErrInvalidRelation, rel)
		}
		return nil
	}
	return fmt.Errorf("%w: %q is not a token, URI, or declared CURIE", ErrInvalidRelation, rel)
}

func (i *Instance) hasCuriePrefix(prefix string) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	_, ok := i.curies[prefix]
	return ok
}

func validateRelationCollisions(names []string) error {
	seen := make(map[string]string, len(names))
	for _, name := range names {
		if relation.Classify(name) != relation.Token {
			continue
		}
		folded := relation.FoldToken(name)
		if previous, exists := seen[folded]; exists && previous != name {
			return fmt.Errorf("%w: bare token %q collides with %q", ErrInvalidRelation, name, previous)
		}
		seen[folded] = name
	}
	return nil
}
