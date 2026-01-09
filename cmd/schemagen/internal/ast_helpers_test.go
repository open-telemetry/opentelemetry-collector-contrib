// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestExtractNameFromTag(t *testing.T) {
	testCases := []struct {
		name      string
		tagSource string
		expected  string
		expectHit bool
	}{
		{
			name:      "uses mapstructure when json present",
			tagSource: `json:"json-name" mapstructure:"map-name"`,
			expected:  "map-name",
			expectHit: true,
		},
		{
			name:      "falls back to mapstructure",
			tagSource: `mapstructure:"host"`,
			expected:  "host",
			expectHit: true,
		},
		{
			name:      "mapstructure with options trimmed",
			tagSource: `mapstructure:"host,omitempty"`,
			expected:  "host",
			expectHit: true,
		},
		{
			name:      "ignores json skip dash uses mapstructure",
			tagSource: `json:"-" mapstructure:"custom_host"`,
			expected:  "custom_host",
			expectHit: true,
		},
		{
			name:      "json only tag ignored",
			tagSource: `json:"json_only"`,
			expected:  "",
			expectHit: false,
		},
		{
			name:      "missing tags",
			tagSource: "",
			expected:  "",
			expectHit: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			field := parseFieldWithTag(t, tc.tagSource)
			value, ok := ExtractNameFromTag(field)
			if ok != tc.expectHit {
				t.Fatalf("expected hit=%v but got %v", tc.expectHit, ok)
			}
			if value != tc.expected {
				t.Fatalf("expected %q but got %q", tc.expected, value)
			}
		})
	}
}

func TestExtractDescriptionFromComment(t *testing.T) {
	testCases := []struct {
		name     string
		group    *ast.CommentGroup
		expected string
		ok       bool
	}{
		{
			name:     "nil comment group",
			group:    nil,
			expected: "",
			ok:       false,
		},
		{
			name: "single line comment",
			group: &ast.CommentGroup{
				List: []*ast.Comment{{Text: "// A simple description"}},
			},
			expected: "A simple description",
			ok:       true,
		},
		{
			name: "multi line mixed comment",
			group: &ast.CommentGroup{
				List: []*ast.Comment{
					{Text: "// First sentence"},
					{Text: "// second sentence"},
					{Text: "/* trailing block */"},
				},
			},
			expected: "First sentence second sentence trailing block",
			ok:       true,
		},
		{
			name: "empty comment text",
			group: &ast.CommentGroup{
				List: []*ast.Comment{
					{Text: "//"},
					{Text: "//   "},
				},
			},
			expected: "",
			ok:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := ExtractDescriptionFromComment(tc.group)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v got %v", tc.ok, ok)
			}
			if value != tc.expected {
				t.Fatalf("expected %q got %q", tc.expected, value)
			}
		})
	}
}

func TestGoPrimitiveToSchemaType(t *testing.T) {
	testCases := []struct {
		name         string
		typeName     string
		expectedType SchemaType
		isCustom     bool
	}{
		{"string type", "string", SchemaTypeString, false},
		{"bool type", "bool", SchemaTypeBoolean, false},
		{"integer types", "int32", SchemaTypeInteger, true},
		{"number types", "float64", SchemaTypeNumber, true},
		{"any type", "any", SchemaTypeAny, true},
		{"unknown type", "Custom", SchemaTypeUnknown, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got, custom := goPrimitiveToSchemaType(tc.typeName); got != tc.expectedType || custom != tc.isCustom {
				t.Fatalf("expected %q:%t got %q:%t", tc.expectedType, tc.isCustom, got, custom)
			}
		})
	}
}

func TestParseImport(t *testing.T) {
	testCases := []struct {
		name     string
		literal  string
		alias    string
		expected string
		nameWant string
	}{
		{
			name:     "uses trailing path segment as name",
			literal:  fmt.Sprintf("%q", "go.opentelemetry.io/collector/confmap/converter"),
			expected: "go.opentelemetry.io/collector/confmap/converter",
			nameWant: "converter",
		},
		{
			name:     "stdlib package retains full name",
			literal:  fmt.Sprintf("%q", "context"),
			expected: "context",
			nameWant: "context",
		},
		{
			name:     "alias overrides parsed name",
			literal:  fmt.Sprintf("%q", "example.com/project/component"),
			alias:    "componentAlias",
			expected: "example.com/project/component",
			nameWant: "componentAlias",
		},
		{
			name:     "alias blank identifier preserved",
			literal:  fmt.Sprintf("%q", "example.com/project/component"),
			alias:    "_",
			expected: "example.com/project/component",
			nameWant: "_",
		},
		{
			name:     "handles literal value without quotes",
			literal:  "example.com/org/pkg/v2",
			expected: "example.com/org/pkg/v2",
			nameWant: "v2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			spec := buildImportSpec(tc.literal, tc.alias)
			full, gotName := ParseImport(spec)
			if full != tc.expected {
				t.Fatalf("expected full path %q got %q", tc.expected, full)
			}
			if gotName != tc.nameWant {
				t.Fatalf("expected name %q got %q", tc.nameWant, gotName)
			}
		})
	}
}

func parseFieldWithTag(t *testing.T, tagContent string) *ast.Field {
	t.Helper()

	tag := ""
	if tagContent != "" {
		tag = fmt.Sprintf(" `%s`", tagContent)
	}

	src := fmt.Sprintf(`package test

	type sample struct {
		Field string%s
	}
	`, tag)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "sample.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	if len(file.Decls) == 0 {
		t.Fatalf("no declarations parsed")
	}

	gen, ok := file.Decls[0].(*ast.GenDecl)
	if !ok || len(gen.Specs) == 0 {
		t.Fatalf("invalid declaration structure")
	}

	typeSpec, ok := gen.Specs[0].(*ast.TypeSpec)
	if !ok {
		t.Fatalf("expected type spec")
	}

	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok || len(structType.Fields.List) == 0 {
		t.Fatalf("expected struct fields")
	}

	return structType.Fields.List[0]
}

func buildImportSpec(literal, alias string) *ast.ImportSpec {
	spec := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: literal,
		},
	}
	if alias != "" {
		spec.Name = ast.NewIdent(alias)
	}
	return spec
}
