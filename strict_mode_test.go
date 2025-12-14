// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"testing"
)

func TestStrictMode_ValueVsPointerPanics(t *testing.T) {
	type User struct{}

	inst := New(WithStrictMode())
	RegisterInstance(inst, func(ctx context.Context, u *User) []Link {
		return nil
	})

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic in strict mode")
		}
	}()

	inst.Wrap(context.Background(), User{})
}

func TestStrictMode_NoGeneratorPanics(t *testing.T) {
	type Unknown struct{}

	inst := New(WithStrictMode())
	// No registration for Unknown

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when no generator exists in strict mode")
		}
	}()

	inst.Wrap(context.Background(), &Unknown{})
}
