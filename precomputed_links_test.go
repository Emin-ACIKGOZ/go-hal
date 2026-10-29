// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func TestCompilePrecomputedLinks_ValidForms(t *testing.T) {
	want := []byte(`{"_links":{"self":{"href":"/x"},"item":[{"href":"/i/1"}],"acme:widget":{"href":"/w"}}}`)

	forms := [][]byte{
		[]byte(`{"self":{"href":"/x"},"item":[{"href":"/i/1"}],"acme:widget":{"href":"/w"}}`),
		[]byte(`{"_links":{"self":{"href":"/x"},"item":[{"href":"/i/1"}],"acme:widget":{"href":"/w"}}}`),
		[]byte(" \n {\"_links\":{\"self\":{\"href\":\"/x\"},\"item\":[{\"href\":\"/i/1\"}],\"acme:widget\":{\"href\":\"/w\"}}} \n"),
	}

	for _, form := range forms {
		t.Run(string(form), func(t *testing.T) {
			links, err := CompilePrecomputedLinks(form)
			if err != nil {
				t.Fatal(err)
			}
			if !links.validated {
				t.Fatal("compiled handle is not validated")
			}
			if !jsonValuesEqualBytes(links.JSON, want) {
				t.Fatalf("JSON = %s, want %s", links.JSON, want)
			}
			wantPrefixes := []string{"acme"}
			if len(links.prefixes) != len(wantPrefixes) || links.prefixes[0] != wantPrefixes[0] {
				t.Fatalf("prefixes = %v, want %v", links.prefixes, wantPrefixes)
			}
		})
	}
}

func jsonValuesEqualBytes(got, want []byte) bool {
	return jsonValuesEqual(got, want) == nil
}

func TestCompilePrecomputedLinks_EmptyAndInvalid(t *testing.T) {
	for _, input := range [][]byte{nil, []byte(" \n\t")} {
		links, err := CompilePrecomputedLinks(input)
		if err != nil {
			t.Fatal(err)
		}
		if links == nil || links.JSON != nil || !links.validated {
			t.Fatalf("compile(%q) = %#v, want validated empty handle", input, links)
		}
	}

	invalid := [][]byte{
		[]byte(`null`),
		[]byte(`[]`),
		[]byte(`not json`),
		[]byte(`{"self":{"href":null}}`),
		[]byte(`{"curies":[{"name":"acme","href":""}]}`),
	}
	for _, input := range invalid {
		t.Run(string(input), func(t *testing.T) {
			links, err := CompilePrecomputedLinks(input)
			if err == nil {
				t.Fatalf("expected error, got %#v", links)
			}
			if links != nil {
				t.Fatalf("error returned handle: %#v", links)
			}
		})
	}
}

func TestWrapPrecomputedLinks_HandleSemantics(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	compiled, err := CompilePrecomputedLinks([]byte(`{"self":{"href":"/x"}}`))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("nil handle uses WrapRaw semantics", func(t *testing.T) {
		inst := New()
		RegisterInstance(inst, func(_ context.Context, _ *User) []Link {
			return []Link{{Rel: "ignored", Href: "/generated"}}
		})

		b, err := json.Marshal(inst.WrapPrecomputedLinks(&User{ID: 1}, nil))
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, b, []byte(`{"id":1}`))
	})

	t.Run("zero handle suppresses generators", func(t *testing.T) {
		inst := New()
		RegisterInstance(inst, func(_ context.Context, _ *User) []Link {
			return []Link{{Rel: "ignored", Href: "/generated"}}
		})

		b, err := json.Marshal(inst.WrapPrecomputedLinks(&User{ID: 1}, &PrecomputedLinks{}))
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, b, []byte(`{"id":1}`))
	})

	t.Run("compiled handle emits links", func(t *testing.T) {
		b, err := json.Marshal(New().WrapPrecomputedLinks(&User{ID: 1}, compiled))
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, b, []byte(`{"id":1,"_links":{"self":{"href":"/x"}}}`))
	})

	t.Run("unvalidated JSON is a deferred error", func(t *testing.T) {
		handle := &PrecomputedLinks{JSON: []byte(`{"_links":{"self":{"href":"/x"}}}`)}
		b, err := json.Marshal(New().WrapPrecomputedLinks(&User{ID: 1}, handle))
		if !errors.Is(err, errPrecomputedUnvalidated) {
			t.Fatalf("error = %v (output %s), want unvalidated handle error", err, b)
		}
		if len(b) != 0 {
			t.Fatalf("error returned partial output: %s", b)
		}
	})
}

func TestMarshalJSON_OutputMutationDoesNotCorruptCachedLinks(t *testing.T) {
	links, err := CompilePrecomputedLinks([]byte(`{"self":{"href":"/x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	env := New().WrapPrecomputedLinks(nil, links)

	first, err := env.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	first[0] = 'x' // Direct callers may mutate returned bytes.

	second, err := env.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, second, []byte(`{"_links":{"self":{"href":"/x"}}}`))
}

func TestWrapPrecomputedLinks_ReuseAndConcurrency(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	links, err := CompilePrecomputedLinks([]byte(`{"self":{"href":"/x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	inst := New()
	want := []byte(`{"id":1,"_links":{"self":{"href":"/x"}}}`)

	first, err := json.Marshal(inst.WrapPrecomputedLinks(&User{ID: 1}, links))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, first, want)

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
			outputs[i], errs[i] = json.Marshal(inst.WrapPrecomputedLinks(&User{ID: 1}, links))
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
