// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestFormatString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "converts underscores to hyphens",
			input:    "hello_world",
			expected: "hello-world",
		},
		{
			name:     "converts to lowercase",
			input:    "HELLO_WORLD",
			expected: "hello-world",
		},
		{
			name:     "handles mixed case and multiple underscores",
			input:    "Hello_Big_WORLD",
			expected: "hello-big-world",
		},
		{
			name:     "handles string with no underscores",
			input:    "HelloWorld",
			expected: "helloworld",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatString(tt.input)
			if got != tt.expected {
				t.Errorf("formatString() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSplitRefWorkflowPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple workflow with version",
			input:    "my-great-workflow@v1.0.0",
			expected: "my-great-workflow",
			wantErr:  false,
		},
		{
			name:     "workflow with SHA",
			input:    "my-great-workflow@3421498310493281409328140932840192384",
			expected: "my-great-workflow",
			wantErr:  false,
		},
		{
			name:     "full path workflow",
			input:    "org/repo/.github/my-file-path/with/folder/build-woot.yaml@v0.2.3",
			expected: "build-woot",
			wantErr:  false,
		},
		{
			name:     "uppercase file",
			input:    "org/repo/.github/my-file-path/with/folder/BUILD-WOOT.yaml@v0.2.3",
			expected: "build-woot",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := splitRefWorkflowPath(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReplaceAPIURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "converts api.github.com URL to html URL",
			input:    "https://api.github.com/repos/open-telemetry/opentelemetry-collector-contrib/pull/1234",
			expected: "https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/1234",
		},
		{
			name:     "converts api.github.com workflow URL to html URL",
			input:    "https://api.github.com/repos/open-telemetry/opentelemetry-collector-contrib/actions/runs/1234",
			expected: "https://github.com/open-telemetry/opentelemetry-collector-contrib/actions/runs/1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceAPIURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddCustomPropertiesToAttrs(t *testing.T) {
	tests := []struct {
		name         string
		customProps  map[string]any
		expectedKeys []string
		expectedVals map[string]any
	}{
		{
			name: "adds string custom properties",
			customProps: map[string]any{
				"team_name":    "open-telemetry",
				"environment":  "development",
				"service_name": "should-be-skipped", // This should be skipped
			},
			expectedKeys: []string{
				"github.repository.custom_properties.team_name",
				"github.repository.custom_properties.environment",
			},
			expectedVals: map[string]any{
				"github.repository.custom_properties.team_name":   "open-telemetry",
				"github.repository.custom_properties.environment": "development",
			},
		},
		{
			name: "adds different types of custom properties",
			customProps: map[string]any{
				"string_prop": "string-value",
				"int_prop":    42,
				"float_prop":  3.14,
				"bool_prop":   true,
			},
			expectedKeys: []string{
				"github.repository.custom_properties.string_prop",
				"github.repository.custom_properties.int_prop",
				"github.repository.custom_properties.float_prop",
				"github.repository.custom_properties.bool_prop",
			},
			expectedVals: map[string]any{
				"github.repository.custom_properties.string_prop": "string-value",
				"github.repository.custom_properties.int_prop":    int64(42),
				"github.repository.custom_properties.float_prop":  3.14,
				"github.repository.custom_properties.bool_prop":   true,
			},
		},
		{
			name: "converts keys to snake_case",
			customProps: map[string]any{
				"camelCase":   "camel-value",
				"PascalCase":  "pascal-value",
				"kebab-case":  "kebab-value",
				"space case":  "space-value",
				"mixed_Case":  "mixed-value",
				"withNumber1": "number-value",
				"with.dots":   "dots-value",
				"$dollar":     "dollar-value",
				"#hash":       "hash-value",
			},
			expectedKeys: []string{
				"github.repository.custom_properties.camel_case",
				"github.repository.custom_properties.pascal_case",
				"github.repository.custom_properties.kebab_case",
				"github.repository.custom_properties.space_case",
				"github.repository.custom_properties.mixed_case",
				"github.repository.custom_properties.with_number1",
				"github.repository.custom_properties.with_dots",
				"github.repository.custom_properties._dollar_dollar",
				"github.repository.custom_properties._hash_hash",
			},
			expectedVals: map[string]any{
				"github.repository.custom_properties.camel_case":     "camel-value",
				"github.repository.custom_properties.pascal_case":    "pascal-value",
				"github.repository.custom_properties.kebab_case":     "kebab-value",
				"github.repository.custom_properties.space_case":     "space-value",
				"github.repository.custom_properties.mixed_case":     "mixed-value",
				"github.repository.custom_properties.with_number1":   "number-value",
				"github.repository.custom_properties.with_dots":      "dots-value",
				"github.repository.custom_properties._dollar_dollar": "dollar-value",
				"github.repository.custom_properties._hash_hash":     "hash-value",
			},
		},
		{
			name:         "handles empty custom properties",
			customProps:  map[string]any{},
			expectedKeys: []string{},
			expectedVals: map[string]any{},
		},
		{
			name:         "handles nil custom properties",
			customProps:  nil,
			expectedKeys: []string{},
			expectedVals: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new resource with empty attributes
			resource := pcommon.NewResource()
			attrs := resource.Attributes()

			// Add custom properties to attributes
			addCustomPropertiesToAttrs(attrs, tt.customProps)

			// Check that all expected keys exist
			for _, key := range tt.expectedKeys {
				_, exists := attrs.Get(key)
				assert.True(t, exists, "Expected key %s not found", key)
			}

			// Check that service_name is not added
			_, exists := attrs.Get("github.repository.custom_properties.service_name")
			assert.False(t, exists)

			// Check values
			for key, expectedVal := range tt.expectedVals {
				switch expected := expectedVal.(type) {
				case string:
					val, _ := attrs.Get(key)
					assert.Equal(t, expected, val.Str())
				case int64:
					val, _ := attrs.Get(key)
					assert.Equal(t, expected, val.Int())
				case float64:
					val, _ := attrs.Get(key)
					assert.Equal(t, expected, val.Double())
				case bool:
					val, _ := attrs.Get(key)
					assert.Equal(t, expected, val.Bool())
				}
			}

			// Check that no unexpected keys were added
			count := 0
			attrs.Range(func(_ string, _ pcommon.Value) bool {
				count++
				return true
			})
			assert.Equal(t, len(tt.expectedKeys), count)
		})
	}
}
