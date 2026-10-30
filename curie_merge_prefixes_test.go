// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"slices"
	"testing"
)

func TestMergePrefixes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{name: "both empty"},
		{name: "empty a", b: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "empty b", a: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "disjoint", a: []string{"a", "c"}, b: []string{"b", "d"}, want: []string{"a", "b", "c", "d"}},
		{
			name: "overlapping",
			a:    []string{"a", "c", "e"},
			b:    []string{"b", "c", "d"},
			want: []string{"a", "b", "c", "d", "e"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mergePrefixes(tc.a, tc.b)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("mergePrefixes(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			switch tc.name {
			case "empty a":
				if len(tc.b) > 0 && &got[0] != &tc.b[0] {
					t.Fatal("mergePrefixes allocated when a was empty")
				}
			case "empty b":
				if len(tc.a) > 0 && &got[0] != &tc.a[0] {
					t.Fatal("mergePrefixes allocated when b was empty")
				}
			}
		})
	}
}
