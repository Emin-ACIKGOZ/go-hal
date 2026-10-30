// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func privateCurieDeclaration() Link {
	return Link{
		Rel:       "curies",
		Name:      "_private",
		Href:      "https://docs.example/rels/{rel}",
		Templated: true,
	}
}

func TestStrictRelations_LocalDeclarationBeforeAndAfterUse(t *testing.T) {
	t.Parallel()

	for _, order := range []string{"before", "after"} {
		for _, target := range []string{"link", "embedded", "cached"} {
			t.Run(order+"/"+target, func(t *testing.T) {
				t.Parallel()

				env := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 1})
				declaration := privateCurieDeclaration()
				use := Link{Rel: "_private:widget", Href: "/widgets/1"}
				addDeclaration := func() { env.AddLink(declaration) }
				addUse := func() {
					switch target {
					case "link":
						env.AddLink(use)
					case "embedded":
						env.AddEmbedded(use.Rel, &embeddedTestUser{ID: 2})
					case "cached":
						env.AddCachedLink(use, &embeddedTestUser{ID: 2})
					}
				}
				if order == "before" {
					addDeclaration()
					addUse()
				} else {
					addUse()
					addDeclaration()
				}

				b, err := json.Marshal(env)
				if err != nil {
					t.Fatalf("declared CURIE rejected: %v", err)
				}
				root := decodeJSONValue(t, b).(map[string]any)
				curies := relationObjects(t, root, "curies")
				if len(curies) != 1 || curies[0]["name"] != "_private" {
					t.Fatalf("curies = %#v, want _private", curies)
				}
			})
		}
	}
}

func TestStrictRelations_UndeclaredNonSchemeCurieFailsAtMarshal(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"link", "embedded"} {
		t.Run(name, func(t *testing.T) {
			env := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 1})
			if name == "link" {
				env.AddLink(Link{Rel: "_private:widget", Href: "/widgets/1"})
			} else {
				env.AddEmbedded("_private:widget", &embeddedTestUser{ID: 2})
			}

			b, err := json.Marshal(env)
			if !errors.Is(err, ErrInvalidRelation) {
				t.Fatalf("error = %v (output %s), want ErrInvalidRelation", err, b)
			}
			if len(b) != 0 {
				t.Fatalf("error returned partial output: %s", b)
			}
			if !strings.Contains(err.Error(), `"_private:widget" is not a token, URI, or declared CURIE`) {
				t.Fatalf("error = %q, want the existing inner message", err)
			}
		})
	}
}

func TestStrictRelations_StaticDeclarationAndUse(t *testing.T) {
	t.Parallel()

	for _, links := range [][]Link{
		{
			{Rel: "_private:widget", Href: "/widgets/1"},
			privateCurieDeclaration(),
		},
		{
			privateCurieDeclaration(),
			{Rel: "_private:widget", Href: "/widgets/1"},
		},
	} {
		inst := New(WithStrictRelations())
		RegisterStatic(inst, &embeddedTestUser{}, links)
		b, err := json.Marshal(inst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
		if err != nil {
			t.Fatalf("static declared CURIE rejected: %v", err)
		}
		root := decodeJSONValue(t, b).(map[string]any)
		curies := relationObjects(t, root, "curies")
		if len(curies) != 1 || curies[0]["name"] != "_private" {
			t.Fatalf("curies = %#v, want _private", curies)
		}
	}
}

func TestStrictRelations_StaticUndeclaredNonSchemeCuriePanics(t *testing.T) {
	t.Parallel()

	assertPanics(t, "hal: RegisterStatic:", func() {
		RegisterStatic(New(WithStrictRelations()), &embeddedTestUser{}, []Link{
			{Rel: "_private:widget", Href: "/widgets/1"},
		})
	})
}

func TestStrictRelations_PrecomputedLocalDeclaration(t *testing.T) {
	t.Parallel()

	links := []byte(`{
		"curies":[{"name":"_private","href":"https://docs.example/rels/{rel}","templated":true}],
		"_private:widget":{"href":"/widgets/1"}
	}`)
	handle, err := CompilePrecomputedLinks(links)
	if err != nil {
		t.Fatal(err)
	}
	env := New(WithStrictRelations()).WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle)
	if env.err != nil {
		t.Fatalf("wrap-time validation rejected a locally declared CURIE: %v", env.err)
	}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal rejected a locally declared CURIE: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["name"] != "_private" {
		t.Fatalf("curies = %#v, want _private", curies)
	}
}

