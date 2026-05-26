// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemagen

import (
	"bytes"
	"encoding/json"
	"errors"

	"gopkg.in/yaml.v3"
)

type schemaElement interface {
	setIsPointer(value bool)
	setDescription(description string)
	setOptional(value bool)
}

type schemaObject interface {
	AddProperty(name string, property schemaElement)
	AddEmbedded(element schemaElement)
}

type baseSchemaElement struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	IsPointer   bool   `json:"x-pointer,omitempty" yaml:"x-pointer,omitempty"`
	IsOptional  bool   `json:"x-optional,omitempty" yaml:"x-optional,omitempty"`
}

func (b *baseSchemaElement) setIsPointer(value bool) {
	b.IsPointer = value
}

func (b *baseSchemaElement) setDescription(value string) {
	b.Description = value
}

func (b *baseSchemaElement) setOptional(value bool) {
	b.IsOptional = value
}

type refSchemaElement struct {
	baseSchemaElement `json:",inline" yaml:",inline"`
	Ref               string `json:"$ref" yaml:"$ref"`
}

type fieldSchemaElement struct {
	baseSchemaElement `json:",inline" yaml:",inline"`
	ElementType       schemaType `json:"type,omitempty" yaml:"type,omitempty"`
	CustomElementType string     `json:"x-customType,omitempty" yaml:"x-customType,omitempty"`
	Format            string     `json:"format,omitempty" yaml:"format,omitempty"`
}

type arraySchemaElement struct {
	fieldSchemaElement `json:",inline" yaml:",inline"`
	Items              schemaElement `json:"items" yaml:"items"`
}

type objectSchemaElement struct {
	schemaObject         `json:"-" yaml:"-"`
	fieldSchemaElement   `json:",inline" yaml:",inline"`
	Properties           map[string]schemaElement `json:"properties,omitempty" yaml:"properties,omitempty"`
	AdditionalProperties schemaElement            `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
	AllOf                []schemaElement          `json:"allOf,omitempty" yaml:"allOf,omitempty"`
}

func (s *objectSchemaElement) AddProperty(name string, property schemaElement) {
	if s.Properties == nil {
		s.Properties = make(map[string]schemaElement)
	}
	s.Properties[name] = property
}

func (s *objectSchemaElement) AddEmbedded(element schemaElement) {
	// prevent duplicates
	if re, ok := element.(*refSchemaElement); ok {
		ref := re.Ref
		for _, refEl := range s.AllOf {
			if r, ok := refEl.(*refSchemaElement); ok && r.Ref == ref {
				return
			}
		}
	}
	s.AllOf = append(s.AllOf, element)
}

type defsSchemaElement map[string]schemaElement

func (d defsSchemaElement) AddDef(name string, property schemaElement) {
	d[name] = property
}

type Schema struct {
	Defs                defsSchemaElement `json:"$defs,omitempty" yaml:"$defs,omitempty"`
	objectSchemaElement `json:",inline" yaml:",inline"`
}

func (s *Schema) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

func (s *Schema) ToYAML() ([]byte, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)

	enc.SetIndent(2)

	if err := enc.Encode(s); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func createSchema() *Schema {
	return &Schema{
		Defs: defsSchemaElement{},
	}
}

func createSimpleField(fieldType schemaType, description string) *fieldSchemaElement {
	return &fieldSchemaElement{
		baseSchemaElement: baseSchemaElement{
			Description: description,
		},
		ElementType: fieldType,
	}
}

func createArrayField(itemType schemaElement, description string) *arraySchemaElement {
	return &arraySchemaElement{
		fieldSchemaElement: fieldSchemaElement{
			baseSchemaElement: baseSchemaElement{
				Description: description,
			},
			ElementType: schemaTypeArray,
		},
		Items: itemType,
	}
}

func createRefField(ref, description string) *refSchemaElement {
	return &refSchemaElement{
		baseSchemaElement: baseSchemaElement{
			Description: description,
		},
		Ref: ref,
	}
}

func createObjectField(description string) *objectSchemaElement {
	return &objectSchemaElement{
		fieldSchemaElement: fieldSchemaElement{
			baseSchemaElement: baseSchemaElement{
				Description: description,
			},
			ElementType: schemaTypeObject,
		},
		Properties: make(map[string]schemaElement),
	}
}

func createMapField(valueType schemaElement, description string) *objectSchemaElement {
	return &objectSchemaElement{
		fieldSchemaElement: fieldSchemaElement{
			baseSchemaElement: baseSchemaElement{
				Description: description,
			},
			ElementType: schemaTypeObject,
		},
		AdditionalProperties: valueType,
	}
}

type schemaType string

const (
	schemaTypeObject  schemaType = "object"
	schemaTypeArray   schemaType = "array"
	schemaTypeString  schemaType = "string"
	schemaTypeInteger schemaType = "integer"
	schemaTypeNumber  schemaType = "number"
	schemaTypeBoolean schemaType = "boolean"
	schemaTypeAny     schemaType = ""
	schemaTypeUnknown schemaType = "-"
)

func mergeSchemas(base schemaObject, additional schemaElement) error {
	if objectElement, ok := additional.(*objectSchemaElement); ok {
		for name, prop := range objectElement.Properties {
			base.AddProperty(name, prop)
		}
		for _, el := range objectElement.AllOf {
			base.AddEmbedded(el)
		}
		return nil
	}
	return errors.New("cannot merge non-object schema elements")
}
