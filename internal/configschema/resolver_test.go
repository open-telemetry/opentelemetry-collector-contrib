// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func testTypeToPackagePath(t *testing.T, v any) string {
	packageDir, err := testDR().TypeToPackagePath(reflect.ValueOf(v).Type())
	require.NoError(t, err)
	return packageDir
}
