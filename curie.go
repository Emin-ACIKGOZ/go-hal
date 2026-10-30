// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	stdjson "encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Emin-ACIKGOZ/go-hal/internal/relation"
	"github.com/Emin-ACIKGOZ/go-hal/internal/uritemplate"
)

// ErrDuplicateCurie is returned when CURIE declarations in one source reuse a
// name. Duplicate declarations are ambiguous and not interoperable.
var ErrDuplicateCurie = errors.New("hal: duplicate CURIE declaration")

// ErrInvalidCurie is returned when a CURIE declaration has an unusable name
// (empty, whitespace/control-bearing, non-UTF-8, colon-bearing, or failing the
// strict ASCII NCName subset), an empty href, or a strict href/templated
// violation.
var ErrInvalidCurie = errors.New("hal: invalid CURIE declaration")

func sortedPrefixSet(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	prefixes := make([]string, 0, len(set))
	for prefix := range set {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	return prefixes
}

func linkRelationPrefixes(links []Link) []string {
	set := make(map[string]struct{})
	for _, link := range links {
		if prefix, _, ok := relation.SplitCurie(link.Rel); ok {
			set[prefix] = struct{}{}
		}
	}
	return sortedPrefixSet(set)
}

func rawRelationPrefixes(links map[string]stdjson.RawMessage) []string {
	set := make(map[string]struct{}, len(links))
	for rel := range links {
		if prefix, _, ok := relation.SplitCurie(rel); ok {
			set[prefix] = struct{}{}
		}
	}
	return sortedPrefixSet(set)
}

func anyRelationPrefixes(links map[string]any) []string {
	var prefixes []string
	for rel := range links {
		if prefix, _, ok := relation.SplitCurie(rel); ok {
			prefixes = append(prefixes, prefix)
		}
	}
	if len(prefixes) == 0 {
		return nil
	}
	sort.Strings(prefixes)
	out := prefixes[:1]
	for _, prefix := range prefixes[1:] {
		if prefix != out[len(out)-1] {
			out = append(out, prefix)
		}
	}
	return out
}

// mergePrefixes merges two sorted, duplicate-free prefix lists. When b is
// empty it returns a unchanged, so callers with no embedded prefixes keep the
// existing allocation-free path.
func mergePrefixes(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	merged := make([]string, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			merged = append(merged, a[i])
			i++
		case a[i] > b[j]:
			merged = append(merged, b[j])
			j++
		default:
			merged = append(merged, a[i])
			i++
			j++
		}
	}
	merged = append(merged, a[i:]...)
	merged = append(merged, b[j:]...)
	return merged
}

func validateCurieName(name string, strict bool) error {
	if name == "" {
		return fmt.Errorf("%w: name must be non-empty", ErrInvalidCurie)
	}
	if !utf8.ValidString(name) {
		return fmt.Errorf("%w: name %q must be valid UTF-8", ErrInvalidCurie, name)
	}
	if strict {
		if !isStrictCurieName(name) {
			return fmt.Errorf("%w: name %q is not an ASCII NCName", ErrInvalidCurie, name)
		}
		return nil
	}
	if strings.ContainsRune(name, ':') {
		return fmt.Errorf("%w: name %q must not contain a colon", ErrInvalidCurie, name)
	}
	if strings.IndexFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) >= 0 {
		return fmt.Errorf("%w: name %q must contain no whitespace or control characters", ErrInvalidCurie, name)
	}
	return nil
}

//nolint:cyclop // Flat NCName character-class scanner.
func isStrictCurieName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if i == 0 {
			if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' {
				continue
			}
			return false
		}
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '.', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

func validateCurieDeclarations(declarations []Link, strict bool) error {
	seen := make(map[string]struct{}, len(declarations))
	for _, declaration := range declarations {
		name := declaration.Name
		if err := validateCurieName(name, strict); err != nil {
			return err
		}
		if strings.TrimSpace(declaration.Href) == "" {
			return fmt.Errorf("%w: %q must have a non-empty href", ErrInvalidCurie, name)
		}
		if strict {
			if err := validateStrictCurieHref(declaration); err != nil {
				return err
			}
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateCurie, name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateStrictCurieHref(declaration Link) error {
	tmpl, err := uritemplate.Parse(declaration.Href)
	if err != nil {
		return fmt.Errorf("%w: %q href: %v", ErrInvalidCurie, declaration.Name, err)
	}
	if !tmpl.SingleRelExpression(uritemplate.OpSimple) && !tmpl.SingleRelExpression(uritemplate.OpReserved) {
		return fmt.Errorf("%w: %q href must be exactly one {rel} or {+rel} expression", ErrInvalidCurie, declaration.Name)
	}
	if !declaration.Templated {
		return fmt.Errorf("%w: %q must set templated true", ErrInvalidCurie, declaration.Name)
	}
	return nil
}

// curieLinks normalizes the user-supplied curies relation to a slice. A single
// Link is the shape produced by one AddLink call; repeated calls produce []any.
func curieLinks(value any) ([]Link, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case Link:
		return []Link{v}, nil
	case []Link:
		return append([]Link(nil), v...), nil
	case []any:
		links := make([]Link, 0, len(v))
		for i, item := range v {
			link, ok := item.(Link)
			if !ok {
				return nil, fmt.Errorf("%w: curies element %d has type %T", ErrInvalidCurie, i, item)
			}
			links = append(links, link)
		}
		return links, nil
	default:
		return nil, fmt.Errorf("%w: curies has type %T", ErrInvalidCurie, value)
	}
}

// mergeCuries combines user declarations with generated declarations. User
// declarations win on name collisions; duplicates within either input were
// already rejected.
func mergeCuries(existing any, generated []Link, strict bool) ([]Link, error) {
	user, err := curieLinks(existing)
	if err != nil {
		return nil, err
	}
	if err := validateCurieDeclarations(user, strict); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(user)+len(generated))
	for _, declaration := range user {
		seen[declaration.Name] = struct{}{}
	}
	merged := append([]Link(nil), user...)
	for _, declaration := range generated {
		if _, exists := seen[declaration.Name]; exists {
			continue
		}
		merged = append(merged, declaration)
		seen[declaration.Name] = struct{}{}
	}
	return merged, nil
}

// curieScope is the marshal-scoped set of CURIE declarations visible to an
// embedded resource: parent declarations plus local overrides.
type curieScope struct {
	declared map[string]Link
}

func extendCurieScope(parent *curieScope, declarations []Link) *curieScope {
	if len(declarations) == 0 {
		return parent
	}
	scope := &curieScope{declared: make(map[string]Link, len(declarations))}
	if parent != nil {
		for name, declaration := range parent.declared {
			scope.declared[name] = declaration
		}
	}
	for _, declaration := range declarations {
		scope.declared[declaration.Name] = declaration
	}
	return scope
}

func (s *curieScope) has(name string) bool {
	if s == nil {
		return false
	}
	_, ok := s.declared[name]
	return ok
}
