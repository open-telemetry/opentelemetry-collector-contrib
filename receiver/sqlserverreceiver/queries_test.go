// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryIODBWithoutInstanceName(t *testing.T) {
	expectedBytes, err := os.ReadFile(path.Join("./testdata", "databaseIOQueryWithoutInstanceName.txt"))
	require.NoError(t, err)
	// Replace all will fix newlines when testing on Windows
	expected := strings.ReplaceAll(string(expectedBytes), "\r\n", "\n")

	actual := getSQLServerDatabaseIOQuery("")

	require.Equal(t, expected, actual)
}

func TestQueryIODBWithInstanceName(t *testing.T) {
	expectedBytes, err := os.ReadFile(path.Join("./testdata", "databaseIOQueryWithInstanceName.txt"))
	require.NoError(t, err)
	// Replace all will fix newlines when testing on Windows
	expected := strings.ReplaceAll(string(expectedBytes), "\r\n", "\n")

	actual := getSQLServerDatabaseIOQuery("instanceName")

	require.Equal(t, expected, actual)
}
