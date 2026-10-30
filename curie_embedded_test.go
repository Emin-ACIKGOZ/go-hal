// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestEmbeddedOnlyPrefixIsDeclaredByGeneratedParent(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")

	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})
	child := inst.WrapRaw(&embeddedTestUser{ID: 2})
	child.AddLink(Link{Rel: "acme:child", Href: "/child"})
	parent.AddEmbedded("acme:widget", child)

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["name"] != "acme" {
		t.Fatalf("parent curies = %#v, want acme", curies)
	}
	childRoot := embeddedObject(t, root, "acme:widget")
	if _, exists := childRoot["_links"].(map[string]any)["curies"]; exists {
		t.Fatalf("child redundantly redeclared the inherited prefix: %s", b)
	}
}

func TestEmbeddedOnlyPrefixIsDeclaredForSliceChildren(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")

	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})
	first := inst.WrapRaw(&embeddedTestUser{ID: 2})
	second := inst.WrapRaw(&embeddedTestUser{ID: 3})
	parent.AddEmbedded("acme:widgets", []*Envelope{first, second})

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["name"] != "acme" {
		t.Fatalf("parent curies = %#v, want acme", curies)
	}
	items, ok := root["_embedded"].(map[string]any)["acme:widgets"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("embedded slice = %#v, want 2 resources", root["_embedded"])
	}
}

func TestEmbeddedOnlyPrefixIsDeclaredByStaticParent(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")
	RegisterStatic(inst, &embeddedTestUser{}, []Link{{Rel: "self", Href: "/users/1"}})

	parent := inst.Wrap(context.Background(), &embeddedTestUser{ID: 1})
	parent.AddEmbedded("acme:widget", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["name"] != "acme" {
		t.Fatalf("static parent curies = %#v, want acme", curies)
	}
}

func TestEmbeddedOnlyPrefixIsSplicedIntoPrecomputedParent(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")
	handle, err := CompilePrecomputedLinks([]byte(`{"self":{"href":"/x","x-vendor":"kept"}}`))
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), handle.JSON...)

	parent := inst.WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle)
	parent.AddEmbedded("acme:widget", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(handle.JSON, before) {
		t.Fatalf("marshaling mutated the reusable handle: %s", handle.JSON)
	}
	if !bytes.Contains(b, []byte(`"x-vendor":"kept"`)) {
		t.Fatalf("precomputed bytes were not preserved: %s", b)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["name"] != "acme" {
		t.Fatalf("precomputed parent curies = %#v, want acme", curies)
	}
}

func TestEmbeddedOnlyPrefixSplicedIntoExistingPrecomputedCuries(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")
	handle, err := CompilePrecomputedLinks([]byte(`{
		"curies":[{"name":"user","href":"https://user.example/{rel}","templated":true,"x-ext":"keep"}],
		"self":{"href":"/x"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), handle.JSON...)

	parent := inst.WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle)
	parent.AddEmbedded("acme:widget", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(handle.JSON, before) {
		t.Fatalf("marshaling mutated the reusable handle: %s", handle.JSON)
	}
	if !bytes.Contains(b, []byte(`"x-ext":"keep"`)) {
		t.Fatalf("existing declaration bytes were not preserved: %s", b)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 2 {
		t.Fatalf("got %d curies, want 2: %#v", len(curies), curies)
	}
	if curies[0]["name"] != "user" || curies[1]["name"] != "acme" {
		t.Fatalf("curies = %#v, want existing user then generated acme", curies)
	}
	if curies[0]["name"] == curies[1]["name"] {
		t.Fatalf("duplicate declarations after splice: %#v", curies)
	}
}

func TestEmbeddedAndLinkPrefixesAreDeclaredOnce(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")

	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddLink(Link{Rel: "acme:parent", Href: "/parent"})
	parent.AddEmbedded("acme:widget", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if got := len(relationObjects(t, root, "curies")); got != 1 {
		t.Fatalf("got %d curies, want exactly one declaration: %s", got, b)
	}
}

func TestUnregisteredEmbeddedPrefixIsNotDeclared(t *testing.T) {
	t.Parallel()

	parent := New().WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("acme:widget", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, exists := root["_links"]; exists {
		t.Fatalf("unregistered prefix produced links: %s", b)
	}
}

func TestEmbeddedPrefixNoPrefixOutputIsByteIdentical(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")
	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("child", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"id":1,"_embedded":{"child":{"id":2}}}`)
	if !bytes.Equal(b, want) {
		t.Fatalf("output = %s, want byte-identical %s", b, want)
	}
}

func TestEmbeddedOnlyPrefixInChildDoesNotLeakToParent(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")

	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})
	child := inst.WrapRaw(&embeddedTestUser{ID: 2})
	child.AddEmbedded("acme:leaf", &embeddedTestUser{ID: 3})
	parent.AddEmbedded("child", child)

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, exists := root["_links"]; exists {
		t.Fatalf("parent declared a child-only prefix: %s", b)
	}
	childRoot := embeddedObject(t, root, "child")
	curies := relationObjects(t, childRoot, "curies")
	if len(curies) != 1 || curies[0]["name"] != "acme" {
		t.Fatalf("child curies = %#v, want acme", curies)
	}
}
