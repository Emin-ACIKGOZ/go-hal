// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"reflect"
	"testing"
)

func TestAppendRelationValueParity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values []any
		want   any
	}{
		{name: "one", values: []any{"a"}, want: "a"},
		{name: "two", values: []any{"a", "b"}, want: []any{"a", "b"}},
		{name: "three", values: []any{"a", "b", "c"}, want: []any{"a", "b", "c"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			values := make(map[string]any)
			for _, val := range tc.values {
				appendRelationValue(values, "rel", val)
			}
			if got := values["rel"]; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("relation value = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestAppendRelationValueKeepsRelationsIndependent(t *testing.T) {
	t.Parallel()

	values := make(map[string]any)
	appendRelationValue(values, "self", "a")
	appendRelationValue(values, "item", "b")
	appendRelationValue(values, "self", "c")

	if got := values["self"]; !reflect.DeepEqual(got, []any{"a", "c"}) {
		t.Fatalf("self = %#v, want [a c]", got)
	}
	if got := values["item"]; got != "b" {
		t.Fatalf("item = %#v, want b", got)
	}
}
