// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type addLinkUser struct {
	ID int `json:"id"`
}

func TestAddLink_StaticAndPrecomputedRejected(t *testing.T) {
	staticInst := New()
	RegisterStatic(staticInst, &addLinkUser{}, []Link{{Rel: "self", Href: "/static"}})

	emptyStaticInst := New()
	RegisterStatic(emptyStaticInst, &addLinkUser{}, nil)

	compiled, err := CompilePrecomputedLinks([]byte(`{"self":{"href":"/compiled"}}`))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		env  *Envelope
	}{
		{name: "static", env: staticInst.Wrap(context.Background(), &addLinkUser{ID: 1})},
		{name: "empty static", env: emptyStaticInst.Wrap(context.Background(), &addLinkUser{ID: 1})},
		{name: "compiled", env: New().WrapPrecomputedLinks(&addLinkUser{ID: 1}, compiled)},
		{name: "safe precomputed", env: New().WrapPrecomputed(context.Background(), &addLinkUser{ID: 1}, []byte(`{"self":{"href":"/safe"}}`))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.env.AddLink(Link{Rel: "edit", Href: "/edit"})

			b, err := json.Marshal(tc.env)
			if !errors.Is(err, ErrAddLinkPrecomputed) {
				t.Fatalf("error = %v (output %s), want ErrAddLinkPrecomputed", err, b)
			}
			if len(b) != 0 {
				t.Fatalf("error returned partial output: %s", b)
			}
		})
	}
}

func TestAddLink_RawAndGeneratedStillAllowed(t *testing.T) {
	t.Run("raw", func(t *testing.T) {
		env := New().WrapRaw(&addLinkUser{ID: 1})
		env.AddLink(Link{Rel: "self", Href: "/raw"})

		b, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, b, []byte(`{"id":1,"_links":{"self":{"href":"/raw"}}}`))
	})

	t.Run("generated", func(t *testing.T) {
		inst := New()
		RegisterInstance(inst, func(_ context.Context, _ *addLinkUser) []Link {
			return []Link{{Rel: "self", Href: "/generated"}}
		})
		env := inst.Wrap(context.Background(), &addLinkUser{ID: 1})
		env.AddLink(Link{Rel: "edit", Href: "/edit"})

		b, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, b, []byte(`{
			"id":1,
			"_links":{
				"self":{"href":"/generated"},
				"edit":{"href":"/edit"}
			}
		}`))
	})
}

func TestAddLink_EmptyRelationIsDeferred(t *testing.T) {
	for _, rel := range []string{"", "   "} {
		t.Run(rel, func(t *testing.T) {
			env := New().WrapRaw(&addLinkUser{ID: 1})
			env.AddLink(Link{Rel: rel, Href: "/x"})

			b, err := json.Marshal(env)
			if !errors.Is(err, errEmptyRelation) {
				t.Fatalf("error = %v (output %s), want empty relation error", err, b)
			}
			if len(b) != 0 {
				t.Fatalf("error returned partial output: %s", b)
			}
		})
	}
}
