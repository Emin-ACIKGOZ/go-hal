// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"fmt"
	"reflect"
)

// CollectionPage represents a project-specific HAL collection convention.
// It includes navigation links, embedded items, and pagination metadata.
//
// Count and Total are always serialized so a total of zero stays observable;
// they are not part of the HAL specification. Empty _links and _embedded maps
// are omitted.
type CollectionPage struct {
	Links    map[string]any `json:"_links,omitempty"`
	Embedded map[string]any `json:"_embedded,omitempty"`
	Count    int            `json:"count"`
	Total    int            `json:"total"`
}

// Collection creates a CollectionPage using the DefaultInstance.
func Collection[T any](ctx context.Context, items []*T, total int, selfLink Link) *CollectionPage {
	return DefaultInstance.Collection(ctx, items, total, selfLink)
}

// Collection wraps a slice of items into a HAL CollectionPage.
// It iterates over the items, wraps each one using the registered generators,
// and constructs the embedded items list. Generators are registered for *T, so
// use []*T to get per-item links; []T does not match a *T generator.
//
// This method panics if items is not a slice.
func (i *Instance) Collection(ctx context.Context, items any, total int, selfLink Link) *CollectionPage {
	selfLink = normalizeLink(selfLink)
	val := reflect.ValueOf(items)
	if val.Kind() != reflect.Slice {
		panic("hal: Collection items must be a slice")
	}
	if i.strictLinks {
		if err := ValidateLink(selfLink); err != nil {
			panic(fmt.Sprintf("hal: Collection: %v", err))
		}
	}

	count := val.Len()
	embeddedItems := make([]*Envelope, count)

	for idx := 0; idx < count; idx++ {
		item := val.Index(idx).Interface()
		embeddedItems[idx] = i.Wrap(ctx, item)
	}

	links := make(map[string]any)
	shaped, _, err := applyLinkShape("self", selfLink, i.shapeFor("self"))
	if err != nil {
		panic(fmt.Sprintf("hal: Collection: %v", err))
	}
	links["self"] = shaped

	return &CollectionPage{
		Links: links,
		Embedded: map[string]any{
			"items": embeddedItems,
		},
		Count: count,
		Total: total,
	}
}
