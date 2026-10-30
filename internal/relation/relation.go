// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

// Package relation contains the single relation classifier shared by link,
// embedded-resource, and CURIE handling.
package relation

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kind is the lexical classification of a HAL link relation.
type Kind uint8

const (
	// Invalid is an empty, non-UTF-8, control-bearing, or whitespace-bearing
	// relation.
	Invalid Kind = iota
	// Token is a bare link relation token such as "self".
	Token
	// Curie is a compact URI such as "acme:widget".
	Curie
	// URI is an absolute URI or a URI reference such as "https://example.com".
	URI
)

func (k Kind) String() string {
	switch k {
	case Token:
		return "token"
	case Curie:
		return "curie"
	case URI:
		return "uri"
	default:
		return "invalid"
	}
}

// HasScheme reports whether ref begins with an RFC 3986 scheme followed by a
// colon. The check is lexical; it does not require "//" after the colon.
func HasScheme(ref string) bool {
	if ref == "" || !isSchemeStart(ref[0]) {
		return false
	}
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		if c == ':' {
			return i > 0
		}
		if !isSchemeByte(c) {
			return false
		}
	}
	return false
}

func isSchemeByte(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z':
		return true
	case c >= 'a' && c <= 'z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '+', c == '-', c == '.':
		return true
	default:
		return false
	}
}

func isSchemeStart(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

// Scheme returns the RFC 3986 scheme of ref, if any.
func Scheme(ref string) (string, bool) {
	if !HasScheme(ref) {
		return "", false
	}
	idx := strings.IndexByte(ref, ':')
	if idx <= 0 || !isSchemeStart(ref[0]) {
		return "", false
	}
	return ref[:idx], true
}

// SplitCurie splits "prefix:reference" when prefix is a plausible CURIE name
// and reference is non-empty. It deliberately rejects authority-like forms
// ("https://example.com") and relative references ("/a:b", "a/b:c").
func SplitCurie(ref string) (prefix, reference string, ok bool) {
	idx := strings.IndexByte(ref, ':')
	if idx <= 0 || idx == len(ref)-1 {
		return "", "", false
	}
	prefix = ref[:idx]
	reference = ref[idx+1:]
	if strings.HasPrefix(reference, "//") {
		return "", "", false
	}
	if !isCuriePrefix(prefix) {
		return "", "", false
	}
	return prefix, reference, true
}

//nolint:cyclop // Flat byte scanner is easier to audit than a table.
func isCuriePrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := prefix[i]
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '_', c == '.', c == '-':
		default:
			return false
		}
	}
	return true
}

// Classify returns the lexical relation kind. Callers that know their declared
// CURIE prefixes should treat non-authority scheme forms as opaque URIs unless
// the prefix is declared.
//
//nolint:cyclop // Fast ASCII path plus the full Unicode path in one classifier.
func Classify(ref string) Kind {
	if ref == "" {
		return Invalid
	}
	colon := -1
	cleanASCII := true
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		if c <= 0x20 || c >= 0x7f {
			cleanASCII = false
			break
		}
		if c == ':' && colon < 0 {
			colon = i
		}
	}
	if cleanASCII {
		if colon < 0 {
			return Token
		}
		return classifyWithColon(ref)
	}
	if !utf8.ValidString(ref) || hasControlOrSpace(ref) {
		return Invalid
	}
	return classifyWithColon(ref)
}

func classifyWithColon(ref string) Kind {
	if _, ok := Scheme(ref); ok {
		if _, _, curie := SplitCurie(ref); curie {
			return Curie
		}
		return URI
	}
	if _, _, ok := SplitCurie(ref); ok {
		return Curie
	}
	if strings.ContainsRune(ref, ':') {
		return Invalid
	}
	return Token
}

func hasControlOrSpace(ref string) bool {
	for _, r := range ref {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

// IsToken reports whether ref is a strict, lowercase registered relation token.
// Extension relation types (URIs and declared CURIEs) are not tokens.
//
//nolint:cyclop // Flat byte scanner is easier to audit than a table.
func IsToken(ref string) bool {
	if ref == "" {
		return false
	}
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		if i == 0 {
			if c < 'a' || c > 'z' {
				return false
			}
			continue
		}
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '.', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// FoldBareToken validates bare-token syntax and returns its ASCII folded key in
// one pass. It is used for case-collision detection; relative references and
// extension relations are not bare tokens.
//
//nolint:cyclop // Single-pass token validation and ASCII folding.
func FoldBareToken(ref string) (string, bool) {
	if ref == "" {
		return "", false
	}
	needsFold := false
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		if i == 0 {
			if !isASCIIAlpha(c) {
				return "", false
			}
		} else if !isASCIIAlpha(c) && (c < '0' || c > '9') && c != '.' && c != '-' && c != '_' {
			return "", false
		}
		if c >= 'A' && c <= 'Z' {
			needsFold = true
		}
	}
	if !needsFold {
		return ref, true
	}
	buf := []byte(ref)
	for i, c := range buf {
		if c >= 'A' && c <= 'Z' {
			buf[i] = c + ('a' - 'A')
		}
	}
	return string(buf), true
}

func isASCIIAlpha(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

// FoldToken returns the ASCII-only case fold used to detect bare-token
// collisions. Non-ASCII bytes are left untouched.
func FoldToken(ref string) string {
	needsFold := false
	for i := 0; i < len(ref); i++ {
		if ref[i] >= 'A' && ref[i] <= 'Z' {
			needsFold = true
			break
		}
	}
	if !needsFold {
		return ref
	}
	buf := []byte(ref)
	for i, c := range buf {
		if c >= 'A' && c <= 'Z' {
			buf[i] = c + ('a' - 'A')
		}
	}
	return string(buf)
}
