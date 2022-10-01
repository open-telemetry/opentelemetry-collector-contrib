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

// Skip tests on Windows temporarily, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11451
//go:build !windows
// +build !windows

package configschema

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"
)

const gcpCollectorPath = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"

func TestTokensToPartialPath(t *testing.T) {
	path, err := requireTokensToPartialPath([]string{gcpCollectorPath, "42"})
	require.NoError(t, err)
	assert.Equal(t, "github.com/!google!cloud!platform/opentelemetry-operations-go/exporter/collector@42", path)
}

func TestPackagePathToGoPath(t *testing.T) {
	dr := testDR()
	dir, err := dr.packagePathToGoPath(gcpCollectorPath)
	require.NoError(t, err)
	_, err = os.ReadDir(dir)
	require.NoError(t, err)
}

func TestPackagePathToGoPath_Error(t *testing.T) {
	_, err := testDR().packagePathToGoPath("foo/bar")
	require.Error(t, err)
}

func TestTypeToPackagePath_Local(t *testing.T) {
	packageDir := testTypeToPackagePath(t, redisreceiver.Config{})
	assert.Equal(t, filepath.Join("..", "..", "receiver", "redisreceiver"), packageDir)
}

func TestTypeToPackagePath_External(t *testing.T) {
	packageDir := testTypeToPackagePath(t, otlpreceiver.Config{})
	assert.Contains(t, packageDir, "pkg/mod/go.opentelemetry.io/collector@")
}

func TestTypeToPackagePath_Error(t *testing.T) {
	dr := NewDirResolver("foo/bar", DefaultModule)
	_, err := dr.TypeToPackagePath(reflect.ValueOf(redisreceiver.Config{}).Type())
	require.Error(t, err)
}

func TestTypeToProjectPath(t *testing.T) {
	dir := testDR().TypeToProjectPath(reflect.ValueOf(redisreceiver.Config{}).Type())
	assert.Equal(t, "../../receiver/redisreceiver", dir)
}

func TestTypetoProjectPath_External(t *testing.T) {
	dir := testDR().TypeToProjectPath(reflect.ValueOf(otlpreceiver.Config{}).Type())
	assert.Equal(t, "", dir)
}

func testTypeToPackagePath(t *testing.T, v interface{}) string {
	packageDir, err := testDR().TypeToPackagePath(reflect.ValueOf(v).Type())
	require.NoError(t, err)
	return packageDir
}
