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

	assertJSONEqual(t, b, []byte(`{
		"id":42,
		"name":"Alice",
		"_links":{"self":{"href":"/users/42"}}
	}`))
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

	assertJSONEqual(t, b, []byte(`{
		"id":1,
		"name":"",
		"_links":{
			"self":{"href":"/users/1"},
			"edit":{"href":"/users/1"},
			"delete":{"href":"/users/1"}
		}
	}`))
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

	b1, err := json.Marshal(env1)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := json.Marshal(env2)
	if err != nil {
		t.Fatal(err)
	}

	assertJSONEqual(t, b1, []byte(`{"id":1,"_links":{"self":{"href":"/users1"}}}`))
	assertJSONEqual(t, b2, []byte(`{"id":2,"_links":{"self":{"href":"/users2"}}}`))
}
