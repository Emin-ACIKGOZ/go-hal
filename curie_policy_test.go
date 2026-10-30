// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const (
	nonTemplateCurie = "https://docs.example/relations/widgets"
	wrongExpression  = "https://docs.example/{?rel}"
)

func TestCuriePolicy_DefaultAcceptsNonTemplateAndEmitsUnchanged(t *testing.T) {
	t.Parallel()

	// RegisterCurie keeps the permissive default.
	inst := New()
	inst.RegisterCurie("acme", nonTemplateCurie)
	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "acme:widget", Href: "/w"})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("default RegisterCurie rejected a non-template href: %v", err)
	}
	curies := curieObjects(t, b)
	if len(curies) != 1 || curies[0]["href"] != nonTemplateCurie {
		t.Fatalf("curies = %#v, want the registered href unchanged", curies)
	}
	if _, exists := curies[0]["templated"]; exists {
		t.Fatalf("non-template declaration claims templated: %#v", curies[0])
	}

	// A user declaration is emitted unchanged as well.
	env = New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "curies", Name: "user", Href: nonTemplateCurie})
	b, err = json.Marshal(env)
	if err != nil {
		t.Fatalf("default user declaration rejected: %v", err)
	}
	curies = curieObjects(t, b)
	if len(curies) != 1 || curies[0]["href"] != nonTemplateCurie {
		t.Fatalf("user curies = %#v", curies)
	}
	if _, exists := curies[0]["templated"]; exists {
		t.Fatalf("user non-template declaration claims templated: %#v", curies[0])
	}

	// Static and precomputed declarations follow the same default.
	staticInst := New()
	RegisterStatic(staticInst, &embeddedTestUser{}, []Link{{Rel: "curies", Name: "acme", Href: nonTemplateCurie}})
	b, err = json.Marshal(staticInst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
	if err != nil {
		t.Fatalf("default static declaration rejected: %v", err)
	}
	if curies := curieObjects(t, b); len(curies) != 1 || curies[0]["href"] != nonTemplateCurie {
		t.Fatalf("static curies = %#v", curies)
	}

	links := []byte(`{"curies":[{"name":"acme","href":"` + nonTemplateCurie + `"}]}`)
	handle, err := CompilePrecomputedLinks(links)
	if err != nil {
		t.Fatalf("default precomputed declaration rejected: %v", err)
	}
	b, err = json.Marshal(New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle))
	if err != nil {
		t.Fatalf("default precomputed marshal rejected: %v", err)
	}
	if !bytes.Contains(b, links) {
		t.Fatalf("precomputed declaration bytes changed: %s", b)
	}
}

func TestCuriePolicy_StrictRejectsNonTemplateDeclarations(t *testing.T) {
	t.Parallel()

	assertPanics(t, "hal: RegisterCurie:", func() {
		New(WithStrictCuries()).RegisterCurie("acme", nonTemplateCurie)
	})
	assertPanics(t, "must be exactly one {rel} or {+rel} expression", func() {
		New(WithStrictCuries()).RegisterCurie("acme", nonTemplateCurie)
	})
	assertPanics(t, "hal: RegisterCurie:", func() {
		New(WithStrictCuries()).RegisterCurie("acme", wrongExpression)
	})

	env := New(WithStrictCuries()).WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "curies", Name: "acme", Href: nonTemplateCurie})
	out, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidCurie) {
		t.Fatalf("error = %v (output %s), want ErrInvalidCurie", err, out)
	}
	if !strings.Contains(err.Error(), "must be exactly one {rel} or {+rel} expression") {
		t.Fatalf("error = %q, want the strict CURIE template message", err)
	}
	if len(out) != 0 {
		t.Fatalf("error returned partial output: %s", out)
	}

	assertPanics(t, "hal: RegisterStatic:", func() {
		RegisterStatic(New(WithStrictCuries()), &embeddedTestUser{}, []Link{
			{Rel: "curies", Name: "acme", Href: nonTemplateCurie},
		})
	})

	invalid := []byte(`{"curies":[{"name":"acme","href":"` + nonTemplateCurie + `"}]}`)
	if _, err := CompilePrecomputedLinksStrict(invalid); !errors.Is(err, ErrInvalidCurie) {
		t.Fatalf("strict precomputed error = %v, want ErrInvalidCurie", err)
	} else if !strings.Contains(err.Error(), "must be exactly one {rel} or {+rel} expression") {
		t.Fatalf("strict precomputed error = %q, want the template message", err)
	}
}
