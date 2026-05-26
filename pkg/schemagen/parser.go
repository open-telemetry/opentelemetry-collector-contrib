// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemagen

import (
	"container/list"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/iancoleman/strcase"
	"golang.org/x/tools/go/packages"
)

type Parser struct {
	config       *Config
	schema       *Schema
	types        map[string]*typeInfo
	processQueue *list.List
	pkg          *packages.Package
	current      *typeInfo
}

type typeInfo struct {
	spec      *ast.TypeSpec
	comms     []*ast.CommentGroup
	imports   map[string]string
	typeName  string
	processed bool
}

func NewParser(cfg *Config) *Parser {
	return &Parser{
		config:       cfg,
		types:        make(map[string]*typeInfo),
		processQueue: list.New(),
	}
}

func (p *Parser) Parse() (*Schema, error) {
	return p.ParsePattern(".")
}

func (p *Parser) ParsePattern(pattern string) (*Schema, error) {
	set := token.NewFileSet()
	pkgs, e := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedModule,
		Fset:  set,
		Dir:   p.config.DirPath,
		Tests: false,
	}, pattern)

	if e != nil {
		return nil, e
	}

	p.pkg = pkgs[0]
	p.schema = createSchema()
	p.processPackages(set, pkgs)

	if err := p.feedProcessQueue(); err != nil {
		return nil, err
	}

	if err := p.processTypes(); err != nil {
		return nil, err
	}

	return p.schema, nil
}

func (p *Parser) processPackages(set *token.FileSet, pkgs []*packages.Package) {
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			p.collectTypesAndImports(file, pkg.PkgPath, ast.NewCommentMap(set, file, file.Comments))
		}
	}
}

func (p *Parser) collectTypesAndImports(file *ast.File, pkgPath string, cmap ast.CommentMap) {
	target := p.types
	imports := make(map[string]string)
	for _, imp := range file.Imports {
		path, name := parseImport(imp)
		isInternal := strings.HasPrefix(path, pkgPath)
		if isInternal {
			path = "." + strings.TrimPrefix(path, pkgPath)
		}
		imports[name] = path
	}
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		comms := cmap[genDecl]
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if typeSpec.Name.IsExported() {
				name := typeSpec.Name.Name
				target[name] = &typeInfo{typeSpec, comms, imports, name, false}
			}
		}
	}
}

func (p *Parser) feedProcessQueue() error {
	configTypeName := p.config.ConfigType
	if p.config.Mode == modeComponent {
		if ti, ok := p.types[configTypeName]; ok {
			p.processQueue.PushBack(ti)
		}
	} else {
		for _, ti := range p.types {
			p.processQueue.PushBack(ti)
		}
	}
	if len(p.types) == 0 {
		return errors.New("no exported types found in the package")
	}
	return nil
}

func (p *Parser) processTypes() error {
	for p.processQueue.Len() > 0 {
		item := p.processQueue.Front()
		p.processQueue.Remove(item)

		ti, _ := item.Value.(*typeInfo)
		typeName := ti.typeName
		if ti.processed {
			continue
		}
		ti.processed = true
		p.current = ti

		schemaElem, err := p.parseType(ti)
		if err != nil {
			return fmt.Errorf("parse type spec %s: %w", typeName, err)
		}
		if schemaElem == nil {
			continue
		}

		if obj, ok := schemaElem.(*objectSchemaElement); ok {
			if len(obj.Properties) == 0 && len(obj.AllOf) == 0 {
				continue
			}
		}

		if p.isConfigType(ti) {
			if obj, ok := schemaElem.(*objectSchemaElement); ok {
				p.schema.objectSchemaElement = *obj
			}
			if field, ok := schemaElem.(*fieldSchemaElement); ok {
				p.schema.ElementType = field.ElementType
			}
			if ref, ok := schemaElem.(*refSchemaElement); ok {
				p.schema.ElementType = schemaTypeObject
				p.schema.AllOf = append(p.schema.AllOf, ref)
			}
		} else {
			p.schema.Defs.AddDef(strcase.ToSnake(typeName), schemaElem)
		}
	}
	return nil
}

func (p *Parser) isConfigType(ti *typeInfo) bool {
	return p.config.Mode == modeComponent && ti.typeName == p.config.ConfigType
}

func (p *Parser) parseType(ti *typeInfo) (schemaElement, error) {
	typeSpec := ti.spec
	switch typeSpec.Type.(type) {
	case *ast.InterfaceType, *ast.FuncType:
		return nil, nil
	}
	schemaElem, err := p.parseExpr(typeSpec.Type)
	if err != nil {
		return nil, err
	}
	if len(ti.comms) > 0 {
		if desc, ok := extractDescriptionFromComment(ti.comms[0]); ok {
			schemaElem.setDescription(desc)
		}
	}
	return schemaElem, nil
}

func (p *Parser) parseExpr(expr ast.Expr) (schemaElement, error) {
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
	case *ast.IndexExpr:
		return p.parseOptional(t)
	}
	return nil, errors.New("unrecognized field type" + fmt.Sprintf(" (%T)", expr))
}

func (p *Parser) parseStruct(structType *ast.StructType) (schemaElement, error) {
	var so schemaObject = createObjectField("")

	for _, field := range structType.Fields.List {
		tag, ok := parseTag(field.Tag)
		if !ok {
			continue
		}
		if len(field.Names) == 0 || tag.Squash {
			if err := p.addEmbeddedField(field, so); err != nil {
				return nil, err
			}
			continue
		}
		p.addNamedFields(field, so)
	}

	return so.(schemaElement), nil
}

