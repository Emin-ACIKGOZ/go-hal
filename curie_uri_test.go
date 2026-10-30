// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCurie_AbsoluteURIIsNotACurie(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("https", "https://docs.example/{rel}")
	inst.RegisterCurie("http", "https://docs.example/http/{rel}")

	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "https://example.com/rels/widget", Href: "/widgets/1"})
	env.AddLink(Link{Rel: "acme:widget", Href: "/widgets/2"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	links := root["_links"].(map[string]any)
	if _, exists := links["curies"]; exists {
		t.Fatalf("absolute URI produced a curies declaration: %s", b)
	}
	if _, exists := links["https://example.com/rels/widget"]; !exists {
		t.Fatal("absolute URI relation was lost")
	}
}

func TestCurie_PrecomputedAbsoluteURIIsNotACurie(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("https", "https://docs.example/{rel}")
	handle, err := CompilePrecomputedLinks([]byte(`{"https://example.com/rels/widget":{"href":"/w"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(handle.prefixes) != 0 {
		t.Fatalf("prefixes = %v, want none for an absolute URI", handle.prefixes)
	}
	b, err := json.Marshal(inst.WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle))
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, exists := root["_links"].(map[string]any)["curies"]; exists {
		t.Fatalf("precomputed absolute URI produced curies: %s", b)
	}
}

func TestCurie_StaticAbsoluteURIIsNotACurie(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("https", "https://docs.example/{rel}")
	RegisterStatic(inst, &embeddedTestUser{}, []Link{
		{Rel: "https://example.com/rels/widget", Href: "/w"},
	})
	b, err := json.Marshal(inst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, exists := root["_links"].(map[string]any)["curies"]; exists {
		t.Fatalf("static absolute URI produced curies: %s", b)
	}
}

func TestCurie_SchemeWithoutAuthorityStillDeclaredWhenRegistered(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("urn", "https://docs.example/urn/{rel}")
	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "urn:isbn:0451450523", Href: "/book"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["name"] != "urn" {
		t.Fatalf("curies = %#v, want urn declaration", curies)
	}
}
