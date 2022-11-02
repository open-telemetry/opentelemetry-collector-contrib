// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// skipping windows to avoid this golang bug: https://github.com/golang/go/issues/51442
//go:build !windows

package cfgmetadatagen

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
)

func TestZipWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "myfile.zip")
	file, err := os.Create(path)
	require.NoError(t, err)
	writer := zip.NewWriter(file)
	zw := zipWriter{zw: writer}
	cfgInfo := getTestCfgInfo(t)
	err = zw.write(cfgInfo, []byte("hello"))
	require.NoError(t, err)
	err = zw.close()
	require.NoError(t, err)
	opened, err := os.Open(path)
	require.NoError(t, err)
	stat, err := os.Stat(path)
	require.NoError(t, err)
	reader, err := zip.NewReader(opened, stat.Size())
	require.NoError(t, err)
	f := reader.File[0]
	rf, err := f.Open()
	require.NoError(t, err)
	bytes, err := io.ReadAll(rf)
	require.NoError(t, err)
	assert.EqualValues(t, "hello", bytes)
}

func getTestCfgInfo(t *testing.T) configschema.CfgInfo {
	c, err := components.Components()
	require.NoError(t, err)
	cfgInfo, err := configschema.GetCfgInfo(c, "receiver", "redis")
	require.NoError(t, err)
	return cfgInfo
}
