// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

type StaticUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestRegisterStatic_Basic(t *testing.T) {
	inst := New()

	RegisterStatic(inst, &StaticUser{}, []Link{
		{Rel: "self", Href: "/users/42"},
	})

	env := inst.Wrap(context.Background(), &StaticUser{ID: 42, Name: "Alice"})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(b, `"id":42`) {
		t.Fatalf("expected id in output: %s", b)
	}
	if !contains(b, `"_links"`) {
		t.Fatalf("expected _links in output: %s", b)
	}
	if !contains(b, `"self"`) {
		t.Fatalf("expected self link in output: %s", b)
	}
}

func TestRegisterStatic_MultipleLinks(t *testing.T) {
	inst := New()

	RegisterStatic(inst, &StaticUser{}, []Link{
		{Rel: "self", Href: "/users/1"},
		{Rel: "edit", Href: "/users/1"},
		{Rel: "delete", Href: "/users/1"},
	})

	env := inst.Wrap(context.Background(), &StaticUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(b, `"self"`) || !contains(b, `"edit"`) || !contains(b, `"delete"`) {
		t.Fatalf("expected all links in output: %s", b)
	}
}

func TestRegisterStatic_DifferentTypes(t *testing.T) {
	inst := New()

	type User1 struct {
		ID int `json:"id"`
	}
	type User2 struct {
		ID int `json:"id"`
	}

	RegisterStatic(inst, &User1{}, []Link{{Rel: "self", Href: "/users1"}})
	RegisterStatic(inst, &User2{}, []Link{{Rel: "self", Href: "/users2"}})

	env1 := inst.Wrap(context.Background(), &User1{ID: 1})
	env2 := inst.Wrap(context.Background(), &User2{ID: 2})

	b1, _ := json.Marshal(env1)
	b2, _ := json.Marshal(env2)

	t.Logf("env1: %s", b1)
	t.Logf("env2: %s", b2)

	if !contains(b1, "/users1") {
		t.Fatalf("expected users1 in env1: %s", b1)
	}
	if !contains(b2, "/users2") {
		t.Fatalf("expected users2 in env2: %s", b2)
	}
}
