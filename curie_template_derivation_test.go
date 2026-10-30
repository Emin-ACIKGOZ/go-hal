// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func curieObjects(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	root := decodeJSONValue(t, data).(map[string]any)
	return relationObjects(t, root, "curies")
}

func TestCurieDeclarationsDeriveTemplatedFromUserLinks(t *testing.T) {
	t.Parallel()

	// A single AddLink is the single-Link shape.
	single := Link{Rel: "curies", Name: "acme", Href: "https://docs.example/{rel}"}
	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(single)
	if single.Templated {
		t.Fatal("AddLink mutated caller's declaration")
	}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	curies := curieObjects(t, b)
	if len(curies) != 1 || curies[0]["templated"] != true {
		t.Fatalf("single declaration = %#v, want templated true", curies)
	}

	// Repeated AddLink calls produce the []any representation.
	env = New().WrapRaw(&embeddedTestUser{ID: 1})
	for _, declaration := range []Link{
		{Rel: "curies", Name: "acme", Href: "https://docs.example/{rel}"},
		{Rel: "curies", Name: "other", Href: "https://docs.example/other/{rel}"},
	} {
		env.AddLink(declaration)
	}
	b, err = json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	curies = curieObjects(t, b)
	if len(curies) != 2 || curies[0]["templated"] != true || curies[1]["templated"] != true {
		t.Fatalf("repeated declarations = %#v, want both templated", curies)
	}
}

func TestCurieDeclarationsDeriveTemplatedFromStaticLinks(t *testing.T) {
	t.Parallel()

	// RegisterStatic consumes the []Link representation and must not mutate it.
	links := []Link{
		{Rel: "curies", Name: "acme", Href: "https://docs.example/{rel}"},
		{Rel: "curies", Name: "other", Href: "https://docs.example/other/{rel}"},
	}
	inst := New()
	RegisterStatic(inst, &embeddedTestUser{}, links)
	for _, link := range links {
		if link.Templated {
			t.Fatalf("RegisterStatic mutated caller's declaration: %#v", link)
		}
	}

	b, err := json.Marshal(inst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
	if err != nil {
		t.Fatal(err)
	}
	curies := curieObjects(t, b)
	if len(curies) != 2 || curies[0]["templated"] != true || curies[1]["templated"] != true {
		t.Fatalf("static declarations = %#v, want both templated", curies)
	}
}

func TestCurieDeclarationsDoNotFlagNonTemplates(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "curies", Name: "acme", Href: "https://docs.example/static"})
	env.AddLink(Link{Rel: "curies", Name: "other", Href: "https://docs.example/{=rel}"})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	curies := curieObjects(t, b)
	if len(curies) != 2 {
		t.Fatalf("curies = %#v, want 2", curies)
	}
	for _, declaration := range curies {
		if _, exists := declaration["templated"]; exists {
			t.Fatalf("non-template declaration claims templated: %#v", declaration)
		}
	}
}

func TestGeneratedCurieDeclarationsUseSharedDerivation(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/{rel}")
	inst.RegisterCurie("reserved", "https://docs.example/{=rel}")
	inst.RegisterCurie("literal", "https://docs.example/static")

	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "acme:widget", Href: "/w"})
	env.AddLink(Link{Rel: "reserved:widget", Href: "/r"})
	env.AddLink(Link{Rel: "literal:widget", Href: "/l"})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	curies := curieObjects(t, b)
	byName := make(map[string]map[string]any, len(curies))
	for _, declaration := range curies {
		name, _ := declaration["name"].(string)
		byName[name] = declaration
	}
	if byName["acme"]["templated"] != true {
		t.Fatalf("acme declaration = %#v, want templated true", byName["acme"])
	}
	if _, exists := byName["reserved"]["templated"]; exists {
		t.Fatalf("reserved operator declaration claims templated: %#v", byName["reserved"])
	}
	if _, exists := byName["literal"]["templated"]; exists {
		t.Fatalf("literal declaration claims templated: %#v", byName["literal"])
	}
}

func TestPrecomputedCurieBytesRemainVerbatim(t *testing.T) {
	t.Parallel()

	links := []byte(`{"curies":[{"name":"acme","href":"https://docs.example/{rel}"}]}`)
	handle, err := CompilePrecomputedLinks(links)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, links) {
		t.Fatalf("precomputed curie bytes were rewritten: %s", b)
	}
	if bytes.Contains(b, []byte(`"templated"`)) {
		t.Fatalf("precomputed curie bytes gained templated: %s", b)
	}
}
