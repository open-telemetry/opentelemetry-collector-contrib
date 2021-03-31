// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

var simple = []byte(`
parameters:
  - name: message
    type: string
    required: true
pipeline:
  id: my_generator
  type: generator
  output: testoutput
  record:
    message1: {{ .message }}
`)

func TestRegisterPlugins(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tempDir := testutil.NewTempDir(t)

		err := ioutil.WriteFile(filepath.Join(tempDir, "test1.yaml"), simple, 0666)
		require.NoError(t, err)

		registry := operator.NewRegistry()
		errs := RegisterPlugins(tempDir, registry)
		require.Len(t, errs, 0)

		_, ok := registry.Lookup("test1")
		require.True(t, ok)
	})

	t.Run("Failure", func(t *testing.T) {
		tempDir := testutil.NewTempDir(t)
		err := ioutil.WriteFile(filepath.Join(tempDir, "invalid.yaml"), []byte("pipepipe:"), 0666)
		require.NoError(t, err)

		errs := RegisterPlugins(tempDir, operator.DefaultRegistry)
		require.Len(t, errs, 1)
		require.Contains(t, errs[0].Error(), "missing the pipeline block")
	})
}

func TestPluginRender(t *testing.T) {
	t.Run("ErrorExecFailure", func(t *testing.T) {
		plugin, err := NewPlugin("panicker", []byte(`pipeline:\n  {{ .panicker }}`))
		require.NoError(t, err)

		params := map[string]interface{}{
			"panicker": func() {
				panic("testpanic")
			},
		}
		_, err = plugin.Render(params)
		require.Contains(t, err.Error(), "failed to render")
	})
}

