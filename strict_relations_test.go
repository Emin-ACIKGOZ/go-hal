// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestWithStrictRelations_Accepts(t *testing.T) {
	t.Parallel()

	inst := New(WithStrictRelations())
	inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")

	valid := []string{
		"self",
		"a.b-c_d",
		"https://example.com/rels/widget",
		"urn:isbn:0451450523",
		"mailto:user@example.com",
		"tag:example.com,2026:widget",
		"acme:widget",
		"undeclared:widget",
	}
	for _, rel := range valid {
		t.Run(rel, func(t *testing.T) {
			env := inst.WrapRaw(&embeddedTestUser{ID: 1})
			env.AddLink(Link{Rel: rel, Href: "/x"})
			if _, err := json.Marshal(env); err != nil {
				t.Fatalf("relation %q rejected: %v", rel, err)
			}
		})
	}
}

func TestWithStrictRelations_Rejects(t *testing.T) {
	t.Parallel()

	inst := New(WithStrictRelations())
	invalid := []string{"Self", "SELF", "/relative", "../up", ":x", "foo/bar", "a b"}
	for _, rel := range invalid {
		t.Run(rel, func(t *testing.T) {
			env := inst.WrapRaw(&embeddedTestUser{ID: 1})
			env.AddLink(Link{Rel: rel, Href: "/x"})
			_, err := json.Marshal(env)
			if !errors.Is(err, ErrInvalidRelation) {
				t.Fatalf("relation %q error = %v, want ErrInvalidRelation", rel, err)
			}
		})
	}
}

func TestWithStrictRelations_AddEmbedded(t *testing.T) {
	t.Parallel()

	env := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("Author", &embeddedTestUser{ID: 2})

	_, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("error = %v, want ErrInvalidRelation", err)
	}
}

func TestWithStrictRelations_RegisterStatic(t *testing.T) {
	t.Parallel()

	assertPanics(t, "hal: RegisterStatic:", func() {
		RegisterStatic(New(WithStrictRelations()), &embeddedTestUser{}, []Link{
			{Rel: "Self", Href: "/x"},
		})
	})
}

func TestWithStrictRelations_Precomputed(t *testing.T) {
	t.Parallel()

	handle, err := CompilePrecomputedLinks([]byte(`{"Self":{"href":"/x"}}`))
	if err != nil {
		t.Fatal(err)
	}

	env := New(WithStrictRelations()).WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle)
	_, err = json.Marshal(env)
	if !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("error = %v, want ErrInvalidRelation", err)
	}

	env = New(WithStrictRelations()).WrapPrecomputed(
		context.Background(),
		&embeddedTestUser{ID: 1},
		[]byte(`{"Self":{"href":"/x"}}`),
	)
	_, err = json.Marshal(env)
	if !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("WrapPrecomputed error = %v, want ErrInvalidRelation", err)
	}
}

func TestWithStrictRelations_DoesNotChangeDefault(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "Self", Href: "/x"})
	if _, err := json.Marshal(env); err != nil {
		t.Fatalf("default relation rejected: %v", err)
	}
}

func TestWithStrictRelations_AcceptsDeclaredNonSchemeCurie(t *testing.T) {
	t.Parallel()

	inst := New(WithStrictRelations(), WithStrictCuries())
	inst.RegisterCurie("_private", "https://docs.example/{rel}")

	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "_private:widget", Href: "/widgets/1"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("declared non-scheme CURIE rejected: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if len(relationObjects(t, root, "_private:widget")) != 1 {
		t.Fatalf("declared CURIE relation lost: %s", b)
	}
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["name"] != "_private" {
		t.Fatalf("curies = %#v, want _private declaration", curies)
	}
}

func TestWithStrictRelations_UndeclaredNonAuthorityFormIsOpaqueURI(t *testing.T) {
	t.Parallel()

	env := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "acme:widget", Href: "/widgets/1"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("undeclared non-authority form rejected: %v", err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if len(relationObjects(t, root, "acme:widget")) != 1 {
		t.Fatalf("opaque URI relation lost: %s", b)
	}
	if _, exists := root["_links"].(map[string]any)["curies"]; exists {
		t.Fatalf("undeclared prefix produced a declaration: %s", b)
	}
}
