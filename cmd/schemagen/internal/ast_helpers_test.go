// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
		name     string
		typeName string
		expected SchemaType
	}{
		{"string type", "string", SchemaTypeString},
		{"bool type", "bool", SchemaTypeBoolean},
		{"integer types", "int32", SchemaTypeInteger},
		{"number types", "float64", SchemaTypeNumber},
		{"any type", "any", ""},
		{"unknown type", "Custom", SchemaTypeNull},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := goPrimitiveToSchemaType(tc.typeName); got != tc.expected {
				t.Fatalf("expected %q got %q", tc.expected, got)
			}
		})
	}
}

func TestGetSchemaIDWithPrefix(t *testing.T) {
	repoDir := setupGitRepo(t)
	cfg := &Config{
		DirPath:        repoDir,
		SchemaPath:     filepath.Join(repoDir, "schemas/config.schema.yaml"),
		SchemaIDPrefix: "https://example.com/root",
	}
	file := parseTestFile(t, "package test")

	id, err := GetSchemaID(file, cfg)

	require.NoError(t, err)
	require.Equal(t, "https://example.com/root/config.schema.yaml", id)
}

func TestGetSchemaIDWithImportComment(t *testing.T) {
	repoDir := setupGitRepo(t)
	cfg := &Config{
		DirPath:    repoDir,
		SchemaPath: filepath.Join(repoDir, "schemas/config.schema.yaml"),
	}
	file := parseTestFile(t, `package test
	// import "github.com/example/repo"`)

	id, err := GetSchemaID(file, cfg)
	require.NoError(t, err)

	basePath, err := getBasePath(repoDir)
	require.NoError(t, err)
	relPath, err := filepath.Rel(strings.TrimSpace(basePath), repoDir)
	require.NoError(t, err)
	repo := "github.com/example/repo"
	rootURL := strings.TrimSuffix(repo, filepath.ToSlash(relPath))
	expected, err := url.JoinPath("https://", rootURL, "blob/main", relPath, "config.schema.yaml")
	require.NoError(t, err)
	require.Equal(t, expected, id)
}

func TestGetSchemaIDErrorWithoutGitOrImport(t *testing.T) {
	cfg := &Config{
		DirPath:    t.TempDir(),
		SchemaPath: "schema.yaml",
	}
	file := parseTestFile(t, "package test")

	_, err := GetSchemaID(file, cfg)
	require.Error(t, err)
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

func parseTestFile(t *testing.T, source string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	require.NoError(t, err)
	return file
}

func setupGitRepo(t *testing.T) string {
	dir := t.TempDir()
	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "remote", "add", "origin", "git@github.com:example/repo.git")
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return resolved
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}
