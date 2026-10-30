// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Emin-ACIKGOZ/go-hal/internal/relation"
	"github.com/Emin-ACIKGOZ/go-hal/internal/uriref"
	"github.com/Emin-ACIKGOZ/go-hal/internal/uritemplate"
)

const specFixtureDir = "testdata/spec"

type specVectorFile struct {
	Valid             []string `json:"valid"`
	Invalid           []string `json:"invalid"`
	ReservedOperators []string `json:"reserved_operators"`
	PercentBraceData  []string `json:"percent_braces_are_data"`
}

func loadSpecJSON(t *testing.T, name string, dst any) {
	t.Helper()
	path := filepath.Join(specFixtureDir, name)
	data, err := os.ReadFile(path) //nolint:gosec // Test-only path from a constant fixture directory.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !json.Valid(data) {
		t.Fatalf("%s is not valid JSON", path)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func loadSpecDocument(t *testing.T, name string) map[string]any {
	t.Helper()
	var document map[string]any
	loadSpecJSON(t, name, &document)
	return document
}

func loadSpecVectors(t *testing.T, name string) specVectorFile {
	t.Helper()
	var vectors specVectorFile
	loadSpecJSON(t, name, &vectors)
	return vectors
}

func TestSpecFixturesAreValidAndUsable(t *testing.T) {
	t.Parallel()

	section6 := loadSpecDocument(t, "draft11-section6.json")
	if _, ok := section6["_links"].(map[string]any); !ok {
		t.Fatal("draft11 section 6 fixture lacks _links")
	}
	if _, ok := section6["_embedded"].(map[string]any); !ok {
		t.Fatal("draft11 section 6 fixture lacks _embedded")
	}

	section83 := loadSpecDocument(t, "draft11-section8-3-curies.json")
	links, ok := section83["_links"].(map[string]any)
	if !ok {
		t.Fatal("draft11 section 8.3 fixture lacks _links")
	}
	if _, ok := links["curies"].([]any); !ok {
		t.Fatal("draft11 section 8.3 fixture lacks a curies array")
	}

	section84 := loadSpecDocument(t, "draft11-section8-4-cache.json")
	if _, ok := section84["_embedded"].(map[string]any); !ok {
		t.Fatal("draft11 section 8.4 fixture lacks _embedded")
	}

	templates := loadSpecVectors(t, "rfc6570-templates.json")
	for _, raw := range templates.Valid {
		if _, err := uritemplate.Parse(raw); err != nil {
			t.Errorf("valid RFC 6570 fixture %q rejected: %v", raw, err)
		}
	}
	for _, raw := range templates.Invalid {
		if _, err := uritemplate.Parse(raw); err == nil {
			t.Errorf("invalid RFC 6570 fixture %q accepted", raw)
		}
	}
	for _, raw := range templates.ReservedOperators {
		tmpl, err := uritemplate.Parse(raw)
		if err != nil {
			t.Errorf("reserved-operator fixture %q did not parse: %v", raw, err)
			continue
		}
		if !tmpl.HasUnsupportedOperator() {
			t.Errorf("reserved-operator fixture %q reported supported", raw)
		}
	}
	for _, raw := range templates.PercentBraceData {
		tmpl, err := uritemplate.Parse(raw)
		if err != nil {
			t.Errorf("percent-brace fixture %q rejected: %v", raw, err)
			continue
		}
		if tmpl.HasExpressions() {
			t.Errorf("percent-brace fixture %q was treated as an expression", raw)
		}
	}

	refs := loadSpecVectors(t, "rfc3986-references.json")
	for _, ref := range refs.Valid {
		if err := uriref.Validate(ref); err != nil {
			t.Errorf("valid RFC 3986 fixture %q rejected: %v", ref, err)
		}
	}
	for _, ref := range refs.Invalid {
		if err := uriref.Validate(ref); err == nil {
			t.Errorf("invalid RFC 3986 fixture %q accepted", ref)
		}
	}
}

func TestSpecFixtureRelationClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ref  string
		want relation.Kind
	}{
		{"self", relation.Token},
		{"acme:widgets", relation.Curie},
		{"https://docs.acme.com/relations/widgets", relation.URI},
	}
	for _, tc := range tests {
		if got := relation.Classify(tc.ref); got != tc.want {
			t.Errorf("Classify(%q) = %v, want %v", tc.ref, got, tc.want)
		}
	}
}
