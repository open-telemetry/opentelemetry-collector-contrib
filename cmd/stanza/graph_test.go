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

package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/opentelemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/require"
)

func graphTest(config, output string) func(t *testing.T) {
	return func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		configPath := filepath.Join(tempDir, "config.yaml")
		err = ioutil.WriteFile(configPath, []byte(config), 0666)
		require.NoError(t, err)

		rootFlags := &RootFlags{
			ConfigFiles: []string{configPath},
		}
		graphCmd := NewGraphCommand(rootFlags)

		// replace stdout
		buf := bytes.NewBuffer([]byte{})
		stdout = buf

		err = graphCmd.Execute()
		require.NoError(t, err)

		require.Equal(t, testutil.Trim(output), testutil.Trim(buf.String()))
	}
}

func TestGraphSimple(t *testing.T) {
	config := `
pipeline:
  - id: generate
    type: generate_input
    output: json_parser
    entry:
      record:
        test: value

  - id: json_parser
    type: json_parser
    output: google_cloud

  - id: google_cloud
    project_id: testproject
    type: google_cloud_output
`

	expected := `
    strict digraph G {
      // Node definitions.
      "$.json_parser";
      "$.generate";
      "$.google_cloud";

      // Edge definitions.
      "$.json_parser" -> "$.google_cloud";
      "$.generate" -> "$.json_parser";
    }`

	graphTest(config, expected)(t)
}

func TestGraphNoIDs(t *testing.T) {
	config := `
pipeline:
  - type: generate_input
    output: json_parser
    entry:
      record:
        test: value

  - type: json_parser
    output: google_cloud_output

  - project_id: testproject
    type: google_cloud_output
`

	expected := `
    strict digraph G {
      // Node definitions.
      "$.json_parser";
      "$.google_cloud_output";
      "$.generate_input";

      // Edge definitions.
      "$.json_parser" -> "$.google_cloud_output";
      "$.generate_input" -> "$.json_parser";
    }`

	graphTest(config, expected)(t)
}

func TestGraphNoOutputs(t *testing.T) {
	config := `
pipeline:
  - id: generate
    type: generate_input
    entry:
      record:
        test: value

  - id: json_parser
    type: json_parser

  - id: google_cloud
    project_id: testproject
    type: google_cloud_output
`

	expected := `
    strict digraph G {
      // Node definitions.
      "$.json_parser";
      "$.generate";
      "$.google_cloud";

      // Edge definitions.
      "$.json_parser" -> "$.google_cloud";
      "$.generate" -> "$.json_parser";
    }`

	graphTest(config, expected)(t)
}

func TestGraphNoOutputsNoIDs(t *testing.T) {
	config := `
pipeline:
  - type: generate_input
    entry:
      record:
        test: value

  - type: json_parser

  - project_id: testproject
    type: google_cloud_output
`

	expected := `
    strict digraph G {
      // Node definitions.
      "$.json_parser";
      "$.google_cloud_output";
      "$.generate_input";

      // Edge definitions.
      "$.json_parser" -> "$.google_cloud_output";
      "$.generate_input" -> "$.json_parser";
    }`

	graphTest(config, expected)(t)
}

func TestGraphMixed(t *testing.T) {
	config := `
pipeline:
  - type: generate_input
    entry:
      record:
        test: value

  - type: json_parser
    output: my_stdout

  - project_id: testproject
    type: google_cloud_output

  - id: my_stdout
    type: stdout
`

	expected := `
    strict digraph G {
      // Node definitions.
      "$.json_parser";
      "$.google_cloud_output";
      "$.my_stdout";
      "$.generate_input";

      // Edge definitions.
      "$.json_parser" -> "$.my_stdout";
      "$.generate_input" -> "$.json_parser";
    }`

	graphTest(config, expected)(t)
}
