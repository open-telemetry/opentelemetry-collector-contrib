// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryIODBWithoutInstanceName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Test is failing on Windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32519")
	}

	expected, err := os.ReadFile(path.Join("./testdata", "databaseIOQueryWithoutInstanceName.txt"))
	require.NoError(t, err)

	actual := getSQLServerDatabaseIOQuery("")

	require.Equal(t, string(expected), actual)
}

func TestQueryIODBWithInstanceName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Test is failing on Windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32519")
	}

	expected, err := os.ReadFile(path.Join("./testdata", "databaseIOQueryWithInstanceName.txt"))
	require.NoError(t, err)

	actual := getSQLServerDatabaseIOQuery("instanceName")

	require.Equal(t, string(expected), actual)
}
