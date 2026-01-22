// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"container/list"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/packages"
)

type Parser struct {
	config       *Config
	schema       *Schema
	types        map[string]*TypeInfo
	imports      map[string]string
	processQueue *list.List
	rootPkg      *packages.Package
}

type TypeInfo struct {
	spec      *ast.TypeSpec
	comms     []*ast.CommentGroup
	pkgName   string
	processed bool
}

func NewParser(cfg *Config) *Parser {
	return &Parser{
		config:       cfg,
		types:        make(map[string]*TypeInfo),
		imports:      make(map[string]string),
		processQueue: list.New(),
	}
}

func (p *Parser) Parse() (*Schema, error) {
	set := token.NewFileSet()
	pkgs, e := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedModule,
		Fset:  set,
		Dir:   p.config.DirPath,
		Tests: false,
	}, "./...")

	if e != nil {
		return nil, e
	}

	p.rootPkg = pkgs[0]
	p.processPackages(set, pkgs)
	p.initializeSchema()
	p.feedProcessQueue()

	if p.processQueue.Len() == 0 {
		msg := "no types to process"
		if p.config.Mode == Component {
			msg += fmt.Sprintf("; check if the root type name '%s' is correct", p.config.RootTypeName)
		}
		return nil, errors.New(msg)
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
	if _, ok := p.types[p.config.RootTypeName]; !ok && p.config.Mode == Component {
		fmt.Printf("Warning: Root type %s not found among collected type specs\n", p.config.RootTypeName)
	}
}

func (p *Parser) initializeSchema() {
	var (
		id    = p.rootPkg.ID
		title = fmt.Sprintf("%s %s", p.rootPkg.Name, p.config.Mode)
	)
	p.schema = CreateSchema(id, title)
}

func (p *Parser) collectTypesAndImports(file *ast.File, pkgPath string, cmap ast.CommentMap) {
	target := p.types
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
				pgkName := file.Name.Name
				target[name] = &TypeInfo{typeSpec, comms, pgkName, false}
			}
		}
	}
	for _, imp := range file.Imports {
		path, name := ParseImport(imp)
		// omit internal package paths
		if strings.HasPrefix(path, pkgPath) {
			continue
		}
		// if mapping defined for package, skip adding import
		if p.config.Mappings[name] != nil {
			continue
		}
		p.imports[name] = path
	}
}

func (p *Parser) feedProcessQueue() {
	filterFn := func(name string, t *TypeInfo) bool {
		if p.config.Mode == Component {
			return name == p.config.RootTypeName
		}
		return t.pkgName == p.rootPkg.Name
	}
	for name, typeSpec := range p.types {
		if filterFn(name, typeSpec) {
			p.processQueue.PushBack(name)
		}
	}
}

func (p *Parser) processTypes() error {
	for p.processQueue.Len() > 0 {
		item := p.processQueue.Front()
		p.processQueue.Remove(item)

		name, _ := item.Value.(string)
		typeInfo := p.types[name]
		if typeInfo.processed {
			continue
		}
		typeInfo.processed = true
		schemaElement, err := p.parseType(typeInfo)
		if err != nil {
			return fmt.Errorf("parse type spec %s: %w", name, err)
		}
		if schemaElement == nil {
			continue
		}

		if obj, ok := schemaElement.(*ObjectSchemaElement); ok {
			isEmpty := len(obj.Properties) == 0 && len(obj.AllOf) == 0
			if isEmpty {
				continue // skip struct types with no exported fields
			}
		}

		if p.isRootType(name) {
			if obj, ok := schemaElement.(*ObjectSchemaElement); ok {
				p.schema.ObjectSchemaElement = *obj
			}
			if field, ok := schemaElement.(*FieldSchemaElement); ok {
				p.schema.ElementType = field.ElementType
			}
		} else {
			if typeInfo.pkgName != p.rootPkg.Name {
				name = typeInfo.pkgName + "." + name
			}
			p.schema.Defs.AddDef(name, schemaElement)
		}
	}
	return nil
}

func (p *Parser) isRootType(name string) bool {
	return p.config.Mode == Component && (name == p.config.RootTypeName || len(p.types) == 1)
}

func (p *Parser) parseType(typeInfo *TypeInfo) (SchemaElement, error) {
	typeSpec := typeInfo.spec
	switch typeSpec.Type.(type) {
	case *ast.InterfaceType, *ast.FuncType:
		// skip these types
		return nil, nil
	}
	schemaElement, err := p.parseExpr(typeSpec.Type)
	if err != nil {
		return nil, err
	}
	if len(typeInfo.comms) > 0 {
		if desc, ok := ExtractDescriptionFromComment(typeInfo.comms[0]); ok {
			schemaElement.setDescription(desc)
		}
	}

	return schemaElement, nil
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
	case *ast.IndexExpr:
		return p.parseOptional(t)
	}

	return nil, errors.New("unrecognized field type" + fmt.Sprintf(" (%T)", expr))
}

