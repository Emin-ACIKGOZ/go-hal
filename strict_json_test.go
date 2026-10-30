// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

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
