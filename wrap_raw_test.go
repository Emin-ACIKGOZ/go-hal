// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"encoding/json"
	"testing"
)

func TestWrapRaw_Basic(t *testing.T) {
	type User struct {
		ID int `json:"id"`
	}

	inst := New()
	env := inst.WrapRaw(&User{ID: 42})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(b, `"id":42`) {
		t.Fatalf("expected data in output: %s", b)
	}
}

func TestWrapRaw_NilData(t *testing.T) {
	inst := New()
	env := inst.WrapRaw(nil)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != "{}" {
		t.Fatalf("expected empty object: %s", b)
	}
}