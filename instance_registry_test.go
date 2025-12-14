// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"reflect"
	"testing"
)

func TestRegisterInstance_TypeKey(t *testing.T) {
	type User struct{}

	inst := New()
	RegisterInstance(inst, func(ctx context.Context, u *User) []Link {
		return nil
	})

	tp := reflect.TypeOf((*User)(nil))
	if _, ok := inst.lookupGenerator(tp); !ok {
		t.Fatal("generator not registered for *User")
	}
}

func TestRegisteredTypes(t *testing.T) {
	type A struct{}
	type B struct{}

	inst := New()
	RegisterInstance(inst, func(context.Context, *A) []Link { return nil })
	RegisterInstance(inst, func(context.Context, *B) []Link { return nil })

	types := inst.RegisteredTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
}
