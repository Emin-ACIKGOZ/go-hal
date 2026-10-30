// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

type duplicateMarshaler struct{}

func (duplicateMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`{"a":1,"a":2}`), nil
}

type scalarMarshaler struct{}

func (scalarMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`"scalar"`), nil
}

func TestWithStrictJSON_RejectsDuplicateDataKeys(t *testing.T) {
	t.Parallel()

	data := json.RawMessage(`{"a":1,"a":2}`)
	for _, tc := range []struct {
		name string
		env  *Envelope
	}{
		{name: "raw", env: New(WithStrictJSON()).WrapRaw(data)},
		{
			name: "generated",
			env: func() *Envelope {
				inst := New(WithStrictJSON())
				RegisterInstance(inst, func(_ context.Context, _ *json.RawMessage) []Link {
					return []Link{{Rel: "self", Href: "/x"}}
				})
				return inst.Wrap(context.Background(), &data)
			}(),
		},
		{
			name: "precomputed",
			env: New(WithStrictJSON()).WrapPrecomputed(
				context.Background(), data, []byte(`{"self":{"href":"/x"}}`),
			),
		},
		{
			name: "static",
			env: func() *Envelope {
				inst := New(WithStrictJSON())
				RegisterStatic(inst, &json.RawMessage{}, []Link{{Rel: "self", Href: "/x"}})
				return inst.Wrap(context.Background(), data)
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := json.Marshal(tc.env)
			if !errors.Is(err, ErrDuplicateJSONKey) {
				t.Fatalf("error = %v (output %s), want ErrDuplicateJSONKey", err, out)
			}
			if len(out) != 0 {
				t.Fatalf("error returned partial output: %s", out)
			}
			var dup *DuplicateKeyError
			if !errors.As(err, &dup) || dup.Key != "a" {
				t.Fatalf("error does not expose DuplicateKeyError: %v", err)
			}
		})
	}
}

func TestWithStrictJSON_NestedDuplicate(t *testing.T) {
	t.Parallel()

	env := New(WithStrictJSON()).WrapRaw(json.RawMessage(`{"outer":{"x":1,"x":2}}`))
	_, err := json.Marshal(env)
	if !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v, want ErrDuplicateJSONKey", err)
	}
}

func TestWithStrictJSON_DefaultRemainsPermissive(t *testing.T) {
	t.Parallel()

	data := json.RawMessage(`{"a":1,"a":2}`)
	for _, env := range []*Envelope{
		New().WrapRaw(data),
		New().WrapPrecomputed(context.Background(), data, []byte(`{"self":{"href":"/x"}}`)),
	} {
		out, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("default marshal rejected duplicate data: %v", err)
		}
		if !json.Valid(out) {
			t.Fatalf("default marshal produced invalid JSON: %s", out)
		}
	}
}

func TestWithStrictJSON_AcceptsCleanData(t *testing.T) {
	t.Parallel()

	env := New(WithStrictJSON()).WrapRaw(map[string]any{"id": 1, "nested": map[string]any{"x": 2}})
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out) {
		t.Fatalf("invalid JSON: %s", out)
	}
}

func TestWithStrictJSON_DoesNotValidateLinks(t *testing.T) {
	t.Parallel()

	env := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	env.AddLink(Link{Rel: "self", Href: ""})
	if _, err := json.Marshal(env); err != nil {
		t.Fatalf("WithStrictJSON unexpectedly validated links: %v", err)
	}
}

func TestWithStrictJSON_SkipsEmptyObject(t *testing.T) {
	t.Parallel()

	env := New(WithStrictJSON()).WrapRaw(json.RawMessage(`{}`))
	if _, err := json.Marshal(env); err != nil {
		t.Fatalf("empty object rejected: %v", err)
	}
}

func TestWithStrictJSON_InheritsIntoPermissiveChild(t *testing.T) {
	t.Parallel()

	parent := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	child := New().WrapRaw(json.RawMessage(`{"a":1,"a":2}`))
	parent.AddEmbedded("child", child)

	out, err := json.Marshal(parent)
	if !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v (output %s), want ErrDuplicateJSONKey", err, out)
	}
	if len(out) != 0 {
		t.Fatalf("error returned partial output: %s", out)
	}
	var dup *DuplicateKeyError
	if !errors.As(err, &dup) || dup.Key != "a" {
		t.Fatalf("error does not expose DuplicateKeyError: %v", err)
	}
}

