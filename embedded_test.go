// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

type embeddedTestUser struct {
	ID int `json:"id"`
}

func TestAddEmbedded_ObjectMultiplicityAndSlices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		add   func(*Envelope)
		want  []byte
		check func(*testing.T, map[string]any)
	}{
		{
			name: "one object",
			add: func(e *Envelope) {
				e.AddEmbedded("author", &embeddedTestUser{ID: 1})
			},
			check: func(t *testing.T, root map[string]any) {
				t.Helper()
				embedded := root["_embedded"].(map[string]any)
				author, ok := embedded["author"].(map[string]any)
				if !ok {
					t.Fatalf("author has type %T, want object", embedded["author"])
				}
				if got := author["id"]; fmt.Sprint(got) != "1" {
					t.Errorf("author.id = %#v, want 1", got)
				}
			},
		},
		{
			name: "repeated objects become array",
			add: func(e *Envelope) {
				e.AddEmbedded("item", &embeddedTestUser{ID: 1})
				e.AddEmbedded("item", &embeddedTestUser{ID: 2})
			},
			check: func(t *testing.T, root map[string]any) {
				t.Helper()
				embedded := root["_embedded"].(map[string]any)
				items, ok := embedded["item"].([]any)
				if !ok || len(items) != 2 {
					t.Fatalf("item = %#v, want 2 elements", embedded["item"])
				}
			},
		},
		{
			name: "one-element slice becomes object",
			add: func(e *Envelope) {
				e.AddEmbedded("item", []*embeddedTestUser{{ID: 1}})
			},
			check: func(t *testing.T, root map[string]any) {
				t.Helper()
				embedded := root["_embedded"].(map[string]any)
				if _, ok := embedded["item"].(map[string]any); !ok {
					t.Fatalf("item has type %T, want object", embedded["item"])
				}
			},
		},
		{
			name: "empty slice is a no-op",
			add: func(e *Envelope) {
				e.AddEmbedded("item", []*embeddedTestUser{})
			},
			check: func(t *testing.T, root map[string]any) {
				t.Helper()
				if _, exists := root["_embedded"]; exists {
					t.Fatal("empty slice produced _embedded")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := New().WrapRaw(&embeddedTestUser{ID: 9})
			tc.add(env)
			b, err := json.Marshal(env)
			if err != nil {
				t.Fatal(err)
			}
			assertNoDuplicateKeys(t, b)
			root, ok := decodeJSONValue(t, b).(map[string]any)
			if !ok {
				t.Fatal("document is not an object")
			}
			tc.check(t, root)
		})
	}
}

func TestAddEmbedded_NestedEnvelopeKeepsLinks(t *testing.T) {
	t.Parallel()

	inst := New()
	RegisterInstance(inst, func(_ context.Context, u *embeddedTestUser) []Link {
		return []Link{{Rel: "self", Href: "/users/" + strconv.Itoa(u.ID)}}
	})

	child := inst.Wrap(context.Background(), &embeddedTestUser{ID: 1})
	parent := inst.WrapRaw(&embeddedTestUser{ID: 9})
	parent.AddEmbedded("author", child)

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root := decodeJSONValue(t, b).(map[string]any)
	embedded := root["_embedded"].(map[string]any)
	author, ok := embedded["author"].(map[string]any)
	if !ok {
		t.Fatalf("author has type %T, want object", embedded["author"])
	}
	self := relationObjects(t, author, "self")
	if len(self) != 1 || self[0]["href"] != "/users/1" {
		t.Fatalf("author self = %#v, want /users/1", self)
	}
}

func TestAddEmbedded_RawValuesAreNotAutoWrapped(t *testing.T) {
	t.Parallel()

	inst := New()
	RegisterInstance(inst, func(_ context.Context, _ *embeddedTestUser) []Link {
		return []Link{{Rel: "self", Href: "/generated"}}
	})

	env := inst.WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("raw", &embeddedTestUser{ID: 2})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	raw := root["_embedded"].(map[string]any)["raw"].(map[string]any)
	if _, hasLinks := raw["_links"]; hasLinks {
		t.Fatal("raw embedded value was auto-wrapped with links")
	}
}

func TestAddEmbedded_AcceptsMapsAndRawObjects(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("map", map[string]any{"name": "M"})
	env.AddEmbedded("raw", json.RawMessage(`{"name":"R"}`))

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONValue(t, b).(map[string]any)
	embedded := root["_embedded"].(map[string]any)
	if embedded["map"].(map[string]any)["name"] != "M" {
		t.Fatalf("map = %#v", embedded["map"])
	}
	if embedded["raw"].(map[string]any)["name"] != "R" {
		t.Fatalf("raw = %#v", embedded["raw"])
	}
}

func TestAddEmbedded_RejectsInvalidValues(t *testing.T) {
	t.Parallel()

	var nilEnvelope *Envelope
	tests := []struct {
		name  string
		value any
	}{
		{name: "nil", value: nil},
		{name: "typed nil envelope", value: nilEnvelope},
		{name: "envelope value", value: Envelope{Data: &embeddedTestUser{ID: 1}}},
		{name: "string", value: "resource"},
		{name: "number", value: 42},
		{name: "bool", value: true},
		{name: "bytes", value: []byte(`{}`)},
		{name: "raw null", value: json.RawMessage(`null`)},
		{name: "raw array", value: json.RawMessage(`[]`)},
		{name: "raw scalar", value: json.RawMessage(`42`)},
		{name: "raw invalid", value: json.RawMessage(`{`)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := New().WrapRaw(&embeddedTestUser{ID: 1})
			env.AddEmbedded("author", tc.value)

			b, err := json.Marshal(env)
			if !errors.Is(err, ErrInvalidEmbedded) {
				t.Fatalf("error = %v (output %s), want ErrInvalidEmbedded", err, b)
			}
			if len(b) != 0 {
				t.Fatalf("error returned partial output: %s", b)
			}
		})
	}
}

func TestAddEmbedded_RejectsInvalidRelations(t *testing.T) {
	t.Parallel()

	for _, rel := range []string{"", "   ", "a b", "a\tb", "a\x00b", "a\xffb"} {
		t.Run(rel, func(t *testing.T) {
			env := New().WrapRaw(&embeddedTestUser{ID: 1})
			env.AddEmbedded(rel, &embeddedTestUser{ID: 2})

			b, err := json.Marshal(env)
			if !errors.Is(err, ErrInvalidRelation) {
				t.Fatalf("error = %v (output %s), want ErrInvalidRelation", err, b)
			}
			if len(b) != 0 {
				t.Fatalf("error returned partial output: %s", b)
			}
		})
	}
}

func TestAddEmbedded_RejectsBareTokenCaseCollision(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("author", &embeddedTestUser{ID: 2})
	env.AddEmbedded("Author", &embeddedTestUser{ID: 3})

	_, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidRelation) || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("error = %v, want bare-token collision", err)
	}
}

func TestAddEmbedded_ValueReceiverMarshalingStillWorks(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("author", &embeddedTestUser{ID: 2})

	want, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(*env)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, got, want)
}

func TestAddEmbedded_FirstErrorWins(t *testing.T) {
	t.Parallel()

	env := New().WrapRaw(&embeddedTestUser{ID: 1})
	env.AddEmbedded("", &embeddedTestUser{ID: 2})
	env.AddEmbedded("author", nil)

	_, err := json.Marshal(env)
	if !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("error = %v, want first ErrInvalidRelation", err)
	}
}