func TestPluginMetadata(t *testing.T) {
	testCases := []struct {
		name      string
		expectErr bool
		template  string
	}{
		{
			name:      "no_meta",
			expectErr: false,
			template: `pipeline:
`,
		},
		{
			name:      "full_meta",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Path
    description: The path to a thing
    type: string
  - name: other
    label: Other Thing
    description: Another parameter
    type: int
pipeline:
`,
		},
		{
			name:      "only_params",
			expectErr: false,
			template: `parameters:
  - name: path
    label: Path
    description: The path to a thing
    type: string
  - name: other
    label: Other Thing
    description: Another parameter
    type: int
pipeline:
`,
		},
		{
			name:      "out_of_order",
			expectErr: false,
			template: `parameters:
  - name: path
    label: Path
    description: The path to a thing
    type: string
  - name: other
    label: Other Thing
    description: Another parameter
    type: int
title: Test Plugin
description: This is a test plugin
pipeline:
`,
		},
		{
			name:      "bad_version",
			expectErr: true,
			template: `version: []
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Path
    description: The path to a thing
    type: string
  - name: other
    label: Other Thing
    description: Another parameter
    type: int
pipeline:
`,
		},
		{
			name:      "bad_title",
			expectErr: true,
			template: `version: 0.0.0
title: []
description: This is a test plugin
parameters:
  - name: path
    label: Path
    description: The path to a thing
    type: string
  - name: other
    label: Other Thing
    description: Another parameter
    type: int
pipeline:
`,
		},
		{
			name:      "bad_description",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: []
parameters:
  - name: path
    label: Path
    description: The path to a thing
    type: string
  - name: other
    label: Other Thing
    description: Another parameter
    type: int
pipeline:
`,
		},
		{
			name:      "bad_parameters",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters: hello
`,
		},
		{
			name:      "bad_parameter_structure",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  path: this used to be supported
pipeline:
`,
		},
		{
			name:      "bad_parameter_label",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: []
    description: The path to a thing
    type: string
pipeline:
`,
		},
		{
			name:      "bad_parameter_description",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Path
    description: []
    type: string
pipeline:
`,
		},
		{
			name:      "bad_parameter",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Path
    description: The path to a thing
    type: {}
pipeline:
`,
		},
		{
			name:      "empty_parameter",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
pipeline:
`,
		},
		{
			name:      "unknown_parameter",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: custom
pipeline:
`,
		},
		{
			name:      "string_parameter",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: string
pipeline:
`,
		},
		{
			name:      "string_parameter_default",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: string
    default: hello
pipeline:
`,
		},
		{
			name:      "string_parameter_default_invalid",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: string
    default: 123
pipeline:
`,
		},
		{
			name:      "strings_parameter",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: strings
pipeline:
`,
		},
		{
			name:      "strings_parameter_default",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: strings
    default:
     - hello
pipeline:
`,
		},
		{
			name:      "strings_parameter_default_invalid",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: strings
    default: hello
pipeline:
`,
		},

		{
			name:      "int_parameter",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: int
pipeline:
`,
		},
		{
			name:      "int_parameter_default",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: int
    default: 123
pipeline:
`,
		},
		{
			name:      "int_parameter_default_invalid",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: int
    default: hello
pipeline:
`,
		},
		{
			name:      "bool_parameter",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: bool
pipeline:
`,
		},
		{
			name:      "bool_parameter_default_true",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: bool
    default: true
pipeline:
`,
		},
		{
			name:      "bool_parameter_default_false",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: bool
    default: false
pipeline:
`,
		},
		{
			name:      "bool_parameter_default_invalid",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: bool
    default: 123
pipeline:
`,
		},
		{
			name:      "enum_parameter",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: enum
    valid_values: ["one", "two"]
pipeline:
`,
		},
		{
			name:      "enum_parameter_alternate",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: enum
    valid_values:
     - one
     - two
pipeline:
`,
		},
		{
			name:      "enum_parameter_default",
			expectErr: false,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: enum
    valid_values:
     - one
     - two
    default: one
pipeline:
`,
		},
		{
			name:      "enum_parameter_default_invalid",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: enum
    valid_values:
     - one
     - two
    default: three
pipeline:
`,
		},
		{
			name:      "enum_parameter_no_valid_values",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: enum
pipeline:
`,
		},
		{
			name:      "default_invalid",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: int
    default: {}
pipeline:
`,
		},
		{
			name:      "non_enum_valid_values",
			expectErr: true,
			template: `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: The thing of the thing
    type: int
    valid_values: [1, 2, 3]
pipeline:
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPlugin(tc.name, []byte(tc.template))
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRenderWithMissingRequired(t *testing.T) {
	template := `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: A parameter
    type: int
    required: true
pipeline:
`

	plugin, err := NewPlugin("plugin", []byte(template))
	require.NoError(t, err)
	_, err = plugin.Render(map[string]interface{}{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required parameter for plugin")
}

func TestRenderWithInvalidParameter(t *testing.T) {
	template := `version: 0.0.0
title: Test Plugin
description: This is a test plugin
parameters:
  - name: path
    label: Parameter
    description: A parameter
    type: int
    required: true
pipeline:
`
	plugin, err := NewPlugin("plugin", []byte(template))
	require.NoError(t, err)
	_, err = plugin.Render(map[string]interface{}{"path": "test"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin parameter failed validation")
}

func TestDefaultPluginFuncWithValue(t *testing.T) {
	result := defaultPluginFunc("default_value", "supplied_value")
	require.Equal(t, "supplied_value", result)
}

func TestDefaultPluginFuncWithoutValue(t *testing.T) {
	result := defaultPluginFunc("default_value", nil)
	require.Equal(t, "default_value", result)
}

func TestSplitPluginFile(t *testing.T) {
	cases := map[int]struct {
		input            string
		expectedMetadata string
		expectedTemplate string
		expectError      bool
	}{
		0: {
			"pipeline:\n",
			"",
			"pipeline:\n",
			false,
		},
		1: {
			"parameters:\npipeline:\n",
			"parameters:\n",
			"pipeline:\n",
			false,
		},
		2: {
			`
parameters:
  - name: my_param
		type: string
		required: false
pipeline:
  - type: stdout
`,
			`
parameters:
  - name: my_param
		type: string
		required: false
`,
			`pipeline:
  - type: stdout
`,
			false,
		},
		3: {
			`
parameters:
  - name: my_param
		type: string
		required: false

# Defaults
# {{ $output := true }}

pipeline:
  - type: stdout
`,
			`
parameters:
  - name: my_param
		type: string
		required: false
`,
			`
# Defaults
# {{ $output := true }}

pipeline:
  - type: stdout
`,
			false,
		},
	}

	for i, tc := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			meta, template, err := splitPluginFile([]byte(tc.input))
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedMetadata, string(meta))
			require.Equal(t, tc.expectedTemplate, string(template))
		})
	}
}
