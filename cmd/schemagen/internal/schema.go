// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

const Version = "https://json-schema.org/draft/2020-12/schema"

type SchemaElement interface {
	setIsPointer(value bool)
	setDescription(description string)
}

type SchemaObject interface {
	AddProperty(name string, property SchemaElement)
	AddEmbeddedRef(ref string)
}

type BaseSchemaElement struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	IsPointer   bool   `json:"x-pointer,omitempty" yaml:"x-pointer,omitempty"`
}

func (b *BaseSchemaElement) setIsPointer(value bool) {
	b.IsPointer = value
}

func (b *BaseSchemaElement) setDescription(value string) {
	b.Description = value
}

type RefSchemaElement struct {
	BaseSchemaElement `json:",inline" yaml:",inline"`
	Ref               string `json:"$ref" yaml:"$ref"`
}

type FieldSchemaElement struct {
	BaseSchemaElement `json:",inline" yaml:",inline"`
	ElementType       SchemaType `json:"type,omitempty" yaml:"type,omitempty"`
	CustomElementType string     `json:"x-customType,omitempty" yaml:"x-customType,omitempty"`
}

type ArraySchemaElement struct {
	FieldSchemaElement `json:",inline" yaml:",inline"`
	Items              SchemaElement `json:"items" yaml:"items"`
}
type ObjectSchemaElement struct {
	SchemaObject         `json:"-" yaml:"-"`
	FieldSchemaElement   `json:",inline" yaml:",inline"`
	Properties           map[string]SchemaElement `json:"properties,omitempty" yaml:"properties,omitempty"`
	AdditionalProperties SchemaElement            `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
	AllOf                []SchemaElement          `json:"allOf,omitempty" yaml:"allOf,omitempty"`
}

func (s *ObjectSchemaElement) AddProperty(name string, property SchemaElement) {
	if s.Properties == nil {
		s.Properties = make(map[string]SchemaElement)
	}
	s.Properties[name] = property
}

func (s *ObjectSchemaElement) AddEmbeddedRef(ref string) {
	s.AllOf = append(s.AllOf, CreateRefField(ref, ""))
}

type DefsSchemaElement map[string]SchemaElement

func (d DefsSchemaElement) AddDef(name string, property SchemaElement) {
	d[name] = property
}

type Schema struct {
	Schema              string            `json:"$schema" yaml:"$schema"`
	ID                  string            `json:"id" yaml:"id"`
	Title               string            `json:"title" yaml:"title"`
	Defs                DefsSchemaElement `json:"$defs,omitempty" yaml:"$defs,omitempty"`
	ObjectSchemaElement `json:",inline" yaml:",inline"`
}

func (s *Schema) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

func (s *Schema) ToYAML() ([]byte, error) {
	return yaml.Marshal(s)
}

func CreateSchema(id, title, description string) *Schema {
	return &Schema{
		Schema: Version,
		ID:     id,
		Title:  title,
		Defs:   DefsSchemaElement{},
		ObjectSchemaElement: ObjectSchemaElement{
			FieldSchemaElement: FieldSchemaElement{
				BaseSchemaElement: BaseSchemaElement{
					Description: description,
				},
				ElementType: SchemaTypeObject,
			},
			Properties: make(map[string]SchemaElement),
		},
	}
}

func CreateSimpleField(fieldType SchemaType, description string) *FieldSchemaElement {
	return &FieldSchemaElement{
		BaseSchemaElement: BaseSchemaElement{
			Description: description,
		},
		ElementType: fieldType,
	}
}

func CreateArrayField(itemType SchemaElement, description string) *ArraySchemaElement {
	return &ArraySchemaElement{
		FieldSchemaElement: FieldSchemaElement{
			BaseSchemaElement: BaseSchemaElement{
				Description: description,
			},
			ElementType: SchemaTypeArray,
		},
		Items: itemType,
	}
}

func CreateRefField(ref, description string) *RefSchemaElement {
	return &RefSchemaElement{
		BaseSchemaElement: BaseSchemaElement{
			Description: description,
		},
		Ref: ref,
	}
}

func CreateObjectField(description string) *ObjectSchemaElement {
	return &ObjectSchemaElement{
		FieldSchemaElement: FieldSchemaElement{
			BaseSchemaElement: BaseSchemaElement{
				Description: description,
			},
			ElementType: SchemaTypeObject,
		},
		Properties: make(map[string]SchemaElement),
	}
}

func CreateMapField(valueType SchemaElement, description string) *ObjectSchemaElement {
	return &ObjectSchemaElement{
		FieldSchemaElement: FieldSchemaElement{
			BaseSchemaElement: BaseSchemaElement{
				Description: description,
			},
			ElementType: SchemaTypeObject,
		},
		AdditionalProperties: valueType,
	}
}

type SchemaType string

const (
	SchemaTypeObject  SchemaType = "object"
	SchemaTypeArray   SchemaType = "array"
	SchemaTypeString  SchemaType = "string"
	SchemaTypeInteger SchemaType = "integer"
	SchemaTypeNumber  SchemaType = "number"
	SchemaTypeBoolean SchemaType = "boolean"
	SchemaTypeNull    SchemaType = "null"
)
