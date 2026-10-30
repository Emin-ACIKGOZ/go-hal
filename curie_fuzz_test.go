// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/Emin-ACIKGOZ/go-hal/internal/uritemplate"
)

func FuzzCurieDeclaration(f *testing.F) {
	f.Add("acme", "https://docs.example/rels/{rel}", true)
	f.Add("1acme", "https://docs.example/rels/{+rel}", true)
	f.Add("ac:me", "https://docs.example/rels/{rel}", true)
	f.Add("acme", "", false)
	f.Add("", "", false)
	f.Add("acme", "https://docs.example/%7Brel%7D", false)

	f.Fuzz(func(t *testing.T, name, href string, templated bool) {
		if !utf8.ValidString(name) || !utf8.ValidString(href) {
			t.Skip()
		}
		env := New().WrapRaw(&embeddedTestUser{ID: 1})
		env.AddLink(Link{Rel: "curies", Name: name, Href: href, Templated: templated})

		out, err := json.Marshal(env)
		if err != nil {
			if len(out) != 0 {
				t.Fatalf("error returned partial output: %s", out)
			}
			return
		}
		if !json.Valid(out) || !utf8.Valid(out) {
			t.Fatalf("invalid JSON output: %s", out)
		}
		assertNoDuplicateKeys(t, out)
	})
}

func FuzzStrictCurieDeclaration(f *testing.F) {
	f.Add("acme", "https://docs.example/rels/{rel}", true)
	f.Add("acme", "https://docs.example/rels/{+rel}", true)
	f.Add("1acme", "https://docs.example/rels/{rel}", true)
	f.Add("acme", "https://docs.example/rels/{?rel}", true)
	f.Add("acme", "https://docs.example/rels/{rel}", false)
	f.Add("", "", false)

	f.Fuzz(func(t *testing.T, name, href string, templated bool) {
		if !utf8.ValidString(name) || !utf8.ValidString(href) {
			t.Skip()
		}
		env := New(WithStrictCuries()).WrapRaw(&embeddedTestUser{ID: 1})
		env.AddLink(Link{Rel: "curies", Name: name, Href: href, Templated: templated})

		out, err := json.Marshal(env)
		if err != nil {
			if len(out) != 0 {
				t.Fatalf("error returned partial output: %s", out)
			}
			return
		}
		if !json.Valid(out) || !utf8.Valid(out) {
			t.Fatalf("valid strict CURIE produced invalid JSON: %s", out)
		}
	})
}

func FuzzCurieRelTemplate(f *testing.F) {
	f.Add("https://docs.example/rels/{rel}")
	f.Add("https://docs.example/rels/{+rel}")
	f.Add("https://docs.example/rels/{?rel}")
	f.Add("{rel}")
	f.Add("%7Brel%7D")
	f.Add("{")

	f.Fuzz(func(t *testing.T, href string) {
		if !utf8.ValidString(href) {
			t.Skip()
		}
		err := validateCurieDeclarations([]Link{{
			Name:      "acme",
			Href:      href,
			Templated: true,
		}}, true)
		if err != nil {
			return
		}
		tmpl, parseErr := uritemplate.Parse(href)
		if parseErr != nil ||
			(!tmpl.SingleRelExpression(uritemplate.OpSimple) && !tmpl.SingleRelExpression(uritemplate.OpReserved)) {
			t.Fatalf("accepted href %q without a single rel expression", href)
		}
	})
}
