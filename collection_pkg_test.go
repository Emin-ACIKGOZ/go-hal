// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"testing"
)

type collectionUser struct {
	ID int `json:"id"`
}

func TestCollection_PackageLevel(t *testing.T) {
	users := []*collectionUser{
		{ID: 1},
		{ID: 2},
	}

	coll := Collection(context.Background(), users, 10, Link{Rel: "self", Href: "/users"})
	b, err := json.Marshal(coll)
	if err != nil {
		t.Fatal(err)
	}

	assertJSONEqual(t, b, []byte(`{
		"_links":{"self":{"href":"/users"}},
		"_embedded":{"items":[{"id":1},{"id":2}]},
		"count":2,
		"total":10
	}`))
}

func TestCollection_Instance(t *testing.T) {
	inst := New()
	users := []*collectionUser{
		{ID: 1},
		{ID: 2},
	}

	coll := inst.Collection(context.Background(), users, 10, Link{Rel: "self", Href: "/users"})
	b, err := json.Marshal(coll)
	if err != nil {
		t.Fatal(err)
	}

	assertJSONEqual(t, b, []byte(`{
		"_links":{"self":{"href":"/users"}},
		"_embedded":{"items":[{"id":1},{"id":2}]},
		"count":2,
		"total":10
	}`))
}
