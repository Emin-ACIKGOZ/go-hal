// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

// Package uritemplate parses RFC 6570 URI Templates into a small AST. It is
// shared by link validation and CURIE validation; it does not expand
// templates.
package uritemplate

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrInvalidTemplate identifies malformed URI Template syntax.
var ErrInvalidTemplate = errors.New("invalid URI template")

// Operator is an RFC 6570 expression operator. The zero value is the simple
// string expansion operator. The constants below cover supported operators and
// legacy/reserved characters that parse but must be rejected by strict callers.
type Operator byte

// Operator constants.
const (
	OpSimple      Operator = 0
	OpReserved    Operator = '+'
	OpFragment    Operator = '#'
	OpLabel       Operator = '.'
	OpPath        Operator = '/'
	OpPathParam   Operator = ';'
	OpQuery       Operator = '?'
	OpQueryCont   Operator = '&'
	OpLegacyEq    Operator = '='
	OpLegacyComma Operator = ','
	OpLegacyBang  Operator = '!'
	OpLegacyAt    Operator = '@'
	OpLegacyPipe  Operator = '|'
)

// Supported reports whether the operator is part of the RFC 6570 set this
// package supports. Reserved/legacy operators parse but are rejected by strict
// link and CURIE contexts.
func (o Operator) Supported() bool {
	switch o {
	case OpSimple, OpReserved, OpFragment, OpLabel, OpPath, OpPathParam, OpQuery, OpQueryCont:
		return true
	default:
		return false
	}
}

// Var is one variable specification inside an expression.
type Var struct {
	Name      string
	MaxLength string // digits after ':', empty when absent
	Explode   bool   // trailing '*'
}

// Expression is one {...} URI Template expression.
type Expression struct {
	Operator Operator
	Vars     []Var
}

// part is either a literal or an expression.
type part struct {
	Literal    string
	Expression *Expression
}

// Template is a parsed URI Template.
type Template struct {
	parts []part
}

// Parse parses raw. A string without expressions is a valid literal template.
//
//nolint:cyclop // Single-pass parser; splitting it obscures the grammar.
func Parse(raw string) (*Template, error) {
	if !utf8.ValidString(raw) {
		return nil, fmt.Errorf("%w: invalid UTF-8", ErrInvalidTemplate)
	}
	t := &Template{}
	literalStart := 0
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			if i > literalStart {
				if err := validateLiteral(raw[literalStart:i]); err != nil {
					return nil, err
				}
				t.parts = append(t.parts, part{Literal: raw[literalStart:i]})
			}
			end := strings.IndexByte(raw[i+1:], '}')
			if end < 0 {
				return nil, fmt.Errorf("%w: unclosed expression", ErrInvalidTemplate)
			}
			end += i + 1
			expr, err := parseExpression(raw[i+1 : end])
			if err != nil {
				return nil, err
			}
			t.parts = append(t.parts, part{Expression: expr})
			i = end
			literalStart = end + 1
		case '}':
			return nil, fmt.Errorf("%w: unmatched closing brace", ErrInvalidTemplate)
		}
	}
	if literalStart < len(raw) {
		if err := validateLiteral(raw[literalStart:]); err != nil {
			return nil, err
		}
		t.parts = append(t.parts, part{Literal: raw[literalStart:]})
	}
	return t, nil
}

