// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package openapi

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestInjectLinkSchema(t *testing.T) {
	doc := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: make(openapi3.Schemas),
		},
	}
	a := New(doc)

	// Defensive fix: ensure Components.Schemas is always initialized
	a.InjectLinkSchema()

	if _, ok := doc.Components.Schemas[LinkSchemaName]; !ok {
		t.Fatal("Link schema not injected")
	}
}

func TestMakeResource_AddsHALFields(t *testing.T) {
	doc := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: make(openapi3.Schemas),
		},
	}
	a := New(doc)
	a.InjectLinkSchema()

	schema := openapi3.NewObjectSchema()
	a.MakeResource(schema)

	if _, ok := schema.Properties["_links"]; !ok {
		t.Fatal("_links not added to schema")
	}
	if _, ok := schema.Properties["_embedded"]; !ok {
		t.Fatal("_embedded not added to schema")
	}
}

func TestMakeCollection(t *testing.T) {
	doc := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: make(openapi3.Schemas),
		},
	}
	a := New(doc)
	a.InjectLinkSchema()

	item := openapi3.NewSchemaRef("", openapi3.NewObjectSchema())
	c := a.MakeCollection(item)

	if _, ok := c.Properties["count"]; !ok {
		t.Fatal("count missing in collection schema")
	}
	if _, ok := c.Properties["_embedded"]; !ok {
		t.Fatal("_embedded missing in collection schema")
	}

	embedded := c.Properties["_embedded"].Value
	if embedded == nil {
		t.Fatal("_embedded schema Value is nil")
	}

	if _, ok := embedded.Properties["items"]; !ok {
		t.Fatal("_embedded.items missing in collection schema")
	}
}
