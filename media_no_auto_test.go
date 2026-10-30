// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMedia_NoAutomaticSelfOrProfileLinks(t *testing.T) {
	t.Parallel()

	inst := New()
	for _, env := range []*Envelope{
		inst.WrapRaw(&embeddedTestUser{ID: 1}),
		inst.Wrap(context.Background(), &embeddedTestUser{ID: 1}),
	} {
		b, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		root := decodeJSONValue(t, b).(map[string]any)
		if _, exists := root["_links"]; exists {
			t.Fatalf("serializer added links automatically: %s", b)
		}
	}
}

func TestMedia_ExplicitSelfAndProfileLinks(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: "/users/1"})
	env.AddLink(Link{Rel: "profile", Href: "https://example.com/profiles/user"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	self := relationObjects(t, root, "self")
	if len(self) != 1 || self[0]["href"] != "/users/1" {
		t.Fatalf("self = %#v", self)
	}
	profile := relationObjects(t, root, "profile")
	if len(profile) != 1 || profile[0]["href"] != "https://example.com/profiles/user" {
		t.Fatalf("profile = %#v", profile)
	}
}

func TestMedia_ContentTypeDoesNotAffectSerialization(t *testing.T) {
	t.Parallel()

	if _, err := ContentType("https://example.com/profiles/order"); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(New().WrapRaw(&embeddedTestUser{ID: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := decodeJSONValue(t, b).(map[string]any)["_links"]; exists {
		t.Fatalf("ContentType changed envelope output: %s", b)
	}
}