func TestStrictRelations_PrecomputedDeferredUntilMarshal(t *testing.T) {
	t.Parallel()

	handle, err := CompilePrecomputedLinks([]byte(`{"_private:widget":{"href":"/widgets/1"}}`))
	if err != nil {
		t.Fatal(err)
	}
	env := New(WithStrictRelations()).WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle)
	if env.err != nil {
		t.Fatalf("wrap-time validation did not defer the local-CURIE check: %v", env.err)
	}
	b, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("error = %v (output %s), want ErrInvalidRelation", err, b)
	}
	if !strings.Contains(err.Error(), `"_private:widget" is not a token, URI, or declared CURIE`) {
		t.Fatalf("error = %q, want the existing inner message", err)
	}
}

func TestStrictRelations_PrecomputedLocalDeclarationWithoutEmbeddedChildren(t *testing.T) {
	t.Parallel()

	handle, err := CompilePrecomputedLinks([]byte(`{
		"curies":[{"name":"_private","href":"https://docs.example/rels/{rel}","templated":true}],
		"self":{"href":"/x"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	env := New(WithStrictRelations()).WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle)
	if _, err := json.Marshal(env); err != nil {
		t.Fatalf("local declaration with no embedded children was ignored: %v", err)
	}
}

func TestStrictRelations_PrecomputedChildInheritsParentDeclaration(t *testing.T) {
	t.Parallel()

	parent := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddLink(privateCurieDeclaration())

	childHandle, err := CompilePrecomputedLinks([]byte(`{"_private:widget":{"href":"/widgets/1"}}`))
	if err != nil {
		t.Fatal(err)
	}
	child := New(WithStrictRelations()).WrapPrecomputedLinks(&embeddedTestUser{ID: 2}, childHandle)
	parent.AddEmbedded("child", child)

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatalf("child did not inherit the parent declaration: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	childRoot := embeddedObject(t, root, "child")
	if len(relationObjects(t, childRoot, "_private:widget")) != 1 {
		t.Fatalf("child relation lost: %s", b)
	}
}

func TestStrictRelations_GeneratedChildInheritsPrecomputedParentDeclaration(t *testing.T) {
	t.Parallel()

	parentHandle, err := CompilePrecomputedLinks([]byte(`{
		"curies":[{"name":"_private","href":"https://docs.example/rels/{rel}","templated":true}],
		"self":{"href":"/parent"}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	parent := New(WithStrictRelations()).WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, parentHandle)
	child := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 2})
	child.AddEmbedded("_private:widgets", &embeddedTestUser{ID: 3})
	parent.AddEmbedded("children", child)

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatalf("generated child did not inherit the precomputed parent declaration: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	childRoot := embeddedObject(t, root, "children")
	embeddedChild := embeddedObject(t, childRoot, "_private:widgets")
	if embeddedChild["id"] != json.Number("3") {
		t.Fatalf("child relation lost: %s", b)
	}
}

func TestStrictRelations_SiblingDeclarationsDoNotLeak(t *testing.T) {
	t.Parallel()

	parent := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 1})

	first := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 2})
	first.AddLink(privateCurieDeclaration())
	first.AddLink(Link{Rel: "_private:widget", Href: "/first"})

	second := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 3})
	second.AddLink(Link{Rel: "_private:widget", Href: "/second"})

	parent.AddEmbedded("first", first)
	parent.AddEmbedded("second", second)

	b, err := json.Marshal(parent)
	if !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("error = %v (output %s), want ErrInvalidRelation", err, b)
	}
	if !strings.Contains(err.Error(), `"_private:widget" is not a token, URI, or declared CURIE`) {
		t.Fatalf("error = %q, want the existing inner message", err)
	}
}

func TestStrictRelations_RegisteredEmbeddedPrefixIsDeclaredAndAccepted(t *testing.T) {
	t.Parallel()

	inst := New(WithStrictRelations())
	inst.RegisterCurie("_private", "https://docs.example/rels/{rel}")
	parent := inst.WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("_private:widget", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatalf("registered non-scheme CURIE key rejected: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["name"] != "_private" {
		t.Fatalf("curies = %#v, want _private", curies)
	}
}

func TestStrictRelations_DefaultStillAcceptsUndeclaredNonSchemeCurie(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "_private:widget", Href: "/widgets/1"})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("default mode rejected an undeclared CURIE: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, exists := root["_links"].(map[string]any)["curies"]; exists {
		t.Fatalf("default mode invented a declaration: %s", b)
	}
}