func TestWithStrictJSON_StrictChildUnderPermissiveParent(t *testing.T) {
	t.Parallel()

	parent := New().WrapRaw(&embeddedTestUser{ID: 1})
	child := New(WithStrictJSON()).WrapRaw(json.RawMessage(`{"a":1,"a":2}`))
	parent.AddEmbedded("child", child)

	if _, err := json.Marshal(parent); !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v, want ErrDuplicateJSONKey", err)
	}
}

func TestWithStrictJSON_InheritanceIsTransitive(t *testing.T) {
	t.Parallel()

	root := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	middle := New().WrapRaw(&embeddedTestUser{ID: 2})
	leaf := New().WrapRaw(json.RawMessage(`{"a":1,"a":2}`))
	middle.AddEmbedded("leaf", leaf)
	root.AddEmbedded("middle", middle)

	if _, err := json.Marshal(root); !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v, want ErrDuplicateJSONKey", err)
	}

	// The failed parent marshal must not mutate the leaf's permissive policy.
	if _, err := json.Marshal(leaf); err != nil {
		t.Fatalf("child was mutated by parent marshal: %v", err)
	}
}

func TestWithStrictJSON_NilAndEmptyChildData(t *testing.T) {
	t.Parallel()

	parent := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("nil", New().WrapRaw(nil))
	parent.AddEmbedded("empty", New().WrapRaw(json.RawMessage(`{}`)))

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatalf("nil/empty child data rejected: %v", err)
	}
	if !json.Valid(b) {
		t.Fatalf("invalid JSON: %s", b)
	}
}

func TestWithStrictJSON_SiblingIsolation(t *testing.T) {
	t.Parallel()

	parent := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("clean", New().WrapRaw(json.RawMessage(`{"ok":1}`)))
	parent.AddEmbedded("bad", New().WrapRaw(json.RawMessage(`{"a":1,"a":2}`)))

	if _, err := json.Marshal(parent); !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v, want ErrDuplicateJSONKey", err)
	}
}

func TestWithStrictJSON_InheritsIntoPrecomputedChildren(t *testing.T) {
	t.Parallel()

	parent := New(WithStrictJSON()).WrapPrecomputed(
		context.Background(),
		&embeddedTestUser{ID: 1},
		[]byte(`{"self":{"href":"/x"}}`),
	)
	child := New().WrapPrecomputed(
		context.Background(),
		json.RawMessage(`{"a":1,"a":2}`),
		[]byte(`{"self":{"href":"/child"}}`),
	)
	parent.AddEmbedded("child", child)

	if _, err := json.Marshal(parent); !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v, want ErrDuplicateJSONKey", err)
	}
}

func TestWithStrictJSON_ScansRawEmbeddedValues(t *testing.T) {
	t.Parallel()

	values := map[string]any{
		"raw":       json.RawMessage(`{"a":1,"a":2}`),
		"marshaler": duplicateMarshaler{},
		"nested":    json.RawMessage(`{"outer":{"a":1,"a":2}}`),
		"raw-map":   map[string]json.RawMessage{"nested": json.RawMessage(`{"a":1,"a":2}`)},
		"escaped":   json.RawMessage(`{"a":1,"\u0061":2}`),
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			parent := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
			parent.AddEmbedded("child", value)

			out, err := json.Marshal(parent)
			if !errors.Is(err, ErrDuplicateJSONKey) {
				t.Fatalf("error = %v (output %s), want ErrDuplicateJSONKey", err, out)
			}
			if len(out) != 0 {
				t.Fatalf("error returned partial output: %s", out)
			}
		})
	}
}

func TestWithStrictJSON_RawEmbeddedErrorContext(t *testing.T) {
	t.Parallel()

	parent := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("child", json.RawMessage(`{"a":1,"a":2}`))

	_, err := json.Marshal(parent)
	if !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v, want ErrDuplicateJSONKey", err)
	}
	if !strings.Contains(err.Error(), `hal: embedded relation "child": hal: embedded resource:`) {
		t.Fatalf("error = %q, want relation and resource context", err)
	}
	var dup *DuplicateKeyError
	if !errors.As(err, &dup) || dup.Key != "a" {
		t.Fatalf("error does not expose DuplicateKeyError: %v", err)
	}
	if dup.Offset != 7 {
		t.Fatalf("Offset = %d, want 7 (relative to the embedded value)", dup.Offset)
	}
}

