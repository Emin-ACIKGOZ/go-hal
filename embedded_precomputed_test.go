// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestAddEmbedded_StaticEnvelope(t *testing.T) {
	t.Parallel()

	inst := New()
	RegisterStatic(inst, &embeddedTestUser{}, []Link{{Rel: "self", Href: "/static"}})

	env := inst.Wrap(context.Background(), &embeddedTestUser{ID: 1})
	env.AddEmbedded("author", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root := decodeJSONValue(t, b).(map[string]any)
	if self := relationObjects(t, root, "self"); len(self) != 1 || self[0]["href"] != "/static" {
		t.Fatalf("self = %#v, want /static", self)
	}
	author := root["_embedded"].(map[string]any)["author"].(map[string]any)
	if got := author["id"]; got == nil {
		t.Fatal("embedded author lost state")
	}
}

func TestAddEmbedded_PrecomputedEnvelope(t *testing.T) {
	t.Parallel()

	links, err := CompilePrecomputedLinks([]byte(`{"self":{"href":"/compiled","x-vendor":"kept"}}`))
	if err != nil {
		t.Fatal(err)
	}
	env := New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, links)
	env.AddEmbedded("author", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)
	if !strings.Contains(string(b), `"x-vendor":"kept"`) {
		t.Fatalf("precomputed link fields were rewritten: %s", b)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, ok := root["_embedded"].(map[string]any); !ok {
		t.Fatalf("_embedded missing from %s", b)
	}
}

func TestAddEmbedded_ZeroPrecomputedHandle(t *testing.T) {
	t.Parallel()

	env := New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, &PrecomputedLinks{})
	env.AddEmbedded("author", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, exists := root["_links"]; exists {
		t.Fatalf("zero handle emitted _links: %s", b)
	}
	if _, ok := root["_embedded"].(map[string]any); !ok {
		t.Fatalf("_embedded missing from %s", b)
	}
}

func TestAddEmbedded_NilPrecomputedHandleUsesRawSemantics(t *testing.T) {
	t.Parallel()

	env := New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, nil)
	env.AddLink(Link{Rel: "self", Href: "/raw"})
	env.AddEmbedded("author", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if self := relationObjects(t, root, "self"); len(self) != 1 || self[0]["href"] != "/raw" {
		t.Fatalf("self = %#v, want /raw", self)
	}
	if _, ok := root["_embedded"].(map[string]any); !ok {
		t.Fatalf("_embedded missing from %s", b)
	}
}

func TestAddEmbedded_StaticEmptyLinks(t *testing.T) {
	t.Parallel()

	inst := New()
	RegisterStatic(inst, &embeddedTestUser{}, nil)
	env := inst.Wrap(context.Background(), &embeddedTestUser{ID: 1})
	env.AddEmbedded("author", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, exists := root["_links"]; exists {
		t.Fatalf("empty static links emitted _links: %s", b)
	}
	if _, ok := root["_embedded"].(map[string]any); !ok {
		t.Fatalf("_embedded missing from %s", b)
	}
}

func TestAddEmbedded_PrecomputedNullData(t *testing.T) {
	t.Parallel()

	links, err := CompilePrecomputedLinks([]byte(`{"self":{"href":"/x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	env := New().WrapPrecomputedLinks(nil, links)
	env.AddEmbedded("author", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, b, []byte(`{
		"_links":{"self":{"href":"/x"}},
		"_embedded":{"author":{"id":2}}
	}`))
}

func TestAddEmbedded_PrecomputedStillRejectsAddLink(t *testing.T) {
	t.Parallel()

	env := New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, &PrecomputedLinks{})
	env.AddLink(Link{Rel: "self", Href: "/x"})
	env.AddEmbedded("author", &embeddedTestUser{ID: 2})

	_, err := json.Marshal(env)
	if !errors.Is(err, ErrAddLinkPrecomputed) {
		t.Fatalf("error = %v, want ErrAddLinkPrecomputed", err)
	}
}
