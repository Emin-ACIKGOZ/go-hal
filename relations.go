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
// bare-token case collisions within one relation namespace. It is used by
// one-shot validation paths such as RegisterStatic, precomputed input, and the
// atomic AddCachedLink helper.
func validateRelationName(rel string, existing map[string]any) error {
	_, folded, isBare, err := analyzeRelation(rel)
	if err != nil {
		return err
	}
	if !isBare {
		return nil
	}
	for name := range existing {
		if name == rel {
			continue
		}
		if previous, ok := collisionToken(name); ok && previous == folded {
			return fmt.Errorf("%w: bare token %q collides with %q", ErrInvalidRelation, rel, name)
		}
	}
	return nil
}

func analyzeRelation(rel string) (relation.Kind, string, bool, error) {
	kind, err := analyzeRelationKind(rel)
	if err != nil {
		return kind, "", false, err
	}
	folded, isBare := collisionToken(rel)
	return kind, folded, isBare, nil
}

func analyzeRelationKind(rel string) (relation.Kind, error) {
	if rel == "" {
		return relation.Invalid, fmt.Errorf("%w: link relation must not be empty", ErrInvalidRelation)
	}
	kind := relation.Classify(rel)
	if kind == relation.Invalid {
		if strings.TrimSpace(rel) == "" {
			return kind, fmt.Errorf("%w: link relation must not be empty", ErrInvalidRelation)
		}
		return kind, fmt.Errorf("%w: %q is not a valid relation", ErrInvalidRelation, rel)
	}
	return kind, nil
}

// collisionToken returns the folded key only for names with actual bare-token
// syntax. Relative references such as "a/b", "?query", and "#frag" are lexical
// tokens for relation classification but are not registered bare tokens and
// must not case-fold into collisions.
func collisionToken(rel string) (string, bool) {
	return relation.FoldBareToken(rel)
}

type relationMeta struct {
	kind   relation.Kind
	folded string
	isBare bool
}

func buildRelationIndex(existing map[string]any) map[string]string {
	size := len(existing) + 1
	if size < defaultLinksCapacity {
		size = defaultLinksCapacity
	}
	index := make(map[string]string, size)
	for name := range existing {
		if folded, ok := collisionToken(name); ok {
			index[folded] = name
		}
	}
	return index
}

func checkRelationCollision(rel, folded string, isBare bool, existing map[string]any, index *map[string]string) error {
	if !isBare {
		return nil
	}
	if *index == nil {
		*index = buildRelationIndex(existing)
	}
	if previous, exists := (*index)[folded]; exists && previous != rel {
		return fmt.Errorf("%w: bare token %q collides with %q", ErrInvalidRelation, rel, previous)
	}
	return nil
}

func recordRelationToken(index *map[string]string, meta relationMeta, rel string) {
	if !meta.isBare {
		return
	}
	if *index == nil {
		*index = make(map[string]string, defaultLinksCapacity)
	}
	(*index)[meta.folded] = rel
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

// validateRelationForLinkAdd performs syntax, strict-relation, and indexed
// collision checks for AddLink/AddEmbedded. The returned metadata lets the
// caller record the folded token without classifying or folding twice.
func (e *Envelope) validateRelationForLinkAdd(rel string, existing map[string]any, index *map[string]string) (relationMeta, error) {
	kind, folded, isBare, err := analyzeRelation(rel)
	if err != nil {
		return relationMeta{}, err
	}
	if err := checkRelationCollision(rel, folded, isBare, existing, index); err != nil {
		return relationMeta{}, err
	}
	if e.instance != nil && e.instance.strictRelations {
		if err := e.instance.validateStrictRelationKind(rel, kind); err != nil {
			return relationMeta{}, err
		}
	}
	return relationMeta{kind: kind, folded: folded, isBare: isBare}, nil
}

func (i *Instance) validateStrictRelation(rel string) error {
	if err := validateRelationSyntax(rel); err != nil {
		return err
	}
	return i.validateStrictRelationKind(rel, relation.Classify(rel))
}

func (i *Instance) validateStrictRelationKind(rel string, kind relation.Kind) error {
	if prefix, _, ok := relation.SplitCurie(rel); ok && i.hasCuriePrefix(prefix) {
		return nil // declared CURIEs may use non-scheme prefixes such as "_private"
	}
	if relation.HasScheme(rel) {
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
		folded, ok := collisionToken(name)
		if !ok {
			continue
		}
		if previous, exists := seen[folded]; exists && previous != name {
			return fmt.Errorf("%w: bare token %q collides with %q", ErrInvalidRelation, name, previous)
		}
		seen[folded] = name
	}
	return nil
}
