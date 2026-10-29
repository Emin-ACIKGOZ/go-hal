// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package openapi

import (
	"context"
	"reflect"
	"strings"
	"testing"

	hal "github.com/Emin-ACIKGOZ/go-hal"
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

func linkJSONProperties(t *testing.T) map[string]bool {
	t.Helper()

	typ := reflect.TypeOf(hal.Link{})
	properties := make(map[string]bool, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			continue
		}
		properties[name] = true
	}
	return properties
}

func TestInjectLinkSchema_CompletesLinkFields(t *testing.T) {
	doc := &openapi3.T{} // Also exercises nil Components and Schemas.
	a := New(doc)
	a.InjectLinkSchema()

	ref, ok := doc.Components.Schemas[LinkSchemaName]
	if !ok {
		t.Fatal("Link schema not injected")
	}
	schema := ref.Value
	if schema == nil {
		t.Fatal("Link schema Value is nil")
	}

	foundHref := false
	for _, required := range schema.Required {
		if required == "href" {
			foundHref = true
		}
	}
	if !foundHref {
		t.Fatalf("href is not required: %v", schema.Required)
	}

	want := linkJSONProperties(t)
	for name := range want {
		if _, ok := schema.Properties[name]; !ok {
			t.Errorf("Link schema is missing %q", name)
		}
	}
	for name := range schema.Properties {
		if !want[name] {
			t.Errorf("Link schema has unexpected property %q", name)
		}
	}

	if got := schema.Properties["templated"].Value.Type; got == nil || !got.Is(openapi3.TypeBoolean) {
		t.Errorf("templated type = %v, want boolean", got)
	}
	for _, name := range []string{"href", "type", "deprecation", "name", "profile", "title", "hreflang", "method"} {
		if got := schema.Properties[name].Value.Type; got == nil || !got.Is(openapi3.TypeString) {
			t.Errorf("%s type = %v, want string", name, got)
		}
	}
}

func TestInjectLinkSchema_Idempotent(t *testing.T) {
	doc := &openapi3.T{
		Components: &openapi3.Components{Schemas: make(openapi3.Schemas)},
	}
	doc.Components.Schemas["Existing"] = openapi3.NewSchemaRef("", openapi3.NewStringSchema())
	a := New(doc)

	a.InjectLinkSchema()
	first := doc.Components.Schemas[LinkSchemaName].Value
	a.InjectLinkSchema()
	second := doc.Components.Schemas[LinkSchemaName].Value

	if _, ok := doc.Components.Schemas["Existing"]; !ok {
		t.Fatal("injection dropped an existing schema")
	}
	if first == nil || second == nil {
		t.Fatal("Link schema Value is nil")
	}
	if len(second.Properties) != len(first.Properties) {
		t.Errorf("properties changed from %d to %d", len(first.Properties), len(second.Properties))
	}
	if len(second.Required) != 1 || second.Required[0] != "href" {
		t.Errorf("required = %v, want [href]", second.Required)
	}
}

func TestMakeResource_ShapeAndIdempotency(t *testing.T) {
	doc := &openapi3.T{}
	a := New(doc)
	a.InjectLinkSchema()

	schema := &openapi3.Schema{} // Also exercises nil Properties.
	schema.WithProperty("id", openapi3.NewIntegerSchema())

	a.MakeResource(schema)
	a.MakeResource(schema)

	if _, ok := schema.Properties["id"]; !ok {
		t.Fatal("MakeResource dropped an existing data property")
	}

	linksSchema := schema.Properties["_links"].Value
	if linksSchema == nil || linksSchema.Type == nil || !linksSchema.Type.Is(openapi3.TypeObject) {
		t.Fatalf("_links schema = %#v, want object", linksSchema)
	}
	valueSchema := linksSchema.AdditionalProperties.Schema.Value
	if valueSchema == nil || len(valueSchema.OneOf) != 2 {
		t.Fatalf("_links value schema = %#v, want oneOf Link or array", valueSchema)
	}
	if got := valueSchema.OneOf[0].Ref; got != "#/components/schemas/"+LinkSchemaName {
		t.Errorf("oneOf[0].Ref = %q", got)
	}
	arraySchema := valueSchema.OneOf[1].Value
	if arraySchema == nil || arraySchema.Type == nil || !arraySchema.Type.Is(openapi3.TypeArray) {
		t.Fatalf("oneOf[1] = %#v, want array", arraySchema)
	}
	if got := arraySchema.Items.Ref; got != "#/components/schemas/"+LinkSchemaName {
		t.Errorf("array items ref = %q", got)
	}

	embedded := schema.Properties["_embedded"].Value
	if embedded == nil || embedded.Type == nil || !embedded.Type.Is(openapi3.TypeObject) {
		t.Fatalf("_embedded schema = %#v, want object", embedded)
	}
}

func TestMakeCollection_ShapeAndItemRef(t *testing.T) {
	doc := &openapi3.T{}
	a := New(doc)
	a.InjectLinkSchema()

	itemRef := openapi3.NewSchemaRef("", openapi3.NewObjectSchema().WithProperty("id", openapi3.NewIntegerSchema()))
	collection := a.MakeCollection(itemRef)

	for _, name := range []string{"_links", "_embedded", "count", "total"} {
		if _, ok := collection.Properties[name]; !ok {
			t.Errorf("collection schema is missing %q", name)
		}
	}
	for _, name := range []string{"count", "total"} {
		if got := collection.Properties[name].Value.Type; got == nil || !got.Is(openapi3.TypeInteger) {
			t.Errorf("%s type = %v, want integer", name, got)
		}
	}

	embedded := collection.Properties["_embedded"].Value
	if embedded == nil || embedded.Type == nil || !embedded.Type.Is(openapi3.TypeObject) {
		t.Fatalf("_embedded schema = %#v, want object", embedded)
	}
	itemsRef := embedded.Properties["items"]
	if itemsRef == nil || itemsRef.Value == nil {
		t.Fatal("_embedded.items schema is nil")
	}
	if itemsRef.Value.Type == nil || !itemsRef.Value.Type.Is(openapi3.TypeArray) {
		t.Fatalf("_embedded.items type = %v, want array", itemsRef.Value.Type)
	}
	if itemsRef.Value.Items != itemRef {
		t.Error("_embedded.items did not preserve the item schema reference")
	}
}

func TestGeneratedDocumentValidates(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "HAL compliance", Version: "1.0.0"},
		Paths:   openapi3.NewPaths(),
	}
	a := New(doc)
	a.InjectLinkSchema()

	resource := openapi3.NewObjectSchema().WithProperty("id", openapi3.NewIntegerSchema())
	a.MakeResource(resource)
	doc.Components.Schemas["User"] = openapi3.NewSchemaRef("", resource)

	collection := a.MakeCollection(openapi3.NewSchemaRef("#/components/schemas/User", resource))
	doc.Components.Schemas["UserCollection"] = openapi3.NewSchemaRef("", collection)

	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("generated OpenAPI document is invalid: %v", err)
	}
}
