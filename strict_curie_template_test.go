// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestStrictCurie_RegisterCurieTemplates(t *testing.T) {
	t.Parallel()

	valid := []string{
		"https://docs.example/{rel}",
		"https://docs.example/{+rel}",
		"urn:example:{rel}",
	}
	for _, href := range valid {
		if err := validateCurieDeclarations([]Link{{
			Name: "acme", Href: href, Templated: curieHrefTemplated(href),
		}}, true); err != nil {
			t.Errorf("strict CURIE href %q rejected: %v", href, err)
		}
	}

	invalid := []string{
		"https://docs.example/{?rel}",
		"https://docs.example/{#rel}",
		"https://docs.example/static",
		"https://docs.example/{other}",
		"https://docs.example/{rel}/{other}",
		"https://docs.example/{Rel}",
		"https://docs.example/{rel:3}",
		"https://docs.example/{rel*}",
		"https://docs.example/%7Brel%7D",
		"https://docs.example/{",
	}
	for _, href := range invalid {
		if err := validateCurieDeclarations([]Link{{
			Name: "acme", Href: href, Templated: curieHrefTemplated(href),
		}}, true); !errors.Is(err, ErrInvalidCurie) {
			t.Errorf("strict CURIE href %q = %v, want ErrInvalidCurie", href, err)
		}
	}
}

func TestStrictCurie_TemplatedMustBeTrue(t *testing.T) {
	t.Parallel()

	err := validateCurieDeclarations([]Link{{
		Name: "acme",
		Href: "https://docs.example/{rel}",
	}}, true)
	if !errors.Is(err, ErrInvalidCurie) {
		t.Fatalf("error = %v, want ErrInvalidCurie", err)
	}
}

func TestStrictCurie_UserDeclaration(t *testing.T) {
	t.Parallel()

	env := New(WithStrictCuries()).WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{
		Rel:       "curies",
		Name:      "acme",
		Href:      "https://docs.example/{?rel}",
		Templated: true,
	})
	_, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidCurie) {
		t.Fatalf("error = %v, want ErrInvalidCurie", err)
	}
}

func TestStrictCurie_DefaultRemainsLenient(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{
		Rel:  "curies",
		Name: "acme",
		Href: "https://docs.example/static",
	})
	if _, err := json.Marshal(env); err != nil {
		t.Fatalf("default CURIE rejected: %v", err)
	}
}

func TestCurieGeneratedTemplatedIsDerived(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/{rel}")
	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "acme:widget", Href: "/w"})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["templated"] != true {
		t.Fatalf("templated declaration = %#v, want true", curies)
	}

	staticInst := New()
	staticInst.RegisterCurie("acme", "https://docs.example/static")
	env = staticInst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "acme:widget", Href: "/w"})
	b, err = json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root = decodeJSONValue(t, b).(map[string]any)
	curies = relationObjects(t, root, "curies")
	if len(curies) != 1 {
		t.Fatalf("got %d curies, want 1", len(curies))
	}
	if _, exists := curies[0]["templated"]; exists {
		t.Fatalf("non-template declaration claims templated: %#v", curies[0])
	}
}

func TestStrictCurie_PrecomputedTemplate(t *testing.T) {
	t.Parallel()

	invalid := [][]byte{
		[]byte(`{"curies":[{"name":"acme","href":"https://docs.example/{?rel}","templated":true}]}`),
		[]byte(`{"curies":[{"name":"acme","href":"https://docs.example/{rel}","templated":false}]}`),
		[]byte(`{"curies":[{"name":"acme","href":"https://docs.example/{rel}"}]}`),
	}
	for _, links := range invalid {
		if _, err := CompilePrecomputedLinksStrict(links); !errors.Is(err, ErrInvalidCurie) {
			t.Errorf("CompilePrecomputedLinksStrict(%s) = %v, want ErrInvalidCurie", links, err)
		}
	}
}

func TestStrictCurie_StaticTemplateDerivesTemplated(t *testing.T) {
	t.Parallel()

	inst := New(WithStrictCuries())
	RegisterStatic(inst, &embeddedTestUser{}, []Link{
		{Rel: "curies", Name: "acme", Href: "https://docs.example/{rel}"},
	})
	b, err := json.Marshal(inst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
	if err != nil {
		t.Fatalf("static declaration rejected: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["templated"] != true {
		t.Fatalf("curies = %#v, want derived templated true", curies)
	}
}
