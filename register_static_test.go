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

func TestRegisterStatic_DuplicateRelations(t *testing.T) {
	tests := []struct {
		name      string
		links     []Link
		wantArray bool
		wantHrefs []string
	}{
		{
			name:      "single relation remains an object",
			links:     []Link{{Rel: "self", Href: "/users/1"}},
			wantHrefs: []string{"/users/1"},
		},
		{
			name: "two relations become an array",
			links: []Link{
				{Rel: "self", Href: "/users/1"},
				{Rel: "self", Href: "/users/1?page=2"},
			},
			wantArray: true,
			wantHrefs: []string{"/users/1", "/users/1?page=2"},
		},
		{
			name: "three relations preserve order",
			links: []Link{
				{Rel: "self", Href: "/users/1"},
				{Rel: "self", Href: "/users/1?page=2"},
				{Rel: "self", Href: "/users/1?page=3"},
			},
			wantArray: true,
			wantHrefs: []string{"/users/1", "/users/1?page=2", "/users/1?page=3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inst := New()
			RegisterStatic(inst, &StaticUser{}, tc.links)

			env := inst.Wrap(context.Background(), &StaticUser{ID: 1})
			b, err := json.Marshal(env)
			if err != nil {
				t.Fatal(err)
			}
			assertNoDuplicateKeys(t, b)

			root, ok := decodeJSONValue(t, b).(map[string]any)
			if !ok {
				t.Fatal("document is not an object")
			}
			assertLinkRelationShapes(t, root)

			links, ok := root["_links"].(map[string]any)
			if !ok {
				t.Fatalf("_links has type %T, want object", root["_links"])
			}
			_, isArray := links["self"].([]any)
			if isArray != tc.wantArray {
				t.Fatalf("self array form = %t, want %t", isArray, tc.wantArray)
			}

			objects := relationObjects(t, root, "self")
			if len(objects) != len(tc.wantHrefs) {
				t.Fatalf("got %d links, want %d", len(objects), len(tc.wantHrefs))
			}
			for i, wantHref := range tc.wantHrefs {
				if got := objects[i]["href"]; got != wantHref {
					t.Errorf("self[%d].href = %#v, want %q", i, got, wantHref)
				}
			}
		})
	}
}

func TestRegisterStatic_ValidatesInputs(t *testing.T) {
	tests := []struct {
		name   string
		inst   *Instance
		target any
		links  []Link
		want   string
	}{
		{
			name:   "nil instance",
			inst:   nil,
			target: &StaticUser{},
			want:   "instance must not be nil",
		},
		{
			name: "nil target",
			inst: New(),
			want: "target must not be nil",
		},
		{
			name:   "value target",
			inst:   New(),
			target: StaticUser{},
			want:   "target must be a pointer",
		},
		{
			name:   "empty relation",
			inst:   New(),
			target: &StaticUser{},
			links:  []Link{{Href: "/x"}},
			want:   "link relation must not be empty",
		},
		{
			name:   "whitespace relation",
			inst:   New(),
			target: &StaticUser{},
			links:  []Link{{Rel: "  ", Href: "/x"}},
			want:   "link relation must not be empty",
		},
		{
			name:   "curie without name",
			inst:   New(),
			target: &StaticUser{},
			links:  []Link{{Rel: "curies", Href: "https://example.com/{rel}"}},
			want:   "invalid CURIE declaration",
		},
		{
			name:   "curie without href",
			inst:   New(),
			target: &StaticUser{},
			links:  []Link{{Rel: "curies", Name: "acme"}},
			want:   "invalid CURIE declaration",
		},
		{
			name:   "duplicate curie",
			inst:   New(),
			target: &StaticUser{},
			links: []Link{
				{Rel: "curies", Name: "acme", Href: "https://example.com/a/{rel}"},
				{Rel: "curies", Name: "acme", Href: "https://example.com/b/{rel}"},
			},
			want: "duplicate CURIE declaration",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertPanics(t, "hal: RegisterStatic: ", func() {
				RegisterStatic(tc.inst, tc.target, tc.links)
			})
			assertPanics(t, tc.want, func() {
				RegisterStatic(tc.inst, tc.target, tc.links)
			})
		})
	}
}

func TestRegisterStatic_AcceptsTypedNilTarget(t *testing.T) {
	inst := New()
	RegisterStatic(inst, (*StaticUser)(nil), []Link{{Rel: "self", Href: "/users"}})

	env := inst.Wrap(context.Background(), &StaticUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, b, []byte(`{"id":1,"name":"","_links":{"self":{"href":"/users"}}}`))
}

func TestRegisterStatic_EmptyLinksSuppressGenerators(t *testing.T) {
	inst := New()
	RegisterInstance(inst, func(_ context.Context, _ *StaticUser) []Link {
		return []Link{{Rel: "ignored", Href: "/generated"}}
	})
	RegisterStatic(inst, &StaticUser{}, nil)

	env := inst.Wrap(context.Background(), &StaticUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, b, []byte(`{"id":1,"name":""}`))
	assertNoDuplicateKeys(t, b)
}

func TestRegisterStatic_SingleCurieBecomesArray(t *testing.T) {
	inst := New()
	RegisterStatic(inst, &StaticUser{}, []Link{
		{Rel: "curies", Name: "acme", Href: "https://example.com/{rel}", Templated: true},
		{Rel: "acme:widget", Href: "/widgets/1"},
	})

	env := inst.Wrap(context.Background(), &StaticUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("document is not an object")
	}
	links, ok := root["_links"].(map[string]any)
	if !ok {
		t.Fatalf("_links has type %T, want object", root["_links"])
	}
	curies, ok := links["curies"].([]any)
	if !ok {
		t.Fatalf("curies has type %T, want array", links["curies"])
	}
	if len(curies) != 1 {
		t.Fatalf("got %d curies, want 1", len(curies))
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
