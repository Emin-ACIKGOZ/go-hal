// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package openapi

import (
	"github.com/getkin/kin-openapi/openapi3"
)

// LinkSchemaName is the key used in the Components.Schemas map for the HAL Link object.
const (
	LinkSchemaName = "Link"
)

// Adapter helps augment an OpenAPI 3.0 document with HAL semantics.
type Adapter struct {
	doc *openapi3.T
}

// New creates a new HAL OpenAPI adapter.
func New(doc *openapi3.T) *Adapter {
	return &Adapter{doc: doc}
}

// InjectLinkSchema adds the standard HAL Link object definition to Components.Schemas.
// It must be called before creating HAL resources to ensure the $ref exists.
// An existing custom Link schema is preserved.
func (a *Adapter) InjectLinkSchema() {
	if a == nil || a.doc == nil {
		return
	}
	if a.doc.Components == nil {
		a.doc.Components = &openapi3.Components{}
	}
	if a.doc.Components.Schemas == nil {
		a.doc.Components.Schemas = make(openapi3.Schemas)
	}
	if link, ok := a.doc.Components.Schemas[LinkSchemaName]; ok && link != nil {
		return
	}
	a.doc.Components.Schemas[LinkSchemaName] =
		openapi3.NewSchemaRef("", newLinkSchema())
}

func newLinkSchema() *openapi3.Schema {
	linkSchema := openapi3.NewObjectSchema().
		WithProperty("href", openapi3.NewStringSchema()).
		WithProperty("templated", openapi3.NewBoolSchema()).
		WithProperty("type", openapi3.NewStringSchema()).
		WithProperty("deprecation", openapi3.NewStringSchema()).
		WithProperty("name", openapi3.NewStringSchema()).
		WithProperty("profile", openapi3.NewStringSchema()).
		WithProperty("title", openapi3.NewStringSchema()).
		WithProperty("hreflang", openapi3.NewStringSchema()).
		WithProperty("method", openapi3.NewStringSchema()) // Hint
	linkSchema.Required = []string{"href"}
	return linkSchema
}

// MakeResource augments a schema to include HAL fields (_links, _embedded).
// It modifies the schema in place to allow existing properties (the "Data")
// to coexist with HAL fields.
func (a *Adapter) MakeResource(schema *openapi3.Schema) {
	if schema == nil {
		return
	}
	if schema.Properties == nil {
		schema.Properties = make(openapi3.Schemas)
	}
	linkRef := a.linkSchemaRef()

	// 1. Add _links
	oneOfSchema := &openapi3.Schema{
		OneOf: []*openapi3.SchemaRef{
			linkRef,
			{
				Value: &openapi3.Schema{
					Type:  &openapi3.Types{openapi3.TypeArray},
					Items: a.linkSchemaRef(),
				},
			},
		},
	}

	linksSchema := openapi3.NewObjectSchema()
	linksSchema.ReadOnly = true
	linksSchema.AdditionalProperties = openapi3.AdditionalProperties{
		Schema: &openapi3.SchemaRef{
			Value: oneOfSchema,
		},
	}
	schema.Properties["_links"] = openapi3.NewSchemaRef("", linksSchema)

	// 2. Add _embedded
	embeddedSchema := openapi3.NewObjectSchema()
	embeddedSchema.ReadOnly = true
	embeddedSchema.WithAnyAdditionalProperties()
	schema.Properties["_embedded"] = openapi3.NewSchemaRef("", embeddedSchema)
}

// linkSchemaRef returns a reference to the adapter document's Link schema. When
// there is no document to hold components, it inlines the standard schema.
func (a *Adapter) linkSchemaRef() *openapi3.SchemaRef {
	if a != nil && a.doc != nil {
		a.InjectLinkSchema()
		if link, ok := a.doc.Components.Schemas[LinkSchemaName]; ok && link != nil {
			return openapi3.NewSchemaRef("#/components/schemas/"+LinkSchemaName, link.Value)
		}
	}
	return openapi3.NewSchemaRef("", newLinkSchema())
}

// MakeCollection creates a new HAL Collection Schema wrapping the provided item schema.
// It returns a standard structure:
//
//	{
//	  _links: { ... },
//	  _embedded: { "items": [ itemSchema... ] },
//	  count: int,
//	  total: int
//	}
func (a *Adapter) MakeCollection(itemSchemaRef *openapi3.SchemaRef) *openapi3.Schema {
	collection := openapi3.NewObjectSchema()

	// Reuse MakeResource to inject _links and basic _embedded structure
	a.MakeResource(collection)

	// Explicitly define the standard "_embedded.items" list
	embeddedItems := openapi3.NewObjectSchema()
	embeddedItems.ReadOnly = true
	itemsArray := &openapi3.Schema{
		Type:  &openapi3.Types{openapi3.TypeArray},
		Items: normalizeItemSchemaRef(itemSchemaRef),
	}
	embeddedItems.WithProperty("items", itemsArray)

	collection.Properties["_embedded"] = openapi3.NewSchemaRef("", embeddedItems)
	collection.WithProperty("count", readOnlyIntegerSchema())
	collection.WithProperty("total", readOnlyIntegerSchema())

	return collection
}

func normalizeItemSchemaRef(ref *openapi3.SchemaRef) *openapi3.SchemaRef {
	if ref == nil || (ref.Ref == "" && ref.Value == nil) {
		return openapi3.NewSchemaRef("", openapi3.NewObjectSchema())
	}
	return ref
}

func readOnlyIntegerSchema() *openapi3.Schema {
	schema := openapi3.NewIntegerSchema()
	schema.ReadOnly = true
	return schema
}
