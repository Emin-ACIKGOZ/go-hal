// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"encoding/json"
	"errors"
	"testing"
)

func newEmptyEmbeddedEnvelope() *Envelope {
	return &Envelope{Data: nil, instance: New(), links: map[string]any{}}
}

func TestEmbeddedDepth_SelfCycleRejected(t *testing.T) {
	t.Parallel()

	env := newEmptyEmbeddedEnvelope()
	env.AddEmbedded("self", env)

	b, err := json.Marshal(env)
	if !errors.Is(err, ErrEmbeddedDepthExceeded) {
		t.Fatalf("error = %v (output %s), want ErrEmbeddedDepthExceeded", err, b)
	}
	if len(b) != 0 {
		t.Fatalf("error returned partial output: %s", b)
	}
}

func TestEmbeddedDepth_TwoNodeCycleRejected(t *testing.T) {
	t.Parallel()

	a := newEmptyEmbeddedEnvelope()
	b := newEmptyEmbeddedEnvelope()
	a.AddEmbedded("b", b)
	b.AddEmbedded("a", a)

	out, err := json.Marshal(a)
	if !errors.Is(err, ErrEmbeddedDepthExceeded) {
		t.Fatalf("error = %v (output %s), want ErrEmbeddedDepthExceeded", err, out)
	}
	if len(out) != 0 {
		t.Fatalf("error returned partial output: %s", out)
	}
}

func TestEmbeddedDepth_MaximumAllowed(t *testing.T) {
	t.Parallel()

	root := newEmptyEmbeddedEnvelope()
	current := root
	// top-level depth 0; add maxEmbeddedDepth children so the deepest is 32.
	for i := 0; i < maxEmbeddedDepth; i++ {
		child := newEmptyEmbeddedEnvelope()
		child.AddEmbedded("value", map[string]any{"depth": i})
		current.AddEmbedded("next", child)
		current = child
	}

	b, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("depth %d rejected: %v", maxEmbeddedDepth, err)
	}
	if !json.Valid(b) {
		t.Fatalf("invalid JSON at maximum depth: %s", b)
	}
}

func TestEmbeddedDepth_ExceededRejected(t *testing.T) {
	t.Parallel()

	root := newEmptyEmbeddedEnvelope()
	current := root
	for i := 0; i < maxEmbeddedDepth+1; i++ {
		child := newEmptyEmbeddedEnvelope()
		child.AddEmbedded("value", map[string]any{"depth": i})
		current.AddEmbedded("next", child)
		current = child
	}

	b, err := json.Marshal(root)
	if !errors.Is(err, ErrEmbeddedDepthExceeded) {
		t.Fatalf("error = %v (output %s), want ErrEmbeddedDepthExceeded", err, b)
	}
	if len(b) != 0 {
		t.Fatalf("error returned partial output: %s", b)
	}
}

func TestEmbeddedDepth_AncestorScopeIsPerMarshal(t *testing.T) {
	t.Parallel()

	shared := newEmptyEmbeddedEnvelope()
	shared.AddEmbedded("value", map[string]any{"ok": true})

	root := newEmptyEmbeddedEnvelope()
	root.AddEmbedded("first", shared)
	root.AddEmbedded("second", shared)

	b, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("reusing a shared child across siblings failed: %v", err)
	}
	embedded := decodeJSONValue(t, b).(map[string]any)["_embedded"].(map[string]any)
	for _, rel := range []string{"first", "second"} {
		child, ok := embedded[rel].(map[string]any)
		if !ok {
			t.Fatalf("%s has type %T, want object", rel, embedded[rel])
		}
		if _, ok := child["_embedded"].(map[string]any); !ok {
			t.Fatalf("%s lost its embedded value", rel)
		}
	}
}
