// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

type Parser struct {
	config    *Config
	schema    *Schema
	typeSpecs map[string]*ast.TypeSpec
}

func NewParser(cfg *Config) *Parser {
	return &Parser{
		config:    cfg,
		typeSpecs: make(map[string]*ast.TypeSpec),
	}
}

func (p *Parser) Parse() (*Schema, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, p.config.FilePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	err = p.initializeSchema(file)
	if err != nil {
		return nil, err
	}
	p.collectTypeSpecs(file)

	if err := p.parseTypes(); err != nil {
		return nil, err
	}

	return p.schema, nil
}

func (p *Parser) initializeSchema(file *ast.File) error {
	id, err := GetSchemaID(file, p.config)
	if err != nil {
		if p.config.SchemaIDPrefix == "" {
			return fmt.Errorf("could not determine schema ID: %w", err)
		}
		id = p.config.SchemaIDPrefix
	}
	p.schema = CreateSchema(id, p.config.RootTypeName, "")
	return nil
}

func (p *Parser) collectTypeSpecs(file *ast.File) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			p.typeSpecs[typeSpec.Name.Name] = typeSpec
		}
	}
	if _, ok := p.typeSpecs[p.config.RootTypeName]; !ok {
		fmt.Printf("Warning: Root type %s not found among collected type specs\n", p.config.RootTypeName)
	}
}

func (p *Parser) parseTypes() error {
	for name, typeSpec := range p.typeSpecs {
		err := p.parseType(name, typeSpec)
		if err != nil {
			return fmt.Errorf("parse type spec %s: %w", name, err)
		}
	}

	return nil
}

func (p *Parser) isRootType(name string) bool {
	return name == p.config.RootTypeName || len(p.typeSpecs) == 1
}

func (p *Parser) parseType(name string, typeSpec *ast.TypeSpec) error {
	schemaElement, err := p.parseExpr(typeSpec.Type)
	if err != nil {
		return err
	}
	if p.isRootType(name) {
		obj, _ := schemaElement.(*ObjectSchemaElement)
		p.schema.ObjectSchemaElement = *obj
	} else {
		p.schema.Defs.AddDef(name, schemaElement)
	}

	return nil
}

func (p *Parser) parseExpr(expr ast.Expr) (SchemaElement, error) {
	switch t := expr.(type) {
	case *ast.ArrayType:
		return p.parseArray(t)
	case *ast.Ident:
		return p.parseIdent(t)
	case *ast.StructType:
		return p.parseStruct(t)
	case *ast.MapType:
		return p.parseMap(t)
	case *ast.StarExpr:
		return p.parsePointer(t)
	case *ast.SelectorExpr:
		return p.parseSelector(t)
	}

	return nil, errors.New("unrecognized field type")
}

func (p *Parser) parseStruct(structType *ast.StructType) (SchemaElement, error) {
	var schemaObject SchemaObject = CreateObjectField("")
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			if err := p.addEmbeddedField(field, schemaObject); err != nil {
				return nil, err
			}
			continue
		}
		p.addNamedFields(field, schemaObject)
	}

	return schemaObject.(SchemaElement), nil
}

func (p *Parser) addEmbeddedField(field *ast.Field, schemaObject SchemaObject) error {
	ident, ok := field.Type.(*ast.Ident)
	if !ok {
		selector, ok := field.Type.(*ast.SelectorExpr)
		if ok {
			fmt.Printf("Warning: skipping embedded field with external type \"%s.%s\"\n", selector.X, selector.Sel)
			return nil
		}
		return errors.New("unrecognized embedded field type ")
	}

	typeName := ident.Name
	if _, exists := p.typeSpecs[typeName]; !exists {
		return fmt.Errorf("type %s not found in collected type specs", typeName)
	}
	schemaObject.AddEmbeddedRef("#/$defs/" + typeName)
	return nil
}

func (p *Parser) addNamedFields(field *ast.Field, schemaObject SchemaObject) {
	for _, ident := range field.Names {
		if ident.Name == "_" {
			continue
		}
		p.addNamedField(ident, field, schemaObject)
	}
}

func (p *Parser) addNamedField(ident *ast.Ident, field *ast.Field, schemaObject SchemaObject) {
	fieldName := ident.Name
	if name, ok := ExtractNameFromTag(field); ok {
		fieldName = name
	}

	element, err := p.parseExpr(field.Type)
	if err != nil {
		fmt.Printf("Error parsing field %s: %v\n", fieldName, err)
		return
	}

	if description, ok := ExtractDescriptionFromComment(field.Doc); ok {
		element.setDescription(description)
	}

	schemaObject.AddProperty(fieldName, element)
}

func (p *Parser) parseArray(array *ast.ArrayType) (SchemaElement, error) {
	itemSchema, err := p.parseExpr(array.Elt)
	if err != nil {
		return nil, err
	}
	return CreateArrayField(itemSchema, ""), nil
}

func (p *Parser) parseIdent(ident *ast.Ident) (SchemaElement, error) {
	if ident.Obj == nil {
		element := CreateSimpleField(goPrimitiveToSchemaType(ident.Name), "")
		if element.ElementType == "" {
			element.CustomElementType = "any"
		}
		return element, nil
	}

	typeSpec, ok := ident.Obj.Decl.(*ast.TypeSpec)
	if !ok {
		return nil, errors.New("unrecognized Ident declaration type")
	}

	typeName := typeSpec.Name.Name
	if _, exists := p.typeSpecs[typeName]; !exists {
		return nil, fmt.Errorf("type %s not found in collected type specs", typeName)
	}
	return CreateRefField("#/$defs/"+typeName, ""), nil
}

func (p *Parser) parseMap(m *ast.MapType) (SchemaElement, error) {
	valueSchema, err := p.parseExpr(m.Value)
	if err != nil {
		return nil, err
	}
	return CreateMapField(valueSchema, ""), nil
}

func (p *Parser) parsePointer(pointer *ast.StarExpr) (SchemaElement, error) {
	element, err := p.parseExpr(pointer.X)
	if err != nil {
		return nil, err
	}
	element.setIsPointer(true)
	return element, nil
}

func (*Parser) parseSelector(selector *ast.SelectorExpr) (SchemaElement, error) {
	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil, errors.New("unrecognized SelectorExpr structure")
	}

	fullTypeName := pkgIdent.Name + "." + selector.Sel.Name
	element := CreateSimpleField(SchemaTypeString, "")
	element.CustomElementType = fullTypeName
	return element, nil
}
