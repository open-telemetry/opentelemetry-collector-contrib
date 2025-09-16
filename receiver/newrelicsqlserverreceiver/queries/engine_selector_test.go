// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package queries

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEngineTypeName(t *testing.T) {
	tests := []struct {
		name          string
		engineEdition int
		expectedName  string
	}{
		{
			name:          "Standard SQL Server",
			engineEdition: StandardSQLServerEngineEdition,
			expectedName:  "Standard SQL Server",
		},
		{
			name:          "Azure SQL Database",
			engineEdition: AzureSQLDatabaseEngineEdition,
			expectedName:  "Azure SQL Database",
		},
		{
			name:          "Azure SQL Managed Instance",
			engineEdition: AzureSQLManagedInstanceEngineEdition,
			expectedName:  "Azure SQL Managed Instance",
		},
		{
			name:          "Unknown engine edition",
			engineEdition: 999,
			expectedName:  "Unknown Engine Edition (999)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetEngineTypeName(tt.engineEdition)
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

func TestTruncateQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		maxLen   int
		expected string
	}{
		{
			name:     "Short query not truncated",
			query:    "SELECT 1",
			maxLen:   20,
			expected: "SELECT 1",
		},
		{
			name:     "Long query truncated",
			query:    "SELECT * FROM sys.dm_os_buffer_descriptors WHERE database_id <> 32767",
			maxLen:   20,
			expected: "SELECT * FROM sys.dm...",
		},
		{
			name:     "Query exactly at limit",
			query:    "SELECT COUNT(*)",
			maxLen:   15, // Length of "SELECT COUNT(*)" is 15
			expected: "SELECT COUNT(*)",
		},
		{
			name:     "Empty query",
			query:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "Zero maxLen",
			query:    "SELECT 1",
			maxLen:   0,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateQuery(tt.query, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetQueryForMetric(t *testing.T) {
	tests := []struct {
		name          string
		defType       QueryDefinitionType
		metricName    string
		engineEdition int
		expectFound   bool
	}{
		{
			name:          "Standard SQL Server - buffer pool query",
			defType:       InstanceQueries,
			metricName:    "sqlserver.instance.buffer_pool_size",
			engineEdition: StandardSQLServerEngineEdition,
			expectFound:   true,
		},
		{
			name:          "Azure SQL Database - buffer pool query",
			defType:       InstanceQueries,
			metricName:    "sqlserver.instance.buffer_pool_size",
			engineEdition: AzureSQLDatabaseEngineEdition,
			expectFound:   true,
		},
		{
			name:          "Unknown metric",
			defType:       InstanceQueries,
			metricName:    "sqlserver.unknown.metric",
			engineEdition: StandardSQLServerEngineEdition,
			expectFound:   false,
		},
		{
			name:          "Unknown engine falls back to Default",
			defType:       InstanceQueries,
			metricName:    "sqlserver.instance.buffer_pool_size",
			engineEdition: 999,
			expectFound:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, found := GetQueryForMetric(tt.defType, tt.metricName, tt.engineEdition)
			assert.Equal(t, tt.expectFound, found, "Query found status should match expectation")
			if tt.expectFound {
				assert.NotEmpty(t, query, "Query should not be empty when found")
			} else {
				assert.Empty(t, query, "Query should be empty when not found")
			}
		})
	}
}
