// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type curieContractUser struct {
	ID int `json:"id"`
}

func marshalCurieLinks(t *testing.T, inst *Instance, links ...Link) map[string]any {
	t.Helper()

	RegisterInstance(inst, func(_ context.Context, _ *curieContractUser) []Link {
		return links
	})

	env := inst.Wrap(context.Background(), &curieContractUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	value := decodeJSONValue(t, b)
	root, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("document has type %T, want object", value)
	}
	return root
}

func TestCurieContract_DeduplicatesPrefixes(t *testing.T) {
	inst := New()
	inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")

	root := marshalCurieLinks(t, inst,
		Link{Rel: "acme:widget", Href: "/widgets/1"},
		Link{Rel: "acme:gadget", Href: "/gadgets/1"},
	)

	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 {
		t.Fatalf("got %d curies, want 1", len(curies))
	}
	curie := curies[0]
	if got := curie["name"]; got != "acme" {
		t.Errorf("name = %#v, want %q", got, "acme")
	}
	if got := curie["href"]; got != "https://docs.example.com/rels/{rel}" {
		t.Errorf("href = %#v, want templated docs URI", got)
	}
	if got := curie["templated"]; got != true {
		t.Errorf("templated = %#v, want true", got)
	}

	if len(relationObjects(t, root, "acme:widget")) != 1 {
		t.Error("original acme:widget relation was lost")
	}
	if len(relationObjects(t, root, "acme:gadget")) != 1 {
		t.Error("original acme:gadget relation was lost")
	}
}

func TestCurieContract_SortsMultiplePrefixes(t *testing.T) {
	inst := New()
	// Register in reverse alphabetical order to prove output is deterministic.
	inst.RegisterCurie("zeta", "https://docs.example.com/zeta/{rel}")
	inst.RegisterCurie("alpha", "https://docs.example.com/alpha/{rel}")

	root := marshalCurieLinks(t, inst,
		Link{Rel: "zeta:thing", Href: "/z"},
		Link{Rel: "alpha:thing", Href: "/a"},
	)

	curies := relationObjects(t, root, "curies")
	if len(curies) != 2 {
		t.Fatalf("got %d curies, want 2", len(curies))
	}
	if got := curies[0]["name"]; got != "alpha" {
		t.Errorf("curies[0].name = %#v, want alpha", got)
	}
	if got := curies[1]["name"]; got != "zeta" {
		t.Errorf("curies[1].name = %#v, want zeta", got)
	}
}

func TestCurieContract_UserDeclarationWins(t *testing.T) {
	inst := New()
	inst.RegisterCurie("acme", "https://registered.example/{rel}")

	root := marshalCurieLinks(t, inst,
		Link{Rel: "curies", Name: "acme", Href: "https://user.example/{rel}", Templated: true},
		Link{Rel: "acme:widget", Href: "/widgets/1"},
	)

	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 {
		t.Fatalf("got %d curies, want 1", len(curies))
	}
	if got := curies[0]["href"]; got != "https://user.example/{rel}" {
		t.Fatalf("curies[0].href = %#v, want user declaration", got)
	}
}

func TestCurieContract_SingleUserDeclarationBecomesArray(t *testing.T) {
	root := marshalCurieLinks(t, New(),
		Link{Rel: "curies", Name: "one", Href: "https://example.com/one/{rel}"},
	)

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

func TestCurieContract_RejectsInvalidUserDeclarations(t *testing.T) {
	tests := []struct {
		name string
		link Link
		want error
	}{
		{
			name: "empty name",
			link: Link{Rel: "curies", Href: "https://example.com/{rel}"},
			want: ErrInvalidCurie,
		},
		{
			name: "whitespace name",
			link: Link{Rel: "curies", Name: "ac me", Href: "https://example.com/{rel}"},
			want: ErrInvalidCurie,
		},
		{
			name: "empty href",
			link: Link{Rel: "curies", Name: "acme"},
			want: ErrInvalidCurie,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := New().WrapRaw(&curieContractUser{ID: 1})
			env.AddLink(tc.link)

			b, err := json.Marshal(env)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v (output %s), want %v", err, b, tc.want)
			}
			if len(b) != 0 {
				t.Fatalf("error returned partial output: %s", b)
			}
		})
	}
}

func TestCurieContract_RejectsDuplicateUserDeclarations(t *testing.T) {
	env := New().WrapRaw(&curieContractUser{ID: 1})
	env.AddLink(Link{Rel: "curies", Name: "acme", Href: "https://example.com/one/{rel}"})
	env.AddLink(Link{Rel: "curies", Name: "acme", Href: "https://example.com/two/{rel}"})

	b, err := json.Marshal(env)
	if !errors.Is(err, ErrDuplicateCurie) {
		t.Fatalf("error = %v (output %s), want ErrDuplicateCurie", err, b)
	}
	if len(b) != 0 {
		t.Fatalf("error returned partial output: %s", b)
	}
}

func TestCurieContract_UnknownPrefixEmitsNothing(t *testing.T) {
	inst := New()
	inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")

	root := marshalCurieLinks(t, inst, Link{Rel: "unknown:thing", Href: "/x"})

	links, ok := root["_links"].(map[string]any)
	if !ok {
		t.Fatalf("_links has type %T, want object", root["_links"])
	}
	if _, exists := links["curies"]; exists {
		t.Fatal("unknown prefix produced a curies declaration")
	}
	if _, exists := links["unknown:thing"]; !exists {
		t.Fatal("unknown prefix link relation was not preserved")
	}
}

func TestCurieContract_NoColonNoDeclaration(t *testing.T) {
	inst := New()
	inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")

	root := marshalCurieLinks(t, inst, Link{Rel: "self", Href: "/x"})

	links, ok := root["_links"].(map[string]any)
	if !ok {
		t.Fatalf("_links has type %T, want object", root["_links"])
	}
	if _, exists := links["curies"]; exists {
		t.Fatal("non-CURIE relation produced a curies declaration")
	}
}
