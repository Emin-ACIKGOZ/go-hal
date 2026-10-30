// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

var grammarInvalidRelations = []string{"Self", "SELF", "a_b", "foo/bar", "?q", "#f", `a"b`}

func precomputedRelation(rel string) []byte {
	key, _ := json.Marshal(rel)
	return []byte(fmt.Sprintf(`{%s:{"href":"/x"}}`, key))
}

func TestDefaultRelationGrammarRemainsLenient(t *testing.T) {
	t.Parallel()

	for _, rel := range grammarInvalidRelations {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()

			t.Run("AddLink", func(t *testing.T) {
				env := New().WrapRaw(&embeddedTestUser{ID: 1})
				env.AddLink(Link{Rel: rel, Href: "/x"})
				b, err := json.Marshal(env)
				if err != nil {
					t.Fatalf("default AddLink rejected %q: %v", rel, err)
				}
				root := decodeJSONValue(t, b).(map[string]any)
				if len(relationObjects(t, root, rel)) != 1 {
					t.Fatalf("relation %q lost: %s", rel, b)
				}
			})

			t.Run("AddEmbedded", func(t *testing.T) {
				env := New().WrapRaw(&embeddedTestUser{ID: 1})
				env.AddEmbedded(rel, &embeddedTestUser{ID: 2})
				b, err := json.Marshal(env)
				if err != nil {
					t.Fatalf("default AddEmbedded rejected %q: %v", rel, err)
				}
				root := decodeJSONValue(t, b).(map[string]any)
				embedded := root["_embedded"].(map[string]any)
				if _, exists := embedded[rel]; !exists {
					t.Fatalf("embedded relation %q lost: %s", rel, b)
				}
			})

			t.Run("RegisterStatic", func(t *testing.T) {
				inst := New()
				links := []Link{{Rel: rel, Href: "/x"}}
				RegisterStatic(inst, &embeddedTestUser{}, links)
				if !reflect.DeepEqual(links, []Link{{Rel: rel, Href: "/x"}}) {
					t.Fatalf("RegisterStatic mutated input: %#v", links)
				}
				b, err := json.Marshal(inst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
				if err != nil {
					t.Fatalf("default static rejected %q: %v", rel, err)
				}
				root := decodeJSONValue(t, b).(map[string]any)
				if len(relationObjects(t, root, rel)) != 1 {
					t.Fatalf("static relation %q lost: %s", rel, b)
				}
			})

			t.Run("Generated", func(t *testing.T) {
				inst := New()
				RegisterInstance(inst, func(_ context.Context, _ *embeddedTestUser) []Link {
					return []Link{{Rel: rel, Href: "/x"}}
				})
				b, err := json.Marshal(inst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
				if err != nil {
					t.Fatalf("default generated relation rejected %q: %v", rel, err)
				}
				root := decodeJSONValue(t, b).(map[string]any)
				if len(relationObjects(t, root, rel)) != 1 {
					t.Fatalf("generated relation %q lost: %s", rel, b)
				}
			})

			t.Run("Precomputed", func(t *testing.T) {
				links := precomputedRelation(rel)
				handle, err := CompilePrecomputedLinks(links)
				if err != nil {
					t.Fatalf("default compile rejected %q: %v", rel, err)
				}
				b, err := json.Marshal(New().WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle))
				if err != nil {
					t.Fatalf("default precomputed rejected %q: %v", rel, err)
				}
				root := decodeJSONValue(t, b).(map[string]any)
				if len(relationObjects(t, root, rel)) != 1 {
					t.Fatalf("precomputed relation %q lost: %s", rel, b)
				}
			})
		})
	}
}

func TestStrictRelationGrammarRejects(t *testing.T) {
	t.Parallel()

	for _, rel := range grammarInvalidRelations {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()

			t.Run("AddLink", func(t *testing.T) {
				env := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 1})
				env.AddLink(Link{Rel: rel, Href: "/x"})
				out, err := json.Marshal(env)
				assertStrictRelationFailure(t, out, err)
				if len(env.links) != 0 {
					t.Fatalf("invalid relation mutated links: %v", env.links)
				}
			})

			t.Run("AddEmbedded", func(t *testing.T) {
				env := New(WithStrictRelations()).WrapRaw(&embeddedTestUser{ID: 1})
				env.AddEmbedded(rel, &embeddedTestUser{ID: 2})
				out, err := json.Marshal(env)
				assertStrictRelationFailure(t, out, err)
				if len(env.embedded) != 0 {
					t.Fatalf("invalid relation mutated embedded: %v", env.embedded)
				}
			})

			t.Run("RegisterStatic", func(t *testing.T) {
				assertPanics(t, "hal: RegisterStatic:", func() {
					RegisterStatic(New(WithStrictRelations()), &embeddedTestUser{}, []Link{
						{Rel: rel, Href: "/x"},
					})
				})
			})

			t.Run("Generated", func(t *testing.T) {
				inst := New(WithStrictRelations())
				RegisterInstance(inst, func(_ context.Context, _ *embeddedTestUser) []Link {
					return []Link{{Rel: rel, Href: "/x"}}
				})
				out, err := json.Marshal(inst.Wrap(context.Background(), &embeddedTestUser{ID: 1}))
				assertStrictRelationFailure(t, out, err)
			})

			t.Run("Precomputed", func(t *testing.T) {
				links := precomputedRelation(rel)
				handle, err := CompilePrecomputedLinks(links)
				if err != nil {
					t.Fatalf("structural compile rejected %q: %v", rel, err)
				}
				env := New(WithStrictRelations()).WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, handle)
				out, err := json.Marshal(env)
				assertStrictRelationFailure(t, out, err)
			})
		})
	}
}

func assertStrictRelationFailure(t *testing.T, out []byte, err error) {
	t.Helper()
	if !errors.Is(err, ErrInvalidRelation) {
		t.Fatalf("error = %v (output %s), want ErrInvalidRelation", err, out)
	}
	if len(out) != 0 {
		t.Fatalf("error returned partial output: %s", out)
	}
}
