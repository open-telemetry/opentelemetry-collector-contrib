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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestBuildAgentSuccess(t *testing.T) {
	mockCfg := Config{}
	mockLogger := zap.NewNop().Sugar()
	mockPluginDir := "/some/path/plugins"
	mockDatabaseFile := filepath.Join(testutil.NewTempDir(t), "test.db")
	mockNamespace := "mynamespace"
	mockOutput := testutil.NewFakeOutput(t)

	agent, err := NewBuilder(mockLogger).
		WithConfig(&mockCfg).
		WithPluginDir(mockPluginDir).
		WithDefaultOutput(mockOutput).
		WithDatabaseFile(mockDatabaseFile, mockNamespace).
		Build()
	require.NoError(t, err)
	require.Equal(t, mockLogger, agent.SugaredLogger)
}

func TestBuildAgentFailureOnDatabase(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	invalidDatabaseFile := filepath.Join(tempDir, "test.db")
	err := ioutil.WriteFile(invalidDatabaseFile, []byte("invalid"), 0755)
	require.NoError(t, err)

	mockCfg := Config{}
	mockLogger := zap.NewNop().Sugar()
	mockPluginDir := "/some/path/plugins"
	mockDatabaseFile := invalidDatabaseFile
	mockNamespace := "mynamespace"
	mockOutput := testutil.NewFakeOutput(t)

	agent, err := NewBuilder(mockLogger).
		WithConfig(&mockCfg).
		WithPluginDir(mockPluginDir).
		WithDatabaseFile(mockDatabaseFile, mockNamespace).
		WithDefaultOutput(mockOutput).
		Build()
	require.Error(t, err)
	require.Nil(t, agent)
}

func TestBuildAgentFailureOnNamespace(t *testing.T) {
	mockCfg := Config{}
	mockLogger := zap.NewNop().Sugar()
	mockPluginDir := "/some/path/plugins"
	mockDatabaseFile := filepath.Join(testutil.NewTempDir(t), "test.db")
	mockNamespace := ""
	mockOutput := testutil.NewFakeOutput(t)

	_, err := NewBuilder(mockLogger).
		WithConfig(&mockCfg).
		WithPluginDir(mockPluginDir).
		WithDatabaseFile(mockDatabaseFile, mockNamespace).
		WithDefaultOutput(mockOutput).
		Build()
	require.Error(t, err)
}

func TestBuildAgentFailureOnPluginRegistry(t *testing.T) {
	mockCfg := Config{}
	mockLogger := zap.NewNop().Sugar()
	mockPluginDir := "[]"
	mockOutput := testutil.NewFakeOutput(t)

	_, err := NewBuilder(mockLogger).
		WithConfig(&mockCfg).
		WithPluginDir(mockPluginDir).
		WithDefaultOutput(mockOutput).
		Build()
	require.NoError(t, err)
}

func TestBuildAgentFailureNoConfigOrGlobs(t *testing.T) {
	mockLogger := zap.NewNop().Sugar()
	mockPluginDir := "/some/plugin/path"
	mockOutput := testutil.NewFakeOutput(t)

	agent, err := NewBuilder(mockLogger).
		WithPluginDir(mockPluginDir).
		WithDefaultOutput(mockOutput).
		Build()
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be built without")
	require.Nil(t, agent)
}

func TestBuildAgentFailureWithBothConfigAndGlobs(t *testing.T) {
	mockCfg := Config{}
	mockLogger := zap.NewNop().Sugar()
	mockPluginDir := "/some/plugin/path"
	mockOutput := testutil.NewFakeOutput(t)

	agent, err := NewBuilder(mockLogger).
		WithConfig(&mockCfg).
		WithConfigFiles([]string{"test"}).
		WithPluginDir(mockPluginDir).
		WithDefaultOutput(mockOutput).
		Build()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not both")
	require.Nil(t, agent)
}

func TestBuildAgentFailureNonexistGlobs(t *testing.T) {
	mockLogger := zap.NewNop().Sugar()
	mockPluginDir := "/some/plugin/path"
	mockOutput := testutil.NewFakeOutput(t)

	agent, err := NewBuilder(mockLogger).
		WithConfigFiles([]string{"/tmp/nonexist"}).
		WithPluginDir(mockPluginDir).
		WithDefaultOutput(mockOutput).
		Build()
	require.Error(t, err)
	require.Contains(t, err.Error(), "read configs from globs")
	require.Nil(t, agent)
}
