// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteSchemaToFile_YAML(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		SchemaPath: filepath.Join(dir, "test-schema.yaml"),
		FileType:   "yaml",
	}
	schema := testSchema(t)

	err := WriteSchemaToFile(schema, cfg)
	require.NoError(t, err)

	outputPath := cfg.SchemaPath
	require.FileExists(t, outputPath)

	expected, err := schema.ToYAML()
	require.NoError(t, err)

	actual, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, string(expected), string(actual))
}

func TestWriteSchemaToFile_JSON(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		SchemaPath: filepath.Join(dir, "test-schema.json"),
		FileType:   "json",
	}
	schema := testSchema(t)

	err := WriteSchemaToFile(schema, cfg)
	require.NoError(t, err)

	outputPath := cfg.SchemaPath
	require.FileExists(t, outputPath)

	expected, err := schema.ToJSON()
	require.NoError(t, err)

	actual, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.JSONEq(t, string(expected), string(actual))
}

func TestWriteSchemaToFile_WriteError(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		// parent directory does not exist, forcing os.WriteFile to fail
		SchemaPath: filepath.Join(dir, "unwritable", "schema.yaml"),
		FileType:   "yaml",
	}
	schema := testSchema(t)

	err := WriteSchemaToFile(schema, cfg)
	require.Error(t, err)

	outputPath := cfg.SchemaPath
	require.NoFileExists(t, outputPath)
}

func testSchema(t *testing.T) *Schema {
	t.Helper()
	schema := CreateSchema("http://example.com/schema", "test", "desc")
	schema.AddProperty("name", CreateSimpleField(SchemaTypeString, "field"))
	return schema
}