func validateLiteral(lit string) error {
	for i := 0; i < len(lit); i++ {
		c := lit[i]
		if c <= 0x20 || c == 0x7f {
			return fmt.Errorf("%w: literal contains a control or space", ErrInvalidTemplate)
		}
		switch c {
		case '"', '\\', '<', '>', '^', '`', '|':
			return fmt.Errorf("%w: literal contains unsafe byte %q", ErrInvalidTemplate, c)
		}
		if c == '%' {
			if i+2 >= len(lit) || !isHex(lit[i+1]) || !isHex(lit[i+2]) {
				return fmt.Errorf("%w: literal has a malformed percent escape", ErrInvalidTemplate)
			}
			i += 2
		}
	}
	return nil
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func parseExpression(body string) (*Expression, error) {
	if body == "" {
		return nil, fmt.Errorf("%w: empty expression", ErrInvalidTemplate)
	}
	expr := &Expression{Operator: OpSimple}
	if isOperator(body[0]) {
		expr.Operator = Operator(body[0])
		body = body[1:]
	}
	if body == "" {
		return nil, fmt.Errorf("%w: expression has no variables", ErrInvalidTemplate)
	}
	for _, spec := range strings.Split(body, ",") {
		v, err := parseVarSpec(spec)
		if err != nil {
			return nil, err
		}
		expr.Vars = append(expr.Vars, v)
	}
	return expr, nil
}

func isOperator(c byte) bool {
	switch Operator(c) {
	case OpReserved, OpFragment, OpLabel, OpPath, OpPathParam, OpQuery, OpQueryCont,
		OpLegacyEq, OpLegacyComma, OpLegacyBang, OpLegacyAt, OpLegacyPipe:
		return true
	default:
		return false
	}
}

func parseVarSpec(spec string) (Var, error) {
	if spec == "" {
		return Var{}, fmt.Errorf("%w: empty variable specification", ErrInvalidTemplate)
	}
	var v Var
	if strings.HasSuffix(spec, "*") {
		v.Explode = true
		spec = spec[:len(spec)-1]
	}
	if idx := strings.IndexByte(spec, ':'); idx >= 0 {
		if v.Explode {
			return Var{}, fmt.Errorf("%w: prefix and explode are mutually exclusive", ErrInvalidTemplate)
		}
		v.MaxLength = spec[idx+1:]
		spec = spec[:idx]
		if !validMaxLength(v.MaxLength) {
			return Var{}, fmt.Errorf("%w: invalid prefix length %q", ErrInvalidTemplate, v.MaxLength)
		}
	}
	if err := validateVarName(spec); err != nil {
		return Var{}, err
	}
	v.Name = spec
	return v, nil
}

func validMaxLength(s string) bool {
	if len(s) == 0 || len(s) > 4 || s[0] == '0' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

//nolint:cyclop // Character-class validation stays in one auditable loop.
func validateVarName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty variable name", ErrInvalidTemplate)
	}
	if name[0] == '.' || name[len(name)-1] == '.' || strings.Contains(name, "..") {
		return fmt.Errorf("%w: invalid variable name %q", ErrInvalidTemplate, name)
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '_', c == '.':
		case c == '%':
			if i+2 >= len(name) || !isHex(name[i+1]) || !isHex(name[i+2]) {
				return fmt.Errorf("%w: malformed variable escape", ErrInvalidTemplate)
			}
			i += 2
		default:
			return fmt.Errorf("%w: invalid variable name %q", ErrInvalidTemplate, name)
		}
	}
	return nil
}

// Expressions returns the expressions in source order.
func (t *Template) Expressions() []Expression {
	var exprs []Expression
	for _, part := range t.parts {
		if part.Expression != nil {
			exprs = append(exprs, *part.Expression)
		}
	}
	return exprs
}

// HasExpressions reports whether the template contains an expression.
func (t *Template) HasExpressions() bool {
	for _, part := range t.parts {
		if part.Expression != nil {
			return true
		}
	}
	return false
}

// HasRelExpression reports whether any expression contains a variable named
// "rel".
func (t *Template) HasRelExpression() bool {
	for _, expr := range t.Expressions() {
		for _, v := range expr.Vars {
			if v.Name == "rel" {
				return true
			}
		}
	}
	return false
}

// HasUnsupportedOperator reports whether any expression uses a reserved or
// unknown operator.
func (t *Template) HasUnsupportedOperator() bool {
	for _, expr := range t.Expressions() {
		if !expr.Operator.Supported() {
			return true
		}
	}
	return false
}

// SingleRelExpression reports whether the template consists of exactly one
// expression with operator op, a single variable named "rel", and no prefix or
// explode modifier. Literal-only templates return false.
func (t *Template) SingleRelExpression(op Operator) bool {
	exprs := t.Expressions()
	if len(exprs) != 1 {
		return false
	}
	expr := exprs[0]
	if expr.Operator != op || len(expr.Vars) != 1 {
		return false
	}
	v := expr.Vars[0]
	return v.Name == "rel" && v.MaxLength == "" && !v.Explode
}
