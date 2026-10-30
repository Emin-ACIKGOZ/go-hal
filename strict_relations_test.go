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
