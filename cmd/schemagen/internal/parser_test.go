//go:build !windows

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package internal

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParser(t *testing.T) {
	type testCase struct {
		title              string
		inputFile          string
		expectedSchemaFile string
		rootType           string
	}

	testCases := []testCase{
		{
			title:              "Test Simple Config Parsing",
			inputFile:          "testdata/SimpleConfig.go",
			expectedSchemaFile: "testdata/simple_config.schema.yaml",
			rootType:           "SimpleConfig",
		},
		{
			title:              "Test Array field Config Parsing",
			inputFile:          "testdata/ArrayFieldConfig.go",
			expectedSchemaFile: "testdata/array_field_config.schema.yaml",
			rootType:           "ArrayFieldConfig",
		},
		{
			title:              "Test Nested Struct Config Parsing",
			inputFile:          "testdata/NestedStructConfig.go",
			expectedSchemaFile: "testdata/nested_struct_config.schema.yaml",
			rootType:           "NestedStructConfig",
		},
		{
			title:              "Test Map field Config Parsing",
			inputFile:          "testdata/MapFieldConfig.go",
			expectedSchemaFile: "testdata/map_field_config.schema.yaml",
			rootType:           "MapFieldConfig",
		},
		{
			title:              "Test Ref field Config Parsing",
			inputFile:          "testdata/RefFieldConfig.go",
			expectedSchemaFile: "testdata/ref_field_config.schema.yaml",
			rootType:           "RefFieldConfig",
		},
		{
			title:              "Test Embedded Struct Config Parsing",
			inputFile:          "testdata/EmbeddedStructConfig.go",
			expectedSchemaFile: "testdata/embedded_struct_config.schema.yaml",
			rootType:           "EmbeddedStructConfig",
		},
		{
			title:              "Test Pointer field Config Parsing",
			inputFile:          "testdata/PointerFieldConfig.go",
			expectedSchemaFile: "testdata/pointer_field_config.schema.yaml",
			rootType:           "PointerFieldConfig",
		},
		{
			title:              "Test complex type field Config Parsing",
			inputFile:          "testdata/ComplexTypeFieldConfig.go",
			expectedSchemaFile: "testdata/complex_type_field_config.schema.yaml",
			rootType:           "ComplexTypeFieldConfig",
		},
		{
			title:              "Test time type fields Config Parsing",
			inputFile:          "testdata/TimeTypeFieldConfig.go",
			expectedSchemaFile: "testdata/time_type_field_config.schema.yaml",
			rootType:           "TimeTypeFieldConfig",
		},
		{
			title:              "Test Mixed Tags Config Parsing",
			inputFile:          "testdata/MixedTagsConfig.go",
			expectedSchemaFile: "testdata/mixed_tags_config.schema.yaml",
			rootType:           "MixedTagsConfig",
		},
		{
			title:              "Test Simple Type Aliases Parsing",
			inputFile:          "testdata/AliasSimpleTypeConfig.go",
			expectedSchemaFile: "testdata/alias_simple_type_config.schema.yaml",
			rootType:           "AliasSimpleTypeConfig",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			expectedBytes, err := os.ReadFile(tc.expectedSchemaFile)
			if err != nil {
				t.Fatalf("Failed to read expected schema file %s: %v", tc.expectedSchemaFile, err)
			}
			expectedSchema := string(expectedBytes)

			cfg := &Config{
				FilePath:       tc.inputFile,
				SchemaIDPrefix: "http://example.com/schema",
				SchemaPath:     "config.schema.yaml",
			}
			if tc.rootType != "" {
				cfg.RootTypeName = tc.rootType
			}
			parser := NewParser(cfg)

			schema, err := parser.Parse()
			require.NoError(t, err)

			rawYaml, err := schema.ToYAML()
			require.NoError(t, err)

			givenYaml := string(rawYaml)
			require.YAMLEq(t, expectedSchema, givenYaml)
		})
	}
}
