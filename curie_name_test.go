// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestCurieName_RejectsColonAndControls(t *testing.T) {
	t.Parallel()

	invalid := []string{"", "ac:me", "ac\x00me", "ac\nme", "ac\tme", "ac me", "ac\xffme"}
	for _, name := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := validateCurieName(name); !errors.Is(err, ErrInvalidCurie) {
				t.Fatalf("validateCurieName(%q) = %v, want ErrInvalidCurie", name, err)
			}
		})
	}

	valid := []string{"acme", "v1", "a_b.c-d", "_private", "éclair"}
	for _, name := range valid {
		if err := validateCurieName(name); err != nil {
			t.Errorf("validateCurieName(%q) = %v, want nil", name, err)
		}
	}
}

func TestCurieName_RegisterCuriePanics(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"ac:me", "ac\x00me"} {
		assertPanics(t, "hal: RegisterCurie:", func() {
			New().RegisterCurie(name, "https://docs.example/{rel}")
		})
	}
}

func TestCurieName_UserDeclarationRejectedOnMarshal(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "curies", Name: "ac:me", Href: "https://docs.example/{rel}", Templated: true})

	b, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidCurie) {
		t.Fatalf("error = %v (output %s), want ErrInvalidCurie", err, b)
	}
	if len(b) != 0 {
		t.Fatalf("error returned partial output: %s", b)
	}
}

func TestCurieName_StaticRejected(t *testing.T) {
	t.Parallel()

	assertPanics(t, "hal: RegisterStatic:", func() {
		RegisterStatic(New(), &embeddedTestUser{}, []Link{
			{Rel: "curies", Name: "ac:me", Href: "https://docs.example/{rel}", Templated: true},
		})
	})
}

func TestCurieName_PrecomputedRejected(t *testing.T) {
	t.Parallel()

	_, err := CompilePrecomputedLinks([]byte(`{"curies":[{"name":"ac:me","href":"https://docs.example/{rel}","templated":true}]}`))
	if !errors.Is(err, ErrInvalidCurie) {
		t.Fatalf("error = %v, want ErrInvalidCurie", err)
	}
}

func TestCurieName_RegisterCurieStillWorks(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/{rel}")
	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "acme:widget", Href: "/widgets/1"})
	if _, err := json.Marshal(env); err != nil {
		t.Fatalf("valid CURIE rejected: %v", err)
	}
}
