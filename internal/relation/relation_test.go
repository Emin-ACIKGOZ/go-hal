// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package relation

import "testing"

func TestClassify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ref  string
		want Kind
	}{
		{"self", Token},
		{"item", Token},
		{"acme:widget", Curie},
		{"v1:orders", Curie},
		{"https://example.com/rels/widget", URI},
		{"mailto:user@example.com", Curie},
		{"urn:isbn:0451450523", Curie},
		{"", Invalid},
		{" self", Invalid},
		{"self ", Invalid},
		{"self\t", Invalid},
		{"sel\x00f", Invalid},
		{"self\xff", Invalid},
		{":widget", Invalid},
		{"https://exa mple.com", Invalid},
	}
	for _, tc := range tests {
		if got := Classify(tc.ref); got != tc.want {
			t.Errorf("Classify(%q) = %v, want %v", tc.ref, got, tc.want)
		}
	}
}

func TestSplitCurie(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ref       string
		wantPfx   string
		wantRef   string
		wantValid bool
	}{
		{"acme:widget", "acme", "widget", true},
		{"a_b.c-d:value", "a_b.c-d", "value", true},
		{"https://example.com", "", "", false},
		{"acme:", "", "", false},
		{":widget", "", "", false},
		{"a/b:c", "", "", false},
		{"acme:widget:extra", "acme", "widget:extra", true},
		{" mail:x", "", "", false},
	}
	for _, tc := range tests {
		prefix, ref, ok := SplitCurie(tc.ref)
		if prefix != tc.wantPfx || ref != tc.wantRef || ok != tc.wantValid {
			t.Errorf("SplitCurie(%q) = (%q, %q, %t), want (%q, %q, %t)",
				tc.ref, prefix, ref, ok, tc.wantPfx, tc.wantRef, tc.wantValid)
		}
	}
}

func TestHasScheme(t *testing.T) {
	t.Parallel()
	for _, ref := range []string{"https://x", "mailto:a", "urn:isbn:1", "a+b:c", "a1:c"} {
		if !HasScheme(ref) {
			t.Errorf("HasScheme(%q) = false, want true", ref)
		}
	}
	for _, ref := range []string{"self", ":x", "1http:x", "a_b:x", ""} {
		if HasScheme(ref) {
			t.Errorf("HasScheme(%q) = true, want false", ref)
		}
	}
}

func TestIsToken(t *testing.T) {
	t.Parallel()
	for _, ref := range []string{"self", "a", "a1", "a.b-c_d"} {
		if !IsToken(ref) {
			t.Errorf("IsToken(%q) = false, want true", ref)
		}
	}
	for _, ref := range []string{"", "Self", "1self", "a:b", "a/b", "a b", "a~b"} {
		if IsToken(ref) {
			t.Errorf("IsToken(%q) = true, want false", ref)
		}
	}
}

func TestFoldTokenIsASCIIOnly(t *testing.T) {
	t.Parallel()
	if got := FoldToken("SELF"); got != "self" {
		t.Fatalf("FoldToken(SELF) = %q, want self", got)
	}
	if got := FoldToken("Ä"); got != "Ä" {
		t.Fatalf("FoldToken(Ä) = %q, non-ASCII must not fold", got)
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()
	for kind, want := range map[Kind]string{
		Invalid: "invalid",
		Token:   "token",
		Curie:   "curie",
		URI:     "uri",
	} {
		if got := kind.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", kind, got, want)
		}
	}
}
