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

package agent

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	_ "github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/transformer/noop"
	"github.com/open-telemetry/opentelemetry-log-collection/pipeline"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestNewConfigFromFile(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	configFile := filepath.Join(tempDir, "config.yaml")
	configContents := `
pipeline:
  - type: noop
`
	err := ioutil.WriteFile(configFile, []byte(configContents), 0755)
	require.NoError(t, err)

	config, err := NewConfigFromFile(configFile)
	require.NoError(t, err)
	require.Equal(t, len(config.Pipeline), 1)
}

func TestNewConfigWithMissingFile(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	configFile := filepath.Join(tempDir, "config.yaml")

	_, err := NewConfigFromFile(configFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not find config file")
}

func TestNewConfigWithInvalidYAML(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	configFile := filepath.Join(tempDir, "config.yaml")
	configContents := `
pipeline:
  invalid: structure
`
	err := ioutil.WriteFile(configFile, []byte(configContents), 0755)
	require.NoError(t, err)

	_, err = NewConfigFromFile(configFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read config file as yaml")
}

func TestNewConfigFromGlobs(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	configFile := filepath.Join(tempDir, "config.yaml")
	configContents := `
pipeline:
  - type: noop
`
	err := ioutil.WriteFile(configFile, []byte(configContents), 0755)
	require.NoError(t, err)

	globs := []string{filepath.Join(tempDir, "*.yaml")}
	config, err := NewConfigFromGlobs(globs)
	require.NoError(t, err)
	require.Equal(t, len(config.Pipeline), 1)
}

func TestNewConfigFromGlobsWithInvalidGlob(t *testing.T) {
	globs := []string{"[]"}
	_, err := NewConfigFromGlobs(globs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "syntax error in pattern")
}

func TestNewConfigFromGlobsWithNoMatches(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	globs := []string{filepath.Join(tempDir, "*.yaml")}
	_, err := NewConfigFromGlobs(globs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "No config files found")
}

func TestNewConfigFromGlobsWithInvalidConfig(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	configFile := filepath.Join(tempDir, "config.yaml")
	configContents := `
pipeline:
  invalid: structure
`
	err := ioutil.WriteFile(configFile, []byte(configContents), 0755)
	require.NoError(t, err)

	globs := []string{filepath.Join(tempDir, "*.yaml")}
	_, err = NewConfigFromGlobs(globs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read config file as yaml")
}

func TestMergeConfigs(t *testing.T) {
	config1 := Config{
		Pipeline: pipeline.Config{
			operator.Config{},
		},
	}

	config2 := Config{
		Pipeline: pipeline.Config{
			operator.Config{},
		},
	}

	config3 := mergeConfigs(&config1, &config2)
	require.Equal(t, len(config3.Pipeline), 2)
}
