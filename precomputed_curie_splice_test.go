// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPrecomputedCuries_PreservesUnknownLinkBytes(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example/{rel}")
	handle, err := CompilePrecomputedLinks([]byte(`{
		"self":{"href":"/x","x-vendor":"kept"},
		"x-extra":{"href":"/e"},
		"acme:widget":{"href":"/w"}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(inst.WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle))
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)
	if !bytes.Contains(b, []byte(`"x-vendor":"kept"`)) {
		t.Fatalf("unknown link field bytes changed: %s", b)
	}
	if !bytes.Contains(b, []byte(`"x-extra":{"href":"/e"}`)) {
		t.Fatalf("unknown relation bytes changed: %s", b)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["name"] != "acme" {
		t.Fatalf("curies = %#v, want acme declaration", curies)
	}
}

func TestPrecomputedCuries_UserAndRegisteredDeclarations(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://registered.example/{rel}")
	handle, err := CompilePrecomputedLinks([]byte(`{
		"curies":[{"name":"user","href":"https://user.example/{rel}","x-ext":"keep"}],
		"acme:widget":{"href":"/w"}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(inst.WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"x-ext":"keep"`)) {
		t.Fatalf("user declaration unknown field was lost: %s", b)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 2 {
		t.Fatalf("got %d curies, want 2: %#v", len(curies), curies)
	}
	if curies[0]["name"] != "user" || curies[1]["name"] != "acme" {
		t.Fatalf("curies = %#v, want user then acme", curies)
	}
}

func TestPrecomputedCuries_UserDeclarationWins(t *testing.T) {
	t.Parallel()

	inst := New()
	inst.RegisterCurie("acme", "https://registered.example/{rel}")
	handle, err := CompilePrecomputedLinks([]byte(`{
		"curies":[{"name":"acme","href":"https://user.example/{rel}"}],
		"acme:widget":{"href":"/w"}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(inst.WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle))
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["href"] != "https://user.example/{rel}" {
		t.Fatalf("curies = %#v, want user href", curies)
	}
}

func TestPrecomputedCuries_NilCachedUserFallsBackToRaw(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"_links":{"curies":[{"name":"user","href":"https://user.example/{rel}"}]}}`)
	registered := []Link{{Rel: "curies", Name: "acme", Href: "https://registered.example/{rel}", Templated: true}}
	merged, err := mergePrecomputedCuries(raw, nil, registered)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(merged) {
		t.Fatalf("invalid JSON: %s", merged)
	}
	if !bytes.Contains(merged, []byte(`"name":"user"`)) || !bytes.Contains(merged, []byte(`"name":"acme"`)) {
		t.Fatalf("merged declarations missing: %s", merged)
	}
}
