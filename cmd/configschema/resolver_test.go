// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	dir := testDR().ReflectValueToProjectPath(reflect.ValueOf(&redisreceiver.Config{}))
	assert.Equal(t, "../../receiver/redisreceiver", dir)
}

func TestTypetoProjectPath_External(t *testing.T) {
	dir := testDR().ReflectValueToProjectPath(reflect.ValueOf(&otlpreceiver.Config{}))
	assert.Equal(t, "", dir)
}

func testTypeToPackagePath(t *testing.T, v interface{}) string {
	packageDir, err := testDR().TypeToPackagePath(reflect.ValueOf(v).Type())
	require.NoError(t, err)
	return packageDir
}