func (p *Parser) addEmbeddedField(field *ast.Field, so schemaObject) error {
	ident, ok := field.Type.(*ast.Ident)
	if !ok {
		selector, ok := field.Type.(*ast.SelectorExpr)
		if ok {
			element, err := p.parseSelector(selector)
			if err != nil {
				return err
			}
			so.AddEmbedded(element)
			return nil
		}
		return errors.New("unrecognized embedded field type ")
	}

	elem, err := p.parseIdent(ident)
	if err == nil {
		switch elem := elem.(type) {
		case *refSchemaElement:
			so.AddEmbedded(elem)
			return nil
		case *objectSchemaElement:
			return mergeSchemas(so, elem)
		}
	}
	return err
}

func (p *Parser) addNamedFields(field *ast.Field, so schemaObject) {
	for _, ident := range field.Names {
		tag, hasTag := parseTag(field.Tag)
		if !ident.IsExported() || !hasTag {
			continue
		}
		fieldName := tag.Name
		if fieldName == "" {
			fieldName = ident.Name
		}
		p.addNamedField(fieldName, field, so)
	}
}

func (p *Parser) addNamedField(fieldName string, field *ast.Field, so schemaObject) {
	element, err := p.parseExpr(field.Type)
	if err != nil {
		fmt.Printf("Error parsing field %s: %v\n", fieldName, err)
		return
	}
	if description, ok := extractDescriptionFromComment(field.Doc); ok {
		element.setDescription(description)
	}
	so.AddProperty(fieldName, element)
}

func (p *Parser) parseArray(array *ast.ArrayType) (schemaElement, error) {
	itemSchema, err := p.parseExpr(array.Elt)
	if err != nil {
		return nil, err
	}
	return createArrayField(itemSchema, ""), nil
}

func (p *Parser) parseIdent(ident *ast.Ident) (schemaElement, error) {
	typeName := ident.Name
	if primitiveType, isCustom := goPrimitiveToSchemaType(typeName); primitiveType != schemaTypeUnknown {
		element := createSimpleField(primitiveType, "")
		if isCustom {
			element.CustomElementType = typeName
		}
		return element, nil
	}

	if info, exists := p.types[typeName]; exists {
		p.processQueue.PushBack(info)
		return createRefField(strcase.ToSnake(typeName), ""), nil
	}

	if ident.Obj != nil {
		if typeSpec, ok := ident.Obj.Decl.(*ast.TypeSpec); ok {
			return p.parseExpr(typeSpec.Type)
		}
	}

	return nil, fmt.Errorf("type %s not found in collected type specs", typeName)
}

func (p *Parser) parseMap(m *ast.MapType) (schemaElement, error) {
	valueSchema, err := p.parseExpr(m.Value)
	if err != nil {
		return nil, err
	}
	return createMapField(valueSchema, ""), nil
}

func (p *Parser) parsePointer(pointer *ast.StarExpr) (schemaElement, error) {
	element, err := p.parseExpr(pointer.X)
	if err != nil {
		return nil, err
	}
	element.setIsPointer(true)
	return element, nil
}

func (p *Parser) parseSelector(selector *ast.SelectorExpr) (schemaElement, error) {
	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil, errors.New("unrecognized SelectorExpr structure")
	}

	pkgName := pkgIdent.Name
	name := selector.Sel.Name
	fullTypeName := pkgName + "." + name

	if path, ok := p.current.imports[pkgName]; ok {
		_, exists := p.config.Mappings[path]
		if !exists {
			allowed := strings.HasPrefix(path, ".")
			if !allowed {
				for _, allowedPath := range p.config.AllowedRefs {
					if strings.HasPrefix(path, allowedPath) {
						allowed = true
						break
					}
				}
			}
			fullID := fmt.Sprintf("%s.%s", path, strcase.ToSnake(name))
			if allowed {
				refID := fullID
				if path == p.config.Namespace || strings.HasPrefix(path, p.config.Namespace+"/") {
					refID, _ = strings.CutPrefix(fullID, p.config.Namespace)
				}
				return createRefField(refID, ""), nil
			}
			element := createSimpleField(schemaTypeAny, "")
			element.CustomElementType = fullID
			return element, nil
		}
		pkgName = path
		fullTypeName = pkgName + "." + name
	}

	if pkg, ok := p.config.Mappings[pkgName]; ok {
		if td, ok := pkg[name]; ok {
			element := createSimpleField(td.SchemaType, "")
			if !td.SkipAnnotation {
				element.CustomElementType = fullTypeName
			}
			element.Format = td.Format
			return element, nil
		}
	}

	if info, exists := p.types[name]; exists {
		p.processQueue.PushBack(info)
		return createRefField(strcase.ToSnake(name), ""), nil
	}

	return nil, fmt.Errorf("unrecognized type in selector: %s", fullTypeName)
}

func (p *Parser) parseOptional(indexExpr *ast.IndexExpr) (schemaElement, error) {
	wrapperType, ok := indexExpr.X.(*ast.SelectorExpr)
	if !ok {
		return nil, errors.New("unrecognized IndexExpr structure")
	}
	wrapperTypeName := wrapperType.Sel.Name

	if wrapperTypeName == "Optional" {
		element, err := p.parseExpr(indexExpr.Index)
		if err == nil {
			element.setOptional(true)
		}
		return element, err
	}

	fmt.Printf("Warning: unrecognized generic type: %s\n", wrapperTypeName)
	return nil, nil
}
