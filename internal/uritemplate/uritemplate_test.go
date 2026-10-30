// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package uritemplate

import (
	"errors"
	"testing"
)

func TestParseRFC6570Examples(t *testing.T) {
	t.Parallel()
	valid := []string{
		"/orders{?id}",
		"https://docs.example.com/relations/{rel}",
		"https://docs.example.com/relations/{+rel}",
		"/users/{id}/orders{/orderId}{?fields*}",
		"{scheme}://{host}/x",
		"{.format}",
		"{;x,y}",
		"{&x,y}",
		"{#frag}",
		"{x:3}",
		"{x*}",
		"literal-only",
		"%7Brel%7D",
	}
	for _, raw := range valid {
		if _, err := Parse(raw); err != nil {
			t.Errorf("Parse(%q) = %v, want nil", raw, err)
		}
	}
}

func TestParseRejectsMalformedTemplates(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"{",
		"}",
		"{}",
		"{x",
		"{x,}",
		"{x:}",
		"{x:0}",
		"{x:12345}",
		"{x**}",
		"{x:3*}",
		"{.x.}",
		"{x..y}",
		"{ x}",
		"literal with space",
		"%zz",
	}
	for _, raw := range invalid {
		if _, err := Parse(raw); !errors.Is(err, ErrInvalidTemplate) {
			t.Errorf("Parse(%q) = %v, want ErrInvalidTemplate", raw, err)
		}
	}
}

func TestPercentEncodedBracesAreData(t *testing.T) {
	t.Parallel()
	tmpl, err := Parse("%7Brel%7D")
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.HasExpressions() {
		t.Fatal("%7B...%7D was parsed as an expression")
	}
	if tmpl.HasRelExpression() {
		t.Fatal("%7B...%7D produced a rel expression")
	}
}

func TestRelExpressions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw      string
		wantRel  bool
		wantOne  bool
		operator Operator
	}{
		{"{rel}", true, true, OpSimple},
		{"{+rel}", true, true, OpReserved},
		{"https://docs/{rel}", true, true, OpSimple},
		{"{?rel}", true, false, OpSimple},
		{"{Rel}", false, false, OpSimple},
		{"{other}", false, false, OpSimple},
		{"literal", false, false, OpSimple},
	}
	for _, tc := range tests {
		tmpl, err := Parse(tc.raw)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.raw, err)
		}
		if got := tmpl.HasRelExpression(); got != tc.wantRel {
			t.Errorf("Parse(%q).HasRelExpression() = %t, want %t", tc.raw, got, tc.wantRel)
		}
		if got := tmpl.SingleRelExpression(tc.operator); got != tc.wantOne {
			t.Errorf("Parse(%q).SingleRelExpression(%q) = %t, want %t", tc.raw, tc.operator, got, tc.wantOne)
		}
	}
}

func TestReservedOperatorsParseButAreUnsupported(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"{=x}", "{,x}", "{!x}", "{@x}", "{|x}"} {
		tmpl, err := Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q): %v", raw, err)
		}
		if !tmpl.HasUnsupportedOperator() {
			t.Errorf("Parse(%q) did not report an unsupported operator", raw)
		}
	}
}

func TestSupportedOperator(t *testing.T) {
	t.Parallel()
	for _, op := range []Operator{OpSimple, OpReserved, OpFragment, OpLabel, OpPath, OpPathParam, OpQuery, OpQueryCont} {
		if !op.Supported() {
			t.Errorf("operator %q reported unsupported", op)
		}
	}
	for _, op := range []Operator{OpLegacyEq, OpLegacyComma, OpLegacyBang, OpLegacyAt, OpLegacyPipe} {
		if op.Supported() {
			t.Errorf("operator %q reported supported", op)
		}
	}
}
