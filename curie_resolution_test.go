// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

type curieResolutionUser struct {
	ID int `json:"id"`
}

func TestCurieResolution_StaticResolvesAtWrap(t *testing.T) {
	inst := New()
	RegisterStatic(inst, &curieResolutionUser{}, []Link{
		{Rel: "acme:widget", Href: "/widgets/1"},
	})
	inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")

	b, err := json.Marshal(inst.Wrap(context.Background(), &curieResolutionUser{ID: 1}))
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("document is not an object")
	}
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 || curies[0]["name"] != "acme" {
		t.Fatalf("curies = %#v, want acme declaration", curies)
	}
}

func TestCurieResolution_LateRegistrationAffectsNextWrap(t *testing.T) {
	links, err := CompilePrecomputedLinks([]byte(`{"acme:widget":{"href":"/widgets/1"}}`))
	if err != nil {
		t.Fatal(err)
	}
	inst := New()

	before, err := json.Marshal(inst.WrapPrecomputedLinks(&curieResolutionUser{ID: 1}, links))
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, before)
	if relationExists(t, before, "curies") {
		t.Fatal("unregistered prefix produced a curies declaration")
	}

	inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")
	after, err := json.Marshal(inst.WrapPrecomputedLinks(&curieResolutionUser{ID: 1}, links))
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, after)
	if !relationExists(t, after, "curies") {
		t.Fatal("late registration did not produce a curies declaration")
	}
}

func TestCurieResolution_HandleReusedAcrossInstances(t *testing.T) {
	links, err := CompilePrecomputedLinks([]byte(`{"acme:widget":{"href":"/widgets/1"}}`))
	if err != nil {
		t.Fatal(err)
	}

	instA := New()
	instA.RegisterCurie("acme", "https://a.example/{rel}")
	instB := New()
	instB.RegisterCurie("acme", "https://b.example/{rel}")

	for _, tc := range []struct {
		name string
		inst *Instance
		want string
	}{
		{name: "first", inst: instA, want: "https://a.example/{rel}"},
		{name: "second", inst: instB, want: "https://b.example/{rel}"},
		{name: "first again", inst: instA, want: "https://a.example/{rel}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.inst.WrapPrecomputedLinks(&curieResolutionUser{ID: 1}, links))
			if err != nil {
				t.Fatal(err)
			}
			root, ok := decodeJSONValue(t, b).(map[string]any)
			if !ok {
				t.Fatal("document is not an object")
			}
			curies := relationObjects(t, root, "curies")
			if len(curies) != 1 || curies[0]["href"] != tc.want {
				t.Fatalf("curies = %#v, want href %q", curies, tc.want)
			}
		})
	}
}

func TestCurieResolution_PrecomputedUserDeclarationWins(t *testing.T) {
	links, err := CompilePrecomputedLinks([]byte(`{
		"curies":[{"name":"acme","href":"https://user.example/{rel}","templated":true}],
		"acme:widget":{"href":"/widgets/1"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	inst := New()
	inst.RegisterCurie("acme", "https://registered.example/{rel}")

	b, err := json.Marshal(inst.WrapPrecomputedLinks(&curieResolutionUser{ID: 1}, links))
	if err != nil {
		t.Fatal(err)
	}
	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("document is not an object")
	}
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 {
		t.Fatalf("got %d curies, want 1", len(curies))
	}
	if got := curies[0]["href"]; got != "https://user.example/{rel}" {
		t.Fatalf("curies[0].href = %#v, want user declaration", got)
	}
}

func TestCurieResolution_PreservesUnknownUserFields(t *testing.T) {
	links, err := CompilePrecomputedLinks([]byte(`{
		"curies":[{"name":"user","href":"https://user.example/{rel}","x-ext":"keep"}],
		"acme:widget":{"href":"/widgets/1"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	inst := New()
	inst.RegisterCurie("acme", "https://registered.example/{rel}")

	b, err := json.Marshal(inst.WrapPrecomputedLinks(&curieResolutionUser{ID: 1}, links))
	if err != nil {
		t.Fatal(err)
	}
	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("document is not an object")
	}
	curies := relationObjects(t, root, "curies")
	if len(curies) != 2 {
		t.Fatalf("got %d curies, want 2", len(curies))
	}
	user := curies[0]
	if user["name"] != "user" || user["x-ext"] != "keep" {
		t.Fatalf("user declaration = %#v, want unknown field preserved", user)
	}
}

func relationExists(t *testing.T, data []byte, rel string) bool {
	t.Helper()
	root, ok := decodeJSONValue(t, data).(map[string]any)
	if !ok {
		t.Fatal("document is not an object")
	}
	links, ok := root["_links"].(map[string]any)
	if !ok {
		return false
	}
	_, exists := links[rel]
	return exists
}
