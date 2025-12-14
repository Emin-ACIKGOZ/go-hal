// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"reflect"
)

type CollectionPage struct {
	Links    map[string]any `json:"_links"`
	Embedded map[string]any `json:"_embedded"`
	Count    int            `json:"count"`
	Total    int            `json:"total,omitempty"`
}

func Collection[T any](ctx context.Context, items []*T, total int, selfLink Link) *CollectionPage {
	return DefaultInstance.Collection(ctx, items, total, selfLink)
}

func (i *Instance) Collection(ctx context.Context, items any, total int, selfLink Link) *CollectionPage {
	val := reflect.ValueOf(items)
	if val.Kind() != reflect.Slice {
		panic("hal: Collection items must be a slice")
	}

	count := val.Len()
	embeddedItems := make([]*Envelope, count)

	for idx := 0; idx < count; idx++ {
		item := val.Index(idx).Interface()
		embeddedItems[idx] = i.Wrap(ctx, item)
	}

	links := make(map[string]any)
	links["self"] = selfLink

	return &CollectionPage{
		Links: links,
		Embedded: map[string]any{
			"items": embeddedItems,
		},
		Count: count,
		Total: total,
	}
}
