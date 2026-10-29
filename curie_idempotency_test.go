// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

type curieIdempotentUser struct {
	ID int `json:"id"`
}

func newCurieIdempotentEnvelope(t *testing.T) *Envelope {
	t.Helper()

	inst := New()
	inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")
	RegisterInstance(inst, func(_ context.Context, _ *curieIdempotentUser) []Link {
		return []Link{
			{Rel: "self", Href: "/users/1"},
			{Rel: "acme:widget", Href: "/widgets/1"},
		}
	})
	return inst.Wrap(context.Background(), &curieIdempotentUser{ID: 1})
}

func TestCurieContract_MarshalIsIdempotent(t *testing.T) {
	env := newCurieIdempotentEnvelope(t)

	first, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, first)

	if _, exists := env.links["curies"]; exists {
		t.Fatal("marshal mutated envelope links by adding curies")
	}
	originalLinks := len(env.links)

	for i := 0; i < 10; i++ {
		next, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, next, first)
	}

	if got := len(env.links); got != originalLinks {
		t.Fatalf("envelope links changed from %d to %d entries", originalLinks, got)
	}
}

func TestCurieContract_MergesUserProvidedCuries(t *testing.T) {
	inst := New()
	inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")
	RegisterInstance(inst, func(_ context.Context, _ *curieIdempotentUser) []Link {
		return []Link{
			{Rel: "curies", Name: "user", Href: "https://example.com/user/{rel}", Templated: true},
			{Rel: "acme:widget", Href: "/widgets/1"},
		}
	})

	env := inst.Wrap(context.Background(), &curieIdempotentUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("document is not an object")
	}
	curies := relationObjects(t, root, "curies")
	if len(curies) != 2 {
		t.Fatalf("got %d curies, want 2 (user + resolved)", len(curies))
	}
	for i, curie := range curies {
		if _, nested := curie["name"]; !nested {
			t.Fatalf("curies[%d] is not a link object: %#v", i, curie)
		}
	}
	if got := curies[0]["name"]; got != "user" {
		t.Errorf("curies[0].name = %#v, want user", got)
	}
	if got := curies[1]["name"]; got != "acme" {
		t.Errorf("curies[1].name = %#v, want acme", got)
	}
}

func TestCurieContract_MergesRepeatedUserCuriesWithoutNesting(t *testing.T) {
	inst := New()
	inst.RegisterCurie("acme", "https://docs.example.com/rels/{rel}")

	env := inst.WrapRaw(&curieIdempotentUser{ID: 1})
	env.AddLink(Link{Rel: "curies", Name: "one", Href: "https://example.com/one/{rel}", Templated: true})
	env.AddLink(Link{Rel: "curies", Name: "two", Href: "https://example.com/two/{rel}", Templated: true})
	env.AddLink(Link{Rel: "acme:widget", Href: "/widgets/1"})

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("document is not an object")
	}
	curies := relationObjects(t, root, "curies")
	if len(curies) != 3 {
		t.Fatalf("got %d curies, want 3", len(curies))
	}
	wantNames := []string{"one", "two", "acme"}
	for i, want := range wantNames {
		if got := curies[i]["name"]; got != want {
			t.Errorf("curies[%d].name = %#v, want %q", i, got, want)
		}
	}
}

func TestCurieContract_ConcurrentMarshalSameEnvelope(t *testing.T) {
	env := newCurieIdempotentEnvelope(t)

	want, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 16
	outputs := make([][]byte, goroutines)
	errs := make([]error, goroutines)
	start := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			outputs[i], errs[i] = json.Marshal(env)
		}()
	}
	close(start)
	wg.Wait()

	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		assertJSONEqual(t, outputs[i], want)
	}
}
