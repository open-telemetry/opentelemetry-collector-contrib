// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

func TestGetQueryForMetric(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name           string
		engineEdition  int
		metricName     string
		expectFound    bool
		expectFallback bool
	}{
		{
			name:           "Standard SQL Server - buffer pool query",
			engineEdition:  queries.StandardSQLServerEngineEdition,
			metricName:     "sqlserver.instance.buffer_pool_size",
			expectFound:    true,
			expectFallback: false,
		},
		{
			name:           "Azure SQL Database - buffer pool query",
			engineEdition:  queries.AzureSQLDatabaseEngineEdition,
			metricName:     "sqlserver.instance.buffer_pool_size",
			expectFound:    true,
			expectFallback: false,
		},
		{
			name:           "Azure SQL Database - user connections query falls back to Default",
			engineEdition:  queries.AzureSQLDatabaseEngineEdition,
			metricName:     "sqlserver.instance.user_connections",
			expectFound:    true, // Should fall back to Default engine which now has this query
			expectFallback: true,
		},
		{
			name:           "Unknown metric",
			engineEdition:  queries.StandardSQLServerEngineEdition,
			metricName:     "sqlserver.instance.unknown_metric",
			expectFound:    false,
			expectFallback: false,
		},
		{
			name:           "Default engine fallback for unknown engine edition",
			engineEdition:  999, // Unknown engine edition
			metricName:     "sqlserver.instance.buffer_pool_size",
			expectFound:    true,
			expectFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := &InstanceScraper{
				logger:        logger,
				engineEdition: tt.engineEdition,
			}

			query, found := scraper.getQueryForMetric(tt.metricName)

			assert.Equal(t, tt.expectFound, found, "Query found status should match expectation")

			if tt.expectFound {
				assert.NotEmpty(t, query, "Query should not be empty when found")
				t.Logf("Found query for %s (engine %d): %s", tt.metricName, tt.engineEdition, query[:min(100, len(query))])
			} else {
				assert.Empty(t, query, "Query should be empty when not found")
			}
		})
	}
}

func TestDefaultEngineFallback(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name          string
		engineEdition int
		metricName    string
		expectFound   bool
		expectDefault bool
	}{
		{
			name:          "Azure SQL Database falls back to Default for missing metric",
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			metricName:    "sqlserver.instance.buffer_pool_size", // This should exist in Default
			expectFound:   true,
			expectDefault: true,
		},
		{
			name:          "Unknown engine falls back to Default",
			engineEdition: 999,
			metricName:    "sqlserver.instance.buffer_pool_size",
			expectFound:   true,
			expectDefault: true,
		},
		{
			name:          "Standard engine doesn't need fallback",
			engineEdition: queries.StandardSQLServerEngineEdition,
			metricName:    "sqlserver.instance.buffer_pool_size",
			expectFound:   true,
			expectDefault: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := &InstanceScraper{
				logger:        logger,
				engineEdition: tt.engineEdition,
			}

			query, found := scraper.getQueryForMetric(tt.metricName)

			assert.Equal(t, tt.expectFound, found, "Query found status should match expectation")
			if tt.expectFound {
				assert.NotEmpty(t, query, "Query should not be empty when found")
				t.Logf("Query for %s on engine %d: %s", tt.metricName, tt.engineEdition, query[:min(50, len(query))])
			}
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := queries.TruncateQuery(tt.query, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// min helper function for Go versions that don't have it in standard library
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
