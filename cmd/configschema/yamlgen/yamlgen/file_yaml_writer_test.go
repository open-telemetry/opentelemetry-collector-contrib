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

package yamlgen

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileYAMLWriter(t *testing.T) {
	dir := t.TempDir()
	w := fileYAMLWriter{fakeDirResolver{dir}}
	err := w.write(getTestCfgInfo(t), []byte("hello"))
	require.NoError(t, err)
	file, err := os.Open(filepath.Join(dir, "cfgschema.yaml"))
	require.NoError(t, err)
	bytes, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.EqualValues(t, "hello", bytes)
}

type fakeDirResolver struct {
	path string
}

func (dr fakeDirResolver) TypeToPackagePath(t reflect.Type) (string, error) {
	return dr.path, nil
}

func (dr fakeDirResolver) ReflectValueToProjectPath(v reflect.Value) string {
	return dr.path
}
