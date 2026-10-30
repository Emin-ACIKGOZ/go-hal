// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

func embeddedObject(t *testing.T, root map[string]any, rel string) map[string]any {
	t.Helper()
	embedded, ok := root["_embedded"].(map[string]any)
	if !ok {
		t.Fatalf("_embedded has type %T, want object", root["_embedded"])
	}
	child, ok := embedded[rel].(map[string]any)
	if !ok {
		t.Fatalf("%s has type %T, want object", rel, embedded[rel])
	}
	return child
}

func TestCurieScope_ParentFallbackAvoidsRedundantChildDeclaration(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")

	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddLink(Link{Rel: "acme:parent", Href: "/parent"})

	child := inst.WrapRaw(&embeddedTestUser{ID: 2})
	child.AddLink(Link{Rel: "acme:child", Href: "/child"})
	parent.AddEmbedded("child", child)

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if len(relationObjects(t, root, "curies")) != 1 {
		t.Fatal("parent did not emit its CURIE declaration")
	}
	childRoot := embeddedObject(t, root, "child")
	links := childRoot["_links"].(map[string]any)
	if _, exists := links["curies"]; exists {
		t.Fatalf("child redundantly redeclared inherited CURIE: %s", b)
	}
	if _, exists := links["acme:child"]; !exists {
		t.Fatal("child lost its inherited-prefix relation")
	}
}

func TestCurieScope_LocalDeclarationOverridesParent(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://registered.example/{rel}")

	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddLink(Link{Rel: "curies", Name: "acme", Href: "https://parent.example/{rel}", Templated: true})
	parent.AddLink(Link{Rel: "acme:parent", Href: "/parent"})

	child := inst.WrapRaw(&embeddedTestUser{ID: 2})
	child.AddLink(Link{Rel: "curies", Name: "acme", Href: "https://child.example/{rel}", Templated: true})
	child.AddLink(Link{Rel: "acme:child", Href: "/child"})
	parent.AddEmbedded("child", child)

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	parentCuries := relationObjects(t, root, "curies")
	if len(parentCuries) != 1 || parentCuries[0]["href"] != "https://parent.example/{rel}" {
		t.Fatalf("parent curies = %#v", parentCuries)
	}
	childRoot := embeddedObject(t, root, "child")
	childCuries := relationObjects(t, childRoot, "curies")
	if len(childCuries) != 1 || childCuries[0]["href"] != "https://child.example/{rel}" {
		t.Fatalf("child curies = %#v, want local override", childCuries)
	}
}

func TestCurieScope_DescendantInheritsNearestOverride(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://registered.example/{rel}")

	rootEnv := inst.WrapRaw(&embeddedTestUser{ID: 1})
	rootEnv.AddLink(Link{Rel: "curies", Name: "acme", Href: "https://root.example/{rel}", Templated: true})
	rootEnv.AddLink(Link{Rel: "acme:root", Href: "/root"})

	mid := inst.WrapRaw(&embeddedTestUser{ID: 2})
	mid.AddLink(Link{Rel: "curies", Name: "acme", Href: "https://mid.example/{rel}", Templated: true})
	mid.AddLink(Link{Rel: "acme:mid", Href: "/mid"})
	rootEnv.AddEmbedded("mid", mid)

	leaf := inst.WrapRaw(&embeddedTestUser{ID: 3})
	leaf.AddLink(Link{Rel: "acme:leaf", Href: "/leaf"})
	mid.AddEmbedded("leaf", leaf)

	b, err := json.Marshal(rootEnv)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	midRoot := embeddedObject(t, root, "mid")
	leafRoot := embeddedObject(t, midRoot, "leaf")
	links := leafRoot["_links"].(map[string]any)
	if _, exists := links["curies"]; exists {
		t.Fatalf("leaf redundantly declared an inherited prefix: %s", b)
	}
}

func TestCurieScope_UnusedParentPrefixIsDeclaredByChild(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")

	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})
	child := inst.WrapRaw(&embeddedTestUser{ID: 2})
	child.AddLink(Link{Rel: "acme:child", Href: "/child"})
	parent.AddEmbedded("child", child)

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, exists := root["_links"]; exists {
		t.Fatalf("parent emitted links it did not have: %s", b)
	}
	childRoot := embeddedObject(t, root, "child")
	if len(relationObjects(t, childRoot, "curies")) != 1 {
		t.Fatalf("child did not declare its own used prefix: %s", b)
	}
}

func TestCurieScope_SiblingsDoNotShareLocalOverrides(t *testing.T) {
	t.Parallel()

	inst := New()
	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})

	for _, name := range []string{"first", "second"} {
		child := inst.WrapRaw(&embeddedTestUser{ID: 2})
		child.AddLink(Link{
			Rel: "curies", Name: "acme",
			Href:      "https://" + name + ".example/{rel}",
			Templated: true,
		})
		child.AddLink(Link{Rel: "acme:item", Href: "/" + name})
		parent.AddEmbedded(name, child)
	}

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	first := relationObjects(t, embeddedObject(t, root, "first"), "curies")
	second := relationObjects(t, embeddedObject(t, root, "second"), "curies")
	if first[0]["href"] != "https://first.example/{rel}" {
		t.Fatalf("first curies = %#v", first)
	}
	if second[0]["href"] != "https://second.example/{rel}" {
		t.Fatalf("second curies = %#v", second)
	}
}

func TestCurieScope_SliceChildrenParticipate(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/rels/{rel}")

	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddLink(Link{Rel: "acme:parent", Href: "/parent"})

	child := inst.WrapRaw(&embeddedTestUser{ID: 2})
	child.AddLink(Link{Rel: "acme:child", Href: "/child"})
	parent.AddEmbedded("items", []*Envelope{child})

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	childRoot, ok := root["_embedded"].(map[string]any)["items"].(map[string]any)
	if !ok {
		t.Fatalf("items = %#v, want object for a one-element slice", root["_embedded"])
	}
	if _, exists := childRoot["_links"].(map[string]any)["curies"]; exists {
		t.Fatalf("slice child redeclared inherited CURIE: %s", b)
	}
}

func TestCurieScope_StaticParentParticipates(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://registered.example/{rel}")
	RegisterStatic(inst, &embeddedTestUser{}, []Link{
		{Rel: "curies", Name: "acme", Href: "https://static.example/{rel}", Templated: true},
		{Rel: "acme:parent", Href: "/parent"},
	})

	parent := inst.Wrap(context.Background(), &embeddedTestUser{ID: 1})
	child := inst.WrapRaw(&embeddedTestUser{ID: 2})
	child.AddLink(Link{Rel: "acme:child", Href: "/child"})
	parent.AddEmbedded("child", child)

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	childRoot := embeddedObject(t, root, "child")
	if _, exists := childRoot["_links"].(map[string]any)["curies"]; exists {
		t.Fatalf("static child redeclared inherited CURIE: %s", b)
	}
}

func TestCurieScope_PrecomputedParentParticipates(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://registered.example/{rel}")
	handle, err := CompilePrecomputedLinks([]byte(`{
		"curies":[{"name":"acme","href":"https://precomputed.example/{rel}","templated":true}],
		"acme:parent":{"href":"/parent"}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	parent := inst.WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle)
	child := inst.WrapRaw(&embeddedTestUser{ID: 2})
	child.AddLink(Link{Rel: "acme:child", Href: "/child"})
	parent.AddEmbedded("child", child)

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	childRoot := embeddedObject(t, root, "child")
	if _, exists := childRoot["_links"].(map[string]any)["curies"]; exists {
		t.Fatalf("precomputed child redeclared inherited CURIE: %s", b)
	}
}
