// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// ErrDuplicateCurie is returned when CURIE declarations in one source reuse a
// name. Duplicate declarations are ambiguous and not interoperable.
var ErrDuplicateCurie = errors.New("hal: duplicate CURIE declaration")

// ErrInvalidCurie is returned when a CURIE declaration has an empty or
// whitespace-bearing name, or an empty href.
var ErrInvalidCurie = errors.New("hal: invalid CURIE declaration")

func validateCurieDeclarations(declarations []Link) error {
	seen := make(map[string]struct{}, len(declarations))
	for _, declaration := range declarations {
		name := declaration.Name
		if name == "" || strings.IndexFunc(name, unicode.IsSpace) >= 0 {
			return fmt.Errorf("%w: name %q must be non-empty and contain no whitespace", ErrInvalidCurie, name)
		}
		if strings.TrimSpace(declaration.Href) == "" {
			return fmt.Errorf("%w: %q must have a non-empty href", ErrInvalidCurie, name)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateCurie, name)
		}
		seen[name] = struct{}{}
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
func mergeCuries(existing any, generated []Link) ([]Link, error) {
	user, err := curieLinks(existing)
	if err != nil {
		return nil, err
	}
	if err := validateCurieDeclarations(user); err != nil {
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
