// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"errors"
	"fmt"
	"sort"
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

func analyzeRelation(rel string) (relation.Kind, relationMeta, error) {
	kind, err := analyzeRelationKind(rel)
	if err != nil {
		return kind, relationMeta{}, err
	}
	folded, isBare := collisionToken(rel)
	return kind, relationMeta{folded: folded, isBare: isBare}, nil
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
	folded string
	isBare bool
}

// relationIndex maps an ASCII-folded bare token to the relation spelling that
// first used it. Only bare tokens participate; CURIEs, URIs, and relative
// references remain case-sensitive in v1.
type relationIndex map[string]string

func relationIndexFromMap(existing map[string]any) relationIndex {
	size := len(existing) + 1
	if size < defaultLinksCapacity {
		size = defaultLinksCapacity
	}
	index := make(relationIndex, size)
	for name := range existing {
		if folded, ok := collisionToken(name); ok {
			index[folded] = name
		}
	}
	return index
}

// check reports a case-insensitive bare-token collision. The first check on an
// envelope without an index rebuilds it from the authoritative relation map.
func (idx *relationIndex) check(rel string, meta relationMeta, existing map[string]any) error {
	if !meta.isBare {
		return nil
	}
	if *idx == nil {
		*idx = relationIndexFromMap(existing)
	}
	if previous, exists := (*idx)[meta.folded]; exists && previous != rel {
		return fmt.Errorf("%w: bare token %q collides with %q", ErrInvalidRelation, rel, previous)
	}
	return nil
}

func (idx *relationIndex) add(rel string, meta relationMeta) {
	if !meta.isBare {
		return
	}
	if *idx == nil {
		*idx = make(relationIndex, defaultLinksCapacity)
	}
	(*idx)[meta.folded] = rel
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
func (e *Envelope) validateRelationForLinkAdd(rel string, existing map[string]any, index *relationIndex) (relationMeta, error) {
	kind, meta, err := analyzeRelation(rel)
	if err != nil {
		return relationMeta{}, err
	}
	if err := index.check(rel, meta, existing); err != nil {
		return relationMeta{}, err
	}
	if e.instance != nil && e.instance.strictRelations && !e.instance.deferStrictRelation(rel) {
		if err := e.instance.validateStrictRelationKind(rel, kind); err != nil {
			return relationMeta{}, err
		}
	}
	return meta, nil
}

func (i *Instance) validateStrictRelationKind(rel string, kind relation.Kind) error {
	return i.validateStrictRelationKindScoped(rel, kind, nil)
}

// validateStrictRelationKindScoped accepts a relation declared on the instance
// or visible through the effective marshal scope. Halting on the existing inner
// messages keeps error compatibility with non-scoped callers.
func (i *Instance) validateStrictRelationKindScoped(rel string, kind relation.Kind, scope *curieScope) error {
	if prefix, _, ok := relation.SplitCurie(rel); ok && (i.hasCuriePrefix(prefix) || scope.has(prefix)) {
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

// deferStrictRelation reports whether strict validation must wait for the
// resource-local and inherited CURIE scope built at marshal time. Only a
// non-scheme CURIE prefix that the instance does not declare can be satisfied
// later by a local declaration; all other strict failures stay add-time.
func (i *Instance) deferStrictRelation(rel string) bool {
	prefix, _, ok := relation.SplitCurie(rel)
	return ok && !relation.HasScheme(rel) && !i.hasCuriePrefix(prefix)
}

func (i *Instance) hasCuriePrefix(prefix string) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	_, ok := i.curies[prefix]
	return ok
}

func validateRelationCollisions(names []string) error {
	index := make(relationIndex, len(names))
	for _, name := range names {
		_, meta, err := analyzeRelation(name)
		if err != nil {
			return err
		}
		if err := index.check(name, meta, nil); err != nil {
			return err
		}
		index.add(name, meta)
	}
	return nil
}

// validateStrictRelationMaps rechecks only the relations whose strict
// validation was deferred at add time, against the effective marshal scope.
// Sorted iteration keeps the first reported error deterministic.
func (i *Instance) validateStrictRelationMaps(links, embedded map[string]any, scope *curieScope) error {
	if i == nil || !i.strictRelations {
		return nil
	}
	rels := make([]string, 0, len(links)+len(embedded))
	for rel := range links {
		if i.deferStrictRelation(rel) {
			rels = append(rels, rel)
		}
	}
	for rel := range embedded {
		if i.deferStrictRelation(rel) {
			rels = append(rels, rel)
		}
	}
	sort.Strings(rels)
	for _, rel := range rels {
		if err := i.validateStrictRelationKindScoped(rel, relation.Classify(rel), scope); err != nil {
			return err
		}
	}
	return nil
}
