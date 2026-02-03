//go:build !windows

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentParser(t *testing.T) {
	type testCase struct {
		title              string
		inputFile          string
		expectedSchemaFile string
		rootType           string
	}

	testCases := []testCase{
		{
			title:              "Test Simple Config Parsing",
			inputFile:          "testdata/test00/SimpleConfig.go",
			expectedSchemaFile: "testdata/test00/simple_config.schema.yaml",
			rootType:           "SimpleConfig",
		},
		{
			title:              "Test Array field Config Parsing",
			inputFile:          "testdata/test01/ArrayFieldConfig.go",
			expectedSchemaFile: "testdata/test01/array_field_config.schema.yaml",
			rootType:           "SimpleArrayConfig",
		},
		{
			title:              "Test Nested Struct Config Parsing",
			inputFile:          "testdata/test02/NestedStructConfig.go",
			expectedSchemaFile: "testdata/test02/nested_struct_config.schema.yaml",
			rootType:           "Config",
		},
		{
			title:              "Test Map field Config Parsing",
			inputFile:          "testdata/test03/MapFieldConfig.go",
			expectedSchemaFile: "testdata/test03/map_field_config.schema.yaml",
			rootType:           "MapConfig",
		},
		{
			title:              "Test Ref field Config Parsing",
			inputFile:          "testdata/test04/RefFieldConfig.go",
			expectedSchemaFile: "testdata/test04/ref_field_config.schema.yaml",
			rootType:           "RefFieldConfig",
		},
		{
			title:              "Test Embedded Struct Config Parsing",
			inputFile:          "testdata/test05/EmbeddedStructConfig.go",
			expectedSchemaFile: "testdata/test05/embedded_struct_config.schema.yaml",
			rootType:           "EmbeddedStructConfig",
		},
		{
			title:              "Test Pointer field Config Parsing",
			inputFile:          "testdata/test06/PointerFieldConfig.go",
			expectedSchemaFile: "testdata/test06/pointer_field_config.schema.yaml",
			rootType:           "PointerFieldConfig",
		},
		{
			title:              "Test complex type field Config Parsing",
			inputFile:          "testdata/test07/ComplexTypeFieldConfig.go",
			expectedSchemaFile: "testdata/test07/complex_type_field_config.schema.yaml",
			rootType:           "ComplexTypeFieldConfig",
		},
		{
			title:              "Test time type fields Config Parsing",
			inputFile:          "testdata/test08/TimeTypeFieldConfig.go",
			expectedSchemaFile: "testdata/test08/time_type_field_config.schema.yaml",
			rootType:           "TimeTypeFieldConfig",
		},
		{
			title:              "Test Mixed Tags Config Parsing",
			inputFile:          "testdata/test09/MixedTagsConfig.go",
			expectedSchemaFile: "testdata/test09/mixed_tags_config.schema.yaml",
			rootType:           "MixedTagsConfig",
		},
		{
			title:              "Test Simple Type Aliases Parsing",
			inputFile:          "testdata/test10/AliasSimpleTypeConfig.go",
			expectedSchemaFile: "testdata/test10/alias_simple_type_config.schema.yaml",
			rootType:           "AliasSimpleTypeConfig",
		},
		{
			title:              "Test External Refs Parsing",
			inputFile:          "testdata/test11/ExternalRefsConfig.go",
			expectedSchemaFile: "testdata/test11/external_refs_config.schema.yaml",
			rootType:           "ExternalRefsConfig",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			expectedBytes, err := os.ReadFile(tc.expectedSchemaFile)
			if err != nil {
				t.Fatalf("Failed to read expected schema file %s: %v", tc.expectedSchemaFile, err)
			}
			expectedSchema := string(expectedBytes)

			dir, _ := filepath.Abs(filepath.Dir(tc.inputFile))
			cfg := &Config{
				Mode:     Component,
				DirPath:  dir,
				Mappings: testMappings(),
				AllowedRefs: []string{
					"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/schemagen",
				},
			}
			if tc.rootType != "" {
				cfg.ConfigType = tc.rootType
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

func TestPackageParser(t *testing.T) {
	dir, _ := filepath.Abs("testdata/external/")
	cfg := &Config{
		Mode:     Package,
		DirPath:  dir,
		Mappings: testMappings(),
	}

	parser := NewParser(cfg)

	schema, err := parser.Parse()
	require.NoError(t, err)

	rawYaml, err := schema.ToYAML()
	require.NoError(t, err)

	expectedBytes, err := os.ReadFile("testdata/external/config.schema.yaml")
	if err != nil {
		t.Fatalf("Failed to read expected schema file: %v", err)
	}
	expectedSchema := string(expectedBytes)

	givenYaml := string(rawYaml)
	require.YAMLEq(t, expectedSchema, givenYaml)
}

func testMappings() Mappings {
	return Mappings{
		"time": PackagesMapping{
			"Time": TypeDesc{
				SchemaType: SchemaTypeString,
				Format:     "date-time",
			},
			"Duration": TypeDesc{
				SchemaType: SchemaTypeString,
				Format:     "duration",
			},
		},
	}
}
