// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func marshalShapedLink(t *testing.T, inst *Instance, links ...Link) map[string]any {
	t.Helper()
	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	for _, link := range links {
		env.AddLink(link)
	}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return decodeJSONValue(t, b).(map[string]any)
}

func TestLinkShape_AutoDefault(t *testing.T) {
	t.Parallel()

	root := marshalShapedLink(t, New(),
		Link{Rel: "self", Href: "/a"},
		Link{Rel: "item", Href: "/1"},
		Link{Rel: "item", Href: "/2"},
	)
	links := root["_links"].(map[string]any)
	if _, ok := links["self"].(map[string]any); !ok {
		t.Fatalf("self has type %T, want object", links["self"])
	}
	if _, ok := links["item"].([]any); !ok {
		t.Fatalf("item has type %T, want array", links["item"])
	}
}

func TestLinkShape_Array(t *testing.T) {
	t.Parallel()

	root := marshalShapedLink(t, New(WithLinkShape(ShapeArray)),
		Link{Rel: "self", Href: "/a"},
		Link{Rel: "item", Href: "/1"},
		Link{Rel: "item", Href: "/2"},
	)
	links := root["_links"].(map[string]any)
	for _, rel := range []string{"self", "item"} {
		items, ok := links[rel].([]any)
		if !ok {
			t.Fatalf("%s has type %T, want array", rel, links[rel])
		}
		if rel == "self" && len(items) != 1 {
			t.Fatalf("self = %#v, want 1 element", items)
		}
	}
}

func TestLinkShape_Object(t *testing.T) {
	t.Parallel()

	root := marshalShapedLink(t, New(WithLinkShape(ShapeObject)),
		Link{Rel: "self", Href: "/a"},
	)
	links := root["_links"].(map[string]any)
	if _, ok := links["self"].(map[string]any); !ok {
		t.Fatalf("self has type %T, want object", links["self"])
	}
}

func TestLinkShape_ObjectRejectsRepeatedLinks(t *testing.T) {
	t.Parallel()

	env := New(WithLinkShape(ShapeObject)).WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "item", Href: "/1"})
	env.AddLink(Link{Rel: "item", Href: "/2"})

	b, err := json.Marshal(env)
	if !errors.Is(err, ErrLinkShape) {
		t.Fatalf("error = %v (output %s), want ErrLinkShape", err, b)
	}
	if len(b) != 0 {
		t.Fatalf("error returned partial output: %s", b)
	}
}

func TestLinkShape_CuriesAlwaysArray(t *testing.T) {
	t.Parallel()

	inst := New(WithLinkShape(ShapeObject))
	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "curies", Name: "acme", Href: "https://docs.example/{rel}", Templated: true})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	links := root["_links"].(map[string]any)
	if _, ok := links["curies"].([]any); !ok {
		t.Fatalf("curies has type %T, want array", links["curies"])
	}
}

func TestLinkShape_GeneratedAndStatic(t *testing.T) {
	t.Parallel()

	inst := New(WithLinkShape(ShapeArray))
	RegisterInstance(inst, func(_ context.Context, _ *embeddedTestUser) []Link {
		return []Link{{Rel: "self", Href: "/generated"}}
	})
	env := inst.Wrap(context.Background(), &embeddedTestUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, ok := root["_links"].(map[string]any)["self"].([]any); !ok {
		t.Fatal("generated link was not shaped as an array")
	}

	staticInst := New(WithLinkShape(ShapeArray))
	RegisterStatic(staticInst, &embeddedTestUser{}, []Link{{Rel: "self", Href: "/static"}})
	b, err = json.Marshal(staticInst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
	if err != nil {
		t.Fatal(err)
	}
	root = decodeJSONValue(t, b).(map[string]any)
	if _, ok := root["_links"].(map[string]any)["self"].([]any); !ok {
		t.Fatal("static link was not shaped as an array")
	}
}

func TestLinkShape_StaticObjectRejectsRepeatedLinks(t *testing.T) {
	t.Parallel()

	assertPanics(t, "hal: RegisterStatic:", func() {
		RegisterStatic(New(WithLinkShape(ShapeObject)), &embeddedTestUser{}, []Link{
			{Rel: "item", Href: "/1"},
			{Rel: "item", Href: "/2"},
		})
	})
}

func TestLinkShape_CollectionSelf(t *testing.T) {
	t.Parallel()

	page := New(WithLinkShape(ShapeArray)).Collection(
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
	if _, ok := root["_links"].(map[string]any)["self"].([]any); !ok {
		t.Fatal("collection self link was not shaped as an array")
	}
}

func TestLinkShape_PrecomputedIsNotRewritten(t *testing.T) {
	t.Parallel()

	handle, err := CompilePrecomputedLinks([]byte(`{"self":{"href":"/x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	env := New(WithLinkShape(ShapeArray)).WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, ok := root["_links"].(map[string]any)["self"].(map[string]any); !ok {
		t.Fatal("precomputed links were rewritten by shape policy")
	}
}

func TestRelationShape_OverridesDefault(t *testing.T) {
	t.Parallel()

	inst := New(
		WithLinkShape(ShapeArray),
		WithRelationShape("self", ShapeObject),
	)
	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: "/a"})
	env.AddLink(Link{Rel: "item", Href: "/1"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	links := root["_links"].(map[string]any)
	if _, ok := links["self"].(map[string]any); !ok {
		t.Fatalf("self has type %T, want object override", links["self"])
	}
	if _, ok := links["item"].([]any); !ok {
		t.Fatalf("item has type %T, want array default", links["item"])
	}
}

func TestRelationShape_CanRelaxGlobalObject(t *testing.T) {
	t.Parallel()

	inst := New(
		WithLinkShape(ShapeObject),
		WithRelationShape("item", ShapeArray),
	)
	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: "/a"})
	env.AddLink(Link{Rel: "item", Href: "/1"})
	env.AddLink(Link{Rel: "item", Href: "/2"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	links := root["_links"].(map[string]any)
	if _, ok := links["item"].([]any); !ok {
		t.Fatalf("item has type %T, want array override", links["item"])
	}
}

func TestRelationShape_AppliesToStatic(t *testing.T) {
	t.Parallel()

	inst := New(WithRelationShape("self", ShapeArray))
	RegisterStatic(inst, &embeddedTestUser{}, []Link{{Rel: "self", Href: "/static"}})
	b, err := json.Marshal(inst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	if _, ok := root["_links"].(map[string]any)["self"].([]any); !ok {
		t.Fatal("per-relation shape was not applied to static links")
	}
}

func TestRelationShape_LastOverrideWins(t *testing.T) {
	t.Parallel()

	root := marshalShapedLink(t, New(
		WithRelationShape("self", ShapeArray),
		WithRelationShape("self", ShapeObject),
	), Link{Rel: "self", Href: "/a"})
	if _, ok := root["_links"].(map[string]any)["self"].(map[string]any); !ok {
		t.Fatal("later relation-shape override did not win")
	}
}

func TestLinkShape_String(t *testing.T) {
	t.Parallel()

	for shape, want := range map[LinkShape]string{
		ShapeAuto:     "auto",
		ShapeArray:    "array",
		ShapeObject:   "object",
		LinkShape(99): "auto",
	} {
		if got := shape.String(); got != want {
			t.Errorf("LinkShape(%d).String() = %q, want %q", shape, got, want)
		}
	}
}
