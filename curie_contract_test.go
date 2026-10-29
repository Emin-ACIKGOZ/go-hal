// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
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
