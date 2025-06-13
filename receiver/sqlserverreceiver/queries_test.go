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

func TestQueryContents(t *testing.T) {
	queryTests := []struct {
		name                     string
		instanceName             string
		getQuery                 func(string) string
		expectedQueryValFilename string
	}{
		{
			name:                     "Test database IO query without instance name",
			instanceName:             "",
			getQuery:                 getSQLServerDatabaseIOQuery,
			expectedQueryValFilename: "databaseIOQueryWithoutInstanceName.txt",
		},
		{
			name:                     "Test database IO query with instance name",
			instanceName:             "instanceName",
			getQuery:                 getSQLServerDatabaseIOQuery,
			expectedQueryValFilename: "databaseIOQueryWithInstanceName.txt",
		},
		{
			name:                     "Test perf counter query without instance name",
			instanceName:             "",
			getQuery:                 getSQLServerPerformanceCounterQuery,
			expectedQueryValFilename: "perfCounterQueryWithoutInstanceName.txt",
		},
		{
			name:                     "Test perf counter query with instance name",
			instanceName:             "instanceName",
			getQuery:                 getSQLServerPerformanceCounterQuery,
			expectedQueryValFilename: "perfCounterQueryWithInstanceName.txt",
		},
		{
			name:                     "Test properties query without instance name",
			instanceName:             "",
			getQuery:                 getSQLServerPropertiesQuery,
			expectedQueryValFilename: "propertyQueryWithoutInstanceName.txt",
		},
		{
			name:                     "Test properties query with instance name",
			instanceName:             "instanceName",
			getQuery:                 getSQLServerPropertiesQuery,
			expectedQueryValFilename: "propertyQueryWithInstanceName.txt",
		},
	}

	for _, tt := range queryTests {
		t.Run(tt.name, func(t *testing.T) {
			expectedBytes, err := os.ReadFile(path.Join("./testdata", tt.expectedQueryValFilename))
			require.NoError(t, err)
			// Replace all will fix newlines when testing on Windows
			expected := strings.ReplaceAll(string(expectedBytes), "\r\n", "\n")

			actual := tt.getQuery(tt.instanceName)
			require.Equal(t, expected, actual)
		})
	}
}

func TestQueryTextAndPlanQueryContents(t *testing.T) {
	queryTests := []struct {
		name                     string
		instanceName             string
		maxQuerySampleCount      uint
		lookbackTime             uint
		getQuery                 func() string
		expectedQueryValFilename string
	}{
		{
			name:                     "Test query text and query plan",
			instanceName:             "",
			maxQuerySampleCount:      1000,
			lookbackTime:             60,
			getQuery:                 getSQLServerQueryTextAndPlanQuery,
			expectedQueryValFilename: "databaseTopQueryWithoutInstanceName.txt",
		},
	}

	for _, tt := range queryTests {
		t.Run(tt.name, func(t *testing.T) {
			expected, err := os.ReadFile(path.Join("./testdata", tt.expectedQueryValFilename))
			require.NoError(t, err)
			actual := tt.getQuery()
			require.NoError(t, err)
			require.Equal(t, strings.TrimSpace(string(expected)), strings.TrimSpace(actual))
		})
	}
}

func TestGetSQLServerQuerySamplesQuery(t *testing.T) {
	queryTests := []struct {
		name                     string
		instanceName             string
		getQuery                 func() string
		expectedQueryValFilename string
		maxRowsPerQuery          uint64
	}{
		{
			name:                     "Test query sample query",
			instanceName:             "",
			maxRowsPerQuery:          1000,
			getQuery:                 getSQLServerQuerySamplesQuery,
			expectedQueryValFilename: "testQuerySampleQuery.txt",
		},
	}

	for _, tt := range queryTests {
		t.Run(tt.name, func(t *testing.T) {
			expectedBytes, err := os.ReadFile(path.Join("./testdata", tt.expectedQueryValFilename))
			require.NoError(t, err)
			// Replace all will fix newlines when testing on Windows
			expected := strings.ReplaceAll(string(expectedBytes), "\r\n", "\n")
			actual := strings.ReplaceAll(tt.getQuery(), "\r\n", "\n")
			require.Equal(t, expected, actual)
		})
	}
}
