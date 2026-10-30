// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestWithStrictCuries_Names(t *testing.T) {
	t.Parallel()

	valid := []string{"acme", "_private", "A1", "a.b-c_d"}
	for _, name := range valid {
		if err := validateCurieName(name, true); err != nil {
			t.Errorf("strict name %q rejected: %v", name, err)
		}
	}

	invalid := []string{"1acme", "-acme", ".acme", "ac:me", "ac me", "éclair", ""}
	for _, name := range invalid {
		if err := validateCurieName(name, true); !errors.Is(err, ErrInvalidCurie) {
			t.Errorf("strict name %q = %v, want ErrInvalidCurie", name, err)
		}
	}
}

func TestWithStrictCuries_DefaultRemainsPermissive(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"1acme", "-acme", "éclair"} {
		if err := validateCurieName(name, false); err != nil {
			t.Errorf("default name %q rejected: %v", name, err)
		}
	}
}

func TestWithStrictCuries_RegisterCurie(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"1acme", "-acme", "ac:me"} {
		assertPanics(t, "hal: RegisterCurie:", func() {
			New(WithStrictCuries()).RegisterCurie(name, "https://docs.example/{rel}")
		})
	}

	inst := New(WithStrictCuries())
	inst.RegisterCurie("acme", "https://docs.example/{rel}")
	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "acme:widget", Href: "/w"})
	if _, err := json.Marshal(env); err != nil {
		t.Fatalf("valid strict CURIE rejected: %v", err)
	}
}

func TestWithStrictCuries_UserDeclaration(t *testing.T) {
	t.Parallel()

	env := New(WithStrictCuries()).WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "curies", Name: "1acme", Href: "https://docs.example/{rel}", Templated: true})

	_, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidCurie) {
		t.Fatalf("error = %v, want ErrInvalidCurie", err)
	}
}

func TestWithStrictCuries_RegisterStatic(t *testing.T) {
	t.Parallel()

	assertPanics(t, "hal: RegisterStatic:", func() {
		RegisterStatic(New(WithStrictCuries()), &embeddedTestUser{}, []Link{
			{Rel: "curies", Name: "1acme", Href: "https://docs.example/{rel}", Templated: true},
		})
	})
}

func TestWithStrictCuries_Precomputed(t *testing.T) {
	t.Parallel()

	invalid := []byte(`{"curies":[{"name":"1acme","href":"https://docs.example/{rel}","templated":true}]}`)
	if _, err := CompilePrecomputedLinksStrict(invalid); !errors.Is(err, ErrInvalidCurie) {
		t.Fatalf("strict compile error = %v, want ErrInvalidCurie", err)
	}
	if _, err := CompilePrecomputedLinks(invalid); err != nil {
		t.Fatalf("structural compile rejected strict-only name: %v", err)
	}

	env := New(WithStrictCuries()).WrapPrecomputed(context.Background(), &embeddedTestUser{ID: 1}, invalid)
	if _, err := json.Marshal(env); !errors.Is(err, ErrInvalidCurie) {
		t.Fatalf("WrapPrecomputed error = %v, want ErrInvalidCurie", err)
	}
}

func TestWithStrictCuries_RejectsNonStrictHandle(t *testing.T) {
	t.Parallel()

	assertRejectsNonStrictPrecomputedHandle(t, WithStrictCuries(), ErrInvalidCurie)
}