func (p *Parser) parseStruct(structType *ast.StructType) (SchemaElement, error) {
	var schemaObject SchemaObject = CreateObjectField("")

	for _, field := range structType.Fields.List {
		tag, ok := ParseTag(field.Tag)
		if !ok {
			continue
		}
		if len(field.Names) == 0 || tag.Squash {
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
			element, err := p.parseSelector(selector)
			if err != nil {
				return err
			}
			if refElement, ok := element.(*RefSchemaElement); ok {
				schemaObject.AddEmbeddedRef(refElement.Ref)
				return nil
			}

			fmt.Printf("Warning: could not find schema reference to type %s.%s\n", selector.X, selector.Sel)
			return nil
		}

		return errors.New("unrecognized embedded field type ")
	}

	typeName := ident.Name
	if _, exists := p.types[typeName]; !exists {
		if typeSpec, ok := ident.Obj.Decl.(*ast.TypeSpec); ok {
			elem, err := p.parseExpr(typeSpec.Type)
			if err != nil {
				return err
			}
			return mergeSchemas(schemaObject, elem)
		}
		return fmt.Errorf("type %s not found in collected type specs", typeName)
	}
	p.processQueue.PushBack(typeName)
	schemaObject.AddEmbeddedRef("#/$defs/" + typeName)
	return nil
}

func (p *Parser) addNamedFields(field *ast.Field, schemaObject SchemaObject) {
	for _, ident := range field.Names {
		tag, hasTag := ParseTag(field.Tag)
		isValid := ident.IsExported() && hasTag
		if !isValid {
			continue
		}
		fieldName := tag.Name
		if fieldName == "" {
			fieldName = ident.Name
		}
		p.addNamedField(fieldName, field, schemaObject)
	}
}

func (p *Parser) addNamedField(fieldName string, field *ast.Field, schemaObject SchemaObject) {
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
	typeName := ident.Name
	if primitiveType, isCustom := goPrimitiveToSchemaType(typeName); primitiveType != SchemaTypeUnknown {
		element := CreateSimpleField(primitiveType, "")
		if isCustom {
			element.CustomElementType = typeName
		}
		return element, nil
	}

	if ident.Obj != nil {
		typeSpec, ok := ident.Obj.Decl.(*ast.TypeSpec)
		if !ok {
			return nil, errors.New("unrecognized Ident declaration type")
		}
		typeName = typeSpec.Name.Name
	}

	if info, exists := p.types[typeName]; exists {
		p.processQueue.PushBack(typeName)
		if info.pkgName != p.rootPkg.Name {
			typeName = info.pkgName + "." + typeName
		}
		return CreateRefField("#/$defs/"+typeName, ""), nil
	}
	return nil, fmt.Errorf("type %s not found in collected type specs", typeName)
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

func (p *Parser) parseSelector(selector *ast.SelectorExpr) (SchemaElement, error) {
	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil, errors.New("unrecognized SelectorExpr structure")
	}

	if path, ok := p.imports[pkgIdent.Name]; ok {
		fullID := fmt.Sprintf("%s#/$defs/%s", path, selector.Sel.Name)
		element := CreateRefField(fullID, "")
		return element, nil
	}

	fullTypeName := pkgIdent.Name + "." + selector.Sel.Name
	if pkg, ok := p.config.Mappings[pkgIdent.Name]; ok {
		if typeDesc, ok := pkg[selector.Sel.Name]; ok {
			element := CreateSimpleField(typeDesc.SchemaType, "")
			element.CustomElementType = fullTypeName
			element.Format = typeDesc.Format
			return element, nil
		}
	}

	name := selector.Sel.Name
	if info, exists := p.types[name]; exists {
		p.processQueue.PushBack(name)
		if info.pkgName != p.rootPkg.Name {
			name = fullTypeName
		}
		element := CreateRefField("#/$defs/"+name, "")
		return element, nil
	}

	return nil, fmt.Errorf("unrecognized type in selector: %s", fullTypeName)
}

func (p *Parser) parseOptional(indexExpr *ast.IndexExpr) (SchemaElement, error) {
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

	return nil, fmt.Errorf("unrecognized generic type: %s", wrapperTypeName)
}