func TestWithStrictJSON_RawEmbeddedArrayIndexContext(t *testing.T) {
	t.Parallel()

	parent := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("children", []json.RawMessage{
		json.RawMessage(`{"ok":1}`),
		json.RawMessage(`{"a":1,"a":2}`),
	})

	_, err := json.Marshal(parent)
	if !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v, want ErrDuplicateJSONKey", err)
	}
	if !strings.Contains(err.Error(), "embedded element 1") {
		t.Fatalf("error = %q, want array element index", err)
	}
}

func TestWithStrictJSON_RawScanRunsAfterObjectCheck(t *testing.T) {
	t.Parallel()

	parent := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("scalar", scalarMarshaler{})

	_, err := json.Marshal(parent)
	if !errors.Is(err, ErrInvalidEmbedded) {
		t.Fatalf("error = %v, want ErrInvalidEmbedded", err)
	}
	if errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("non-object value was scanned as an object: %v", err)
	}
}

func TestWithStrictJSON_DefaultRawEmbeddedRemainsPermissive(t *testing.T) {
	t.Parallel()

	parent := New().WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("child", json.RawMessage(`{"a":1,"a":2}`))

	b, err := json.Marshal(parent)
	if err != nil {
		t.Fatalf("default marshal rejected raw embedded duplicate: %v", err)
	}
	if !strings.Contains(string(b), `{"a":1,"a":2}`) {
		t.Fatalf("default output changed raw embedded bytes: %s", b)
	}
}

func TestWithStrictJSON_RawScanIsTransitive(t *testing.T) {
	t.Parallel()

	root := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	middle := New().WrapRaw(&embeddedTestUser{ID: 2})
	middle.AddEmbedded("leaf", json.RawMessage(`{"a":1,"a":2}`))
	root.AddEmbedded("middle", middle)

	if _, err := json.Marshal(root); !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v, want ErrDuplicateJSONKey", err)
	}
}

func TestWithStrictJSON_HiddenEnvelopeInsideRawMapIsScanned(t *testing.T) {
	t.Parallel()

	hidden := New().WrapRaw(json.RawMessage(`{"a":1,"a":2}`))

	parent := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("hidden", map[string]any{"resource": hidden})

	out, err := json.Marshal(parent)
	if !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v (output %s), want ErrDuplicateJSONKey", err, out)
	}
	if len(out) != 0 {
		t.Fatalf("error returned partial output: %s", out)
	}
}

func TestWithStrictJSON_HiddenStrictEnvelopePreservesDuplicateCause(t *testing.T) {
	t.Parallel()

	hidden := New(WithStrictJSON()).WrapRaw(json.RawMessage(`{"a":1,"a":2}`))

	parent := New().WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("hidden", map[string]any{"resource": hidden})

	out, err := json.Marshal(parent)
	if !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v (output %s), want ErrDuplicateJSONKey", err, out)
	}
	if !errors.Is(err, ErrInvalidEmbedded) {
		t.Fatalf("error = %v, want ErrInvalidEmbedded context", err)
	}
	if len(out) != 0 {
		t.Fatalf("error returned partial output: %s", out)
	}
}

func BenchmarkMarshal_StrictEmbedded(b *testing.B) {
	parent := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("child", json.RawMessage(`{"id":2,"name":"clean"}`))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(parent); err != nil {
			b.Fatal(err)
		}
	}
}

func TestWithStrictJSON_RepeatedAndConcurrentMarshal(t *testing.T) {
	t.Parallel()

	parent := New(WithStrictJSON()).WrapRaw(&embeddedTestUser{ID: 1})
	parent.AddEmbedded("child", New().WrapRaw(json.RawMessage(`{"ok":1}`)))

	first, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		got, err := json.Marshal(parent)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(first) {
			t.Fatalf("repeated marshal changed output:\n%s\n%s", first, got)
		}
	}

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := json.Marshal(parent); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent marshal failed: %v", err)
	}
}
