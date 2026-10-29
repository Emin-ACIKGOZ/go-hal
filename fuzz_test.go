// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"unicode/utf8"
)

func FuzzMarshal_WithRegisteredGenerator(f *testing.F) {
	f.Fuzz(func(t *testing.T, id int, name string) {
		if !utf8.ValidString(name) {
			t.Skip()
		}

		type User struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}

		inst := New()
		RegisterInstance(inst, func(_ context.Context, u *User) []Link {
			return []Link{{Rel: "self", Href: "/users/" + strconv.Itoa(u.ID)}}
		})

		env := inst.Wrap(context.Background(), &User{ID: id, Name: name})
		out, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var result map[string]any
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if result == nil {
			t.Fatal("expected result to not be nil")
		}
	})
}

func FuzzMarshal_WithoutGenerator(f *testing.F) {
	f.Fuzz(func(t *testing.T, id int) {
		type User struct {
			ID int `json:"id"`
		}

		inst := New()
		env := inst.Wrap(context.Background(), &User{ID: id})
		out, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var result map[string]any
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if result == nil {
			t.Fatal("expected result to not be nil")
		}
	})
}

func FuzzLink_AddLink(f *testing.F) {
	f.Fuzz(func(t *testing.T, rel, href string) {
		if rel == "" || href == "" {
			t.Skip()
		}
		if !utf8.ValidString(rel) || !utf8.ValidString(href) {
			t.Skip()
		}

		e := &Envelope{
			Data:     nil,
			instance: New(),
			links:    make(map[string]any),
		}
		e.AddLink(Link{Rel: rel, Href: href})

		if e.links[rel] == nil {
			t.Fatalf("link %s not added", rel)
		}
	})
}
