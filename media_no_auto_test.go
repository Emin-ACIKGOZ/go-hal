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

func TestMedia_MultipleProfileLinksMarshalAsArray(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "profile", Href: "https://example.com/profiles/order"})
	env.AddLink(Link{Rel: "profile", Href: "https://example.com/profiles/order-v2"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	profiles := relationObjects(t, root, "profile")
	if len(profiles) != 2 {
		t.Fatalf("profile = %#v, want 2 links", profiles)
	}
	if profiles[0]["href"] != "https://example.com/profiles/order" ||
		profiles[1]["href"] != "https://example.com/profiles/order-v2" {
		t.Fatalf("profile order changed: %#v", profiles)
	}
}

func TestMedia_CollectionEmitsSelfButNoProfile(t *testing.T) {
	t.Parallel()

	page := New().Collection(
		context.Background(),
		[]*embeddedTestUser{{ID: 1}},
		1,
		Link{Rel: "self", Href: "/users"},
	)
	b, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if len(relationObjects(t, root, "self")) != 1 {
		t.Fatalf("collection self missing: %s", b)
	}
	if _, exists := root["_links"].(map[string]any)["profile"]; exists {
		t.Fatalf("collection invented a profile link: %s", b)
	}
}

func TestMedia_EmbeddedResourcesDoNotInheritRootProfile(t *testing.T) {
	t.Parallel()

	parent := New().WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddLink(Link{Rel: "profile", Href: "https://example.com/profiles/order"})
	parent.AddEmbedded("child", New().WrapRaw(&embeddedTestUser{ID: 2}))

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if len(relationObjects(t, root, "profile")) != 1 {
		t.Fatalf("root profile missing: %s", b)
	}
	childRoot := embeddedObject(t, root, "child")
	if _, exists := childRoot["_links"]; exists {
		t.Fatalf("embedded resource inherited the root profile: %s", b)
	}
}

func TestMedia_RepeatedMarshalIsByteIdentical(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: "/users/1"})
	env.AddLink(Link{Rel: "profile", Href: "https://example.com/profiles/user"})

	first, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("repeated marshal changed output:\n%s\n%s", first, second)
	}
}

func TestMedia_StaticAndPrecomputedGainNoLinks(t *testing.T) {
	t.Parallel()

	staticInst := New()
	RegisterStatic(staticInst, &embeddedTestUser{}, nil)
	staticOut, err := json.Marshal(staticInst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := decodeJSONValue(t, staticOut).(map[string]any)["_links"]; exists {
		t.Fatalf("static envelope gained links: %s", staticOut)
	}

	precomputedOut, err := json.Marshal(New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, &PrecomputedLinks{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := decodeJSONValue(t, precomputedOut).(map[string]any)["_links"]; exists {
		t.Fatalf("precomputed envelope gained links: %s", precomputedOut)
	}
}
