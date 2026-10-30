// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestCompilePrecomputedLinksStrict(t *testing.T) {
	t.Parallel()

	valid := []byte(`{"self":{"href":"/x"},"find":{"href":"/x{?id}","templated":true}}`)
	handle, err := CompilePrecomputedLinksStrict(valid)
	if err != nil {
		t.Fatalf("valid strict handle rejected: %v", err)
	}
	if !handle.strictLinks || !handle.strictCuries {
		t.Fatalf("strict flags = (%t, %t), want both true", handle.strictLinks, handle.strictCuries)
	}

	invalid := [][]byte{
		[]byte(`{"self":{"href":""}}`),
		[]byte(`{"find":{"href":"/x{?id}"}}`),
		[]byte(`{"self":{"href":"/x","templated":true}}`),
		[]byte(`{"self":{"href":"/x","type":"nope"}}`),
	}
	for _, links := range invalid {
		if _, err := CompilePrecomputedLinksStrict(links); !errors.Is(err, ErrInvalidLink) {
			t.Errorf("CompilePrecomputedLinksStrict(%s) = %v, want ErrInvalidLink", links, err)
		}
	}
}

func TestCompilePrecomputedLinks_RemainsStructural(t *testing.T) {
	t.Parallel()

	handle, err := CompilePrecomputedLinks([]byte(`{"self":{"href":""}}`))
	if err != nil {
		t.Fatalf("structural compile rejected permissive link: %v", err)
	}
	if handle.strictLinks || handle.strictCuries {
		t.Fatal("structural compile marked strict flags")
	}
}

func TestWrapPrecomputed_StrictInstanceValidates(t *testing.T) {
	t.Parallel()

	env := New(WithStrictLinks()).WrapPrecomputed(
		context.Background(),
		&embeddedTestUser{ID: 1},
		[]byte(`{"find":{"href":"/x{?id}"}}`),
	)
	if _, err := json.Marshal(env); !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("error = %v, want ErrInvalidLink", err)
	}

	env = New(WithStrictLinks()).WrapPrecomputed(
		context.Background(),
		&embeddedTestUser{ID: 1},
		[]byte(`{"find":{"href":"/x{?id}","templated":true}}`),
	)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("valid strict precomputed rejected: %v", err)
	}
	if !json.Valid(b) {
		t.Fatalf("invalid JSON: %s", b)
	}
}

func TestWrapPrecomputedLinks_StrictRejectsNonStrictHandle(t *testing.T) {
	t.Parallel()

	structural, err := CompilePrecomputedLinks([]byte(`{"self":{"href":"/x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	strict, err := CompilePrecomputedLinksStrict([]byte(`{"self":{"href":"/x"}}`))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		handle *PrecomputedLinks
		want   error
	}{
		{name: "structural", handle: structural, want: ErrInvalidLink},
		{name: "strict", handle: strict},
		{name: "zero", handle: &PrecomputedLinks{}},
		{name: "nil", handle: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := New(WithStrictLinks()).WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, tc.handle)
			b, err := json.Marshal(env)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("error = %v (output %s), want %v", err, b, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("valid handle rejected: %v", err)
			}
		})
	}
}

func TestRegisterStatic_StrictValidatesLinks(t *testing.T) {
	t.Parallel()

	assertPanics(t, "hal: RegisterStatic: hal: invalid link", func() {
		RegisterStatic(New(WithStrictLinks()), &embeddedTestUser{}, []Link{
			{Rel: "self", Href: ""},
		})
	})

	// Permissive registration keeps accepting the same link.
	inst := New()
	RegisterStatic(inst, &embeddedTestUser{}, []Link{{Rel: "self", Href: ""}})
	env := inst.Wrap(context.Background(), &embeddedTestUser{ID: 1})
	if _, err := json.Marshal(env); err != nil {
		t.Fatalf("permissive static rejected: %v", err)
	}
}

func TestCollection_StrictValidatesSelfLink(t *testing.T) {
	t.Parallel()

	inst := New(WithStrictLinks())
	assertPanics(t, "hal: Collection:", func() {
		inst.Collection(context.Background(), []*embeddedTestUser{}, 0, Link{Rel: "self", Href: ""})
	})
}
