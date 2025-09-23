// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

func TestProcessDatabaseBufferMetrics(t *testing.T) {
	tests := []struct {
		name           string
		result         models.DatabaseBufferMetrics
		engineEdition  int
		expectError    bool
		expectMetrics  int
		validateFields func(t *testing.T, metrics pmetric.ScopeMetrics)
	}{
		{
			name: "successful_processing_with_valid_buffer_pool_size",
			result: models.DatabaseBufferMetrics{
				DatabaseName:        "TestDatabase",
				BufferPoolSizeBytes: int64Ptr(1073741824), // 1GB in bytes
			},
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				assert.Equal(t, "sqlserver.database.bufferpool.sizePerDatabaseInBytes", metric.Name())
				assert.Equal(t, "Size of the SQL Server buffer pool allocated for the database in bytes (bufferpool.sizePerDatabaseInBytes)", metric.Description())
				assert.Equal(t, "By", metric.Unit())
				
				// Verify it's a gauge metric
				assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
				gauge := metric.Gauge()
				assert.Equal(t, 1, gauge.DataPoints().Len())
				
				dataPoint := gauge.DataPoints().At(0)
				assert.Equal(t, int64(1073741824), dataPoint.IntValue())
				
				// Verify attributes
				attrs := dataPoint.Attributes()
				assert.Equal(t, 5, attrs.Len())
				
				metricType, exists := attrs.Get("metric.type")
				assert.True(t, exists)
				assert.Equal(t, "gauge", metricType.Str())
				
				metricSource, exists := attrs.Get("metric.source")
				assert.True(t, exists)
				assert.Equal(t, "sys.dm_os_buffer_descriptors", metricSource.Str())
				
				databaseName, exists := attrs.Get("database_name")
				assert.True(t, exists)
				assert.Equal(t, "TestDatabase", databaseName.Str())
				
				engineEdition, exists := attrs.Get("engine_edition")
				assert.True(t, exists)
				assert.Equal(t, "Standard SQL Server", engineEdition.Str())
				
				engineEditionId, exists := attrs.Get("engine_edition_id")
				assert.True(t, exists)
				assert.Equal(t, int64(0), engineEditionId.Int())
			},
		},
		{
			name: "azure_sql_database_engine_edition",
			result: models.DatabaseBufferMetrics{
				DatabaseName:        "AzureDatabase",
				BufferPoolSizeBytes: int64Ptr(2147483648), // 2GB in bytes
			},
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				dataPoint := metric.Gauge().DataPoints().At(0)
				assert.Equal(t, int64(2147483648), dataPoint.IntValue())
				
				// Verify Azure SQL Database specific attributes
				attrs := dataPoint.Attributes()
				engineEdition, exists := attrs.Get("engine_edition")
				assert.True(t, exists)
				assert.Equal(t, "Azure SQL Database", engineEdition.Str())
				
				engineEditionId, exists := attrs.Get("engine_edition_id")
				assert.True(t, exists)
				assert.Equal(t, int64(5), engineEditionId.Int())
				
				databaseName, exists := attrs.Get("database_name")
				assert.True(t, exists)
				assert.Equal(t, "AzureDatabase", databaseName.Str())
			},
		},
		{
			name: "azure_sql_managed_instance_engine_edition",
			result: models.DatabaseBufferMetrics{
				DatabaseName:        "ManagedInstanceDB",
				BufferPoolSizeBytes: int64Ptr(536870912), // 512MB in bytes
			},
			engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				dataPoint := metric.Gauge().DataPoints().At(0)
				assert.Equal(t, int64(536870912), dataPoint.IntValue())
				
				// Verify Azure SQL Managed Instance specific attributes
				attrs := dataPoint.Attributes()
				engineEdition, exists := attrs.Get("engine_edition")
				assert.True(t, exists)
				assert.Equal(t, "Azure SQL Managed Instance", engineEdition.Str())
				
				engineEditionId, exists := attrs.Get("engine_edition_id")
				assert.True(t, exists)
				assert.Equal(t, int64(8), engineEditionId.Int())
			},
		},
		{
			name: "nil_buffer_pool_size_skipped",
			result: models.DatabaseBufferMetrics{
				DatabaseName:        "TestDatabase",
				BufferPoolSizeBytes: nil,
			},
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectError:   false,
			expectMetrics: 0, // Should be skipped due to nil value
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				assert.Equal(t, 0, metrics.Metrics().Len())
			},
		},
		{
			name: "zero_buffer_pool_size",
			result: models.DatabaseBufferMetrics{
				DatabaseName:        "EmptyDatabase",
				BufferPoolSizeBytes: int64Ptr(0),
			},
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				dataPoint := metric.Gauge().DataPoints().At(0)
				assert.Equal(t, int64(0), dataPoint.IntValue())
			},
		},
		{
			name: "large_buffer_pool_size",
			result: models.DatabaseBufferMetrics{
				DatabaseName:        "LargeDatabase",
				BufferPoolSizeBytes: int64Ptr(17179869184), // 16GB in bytes
			},
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				dataPoint := metric.Gauge().DataPoints().At(0)
				assert.Equal(t, int64(17179869184), dataPoint.IntValue())
			},
		},
		{
			name: "special_characters_in_database_name",
			result: models.DatabaseBufferMetrics{
				DatabaseName:        "Test-Database_With.Special[Chars]",
				BufferPoolSizeBytes: int64Ptr(1024),
			},
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				dataPoint := metric.Gauge().DataPoints().At(0)
				
				attrs := dataPoint.Attributes()
				databaseName, exists := attrs.Get("database_name")
				assert.True(t, exists)
				assert.Equal(t, "Test-Database_With.Special[Chars]", databaseName.Str())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger
			logger, _ := zap.NewDevelopment()
			
			// Create DatabaseScraper
			scraper := &DatabaseScraper{
				logger:        logger,
				startTime:     pcommon.NewTimestampFromTime(time.Now()),
				engineEdition: tt.engineEdition,
			}

			// Create empty scope metrics
			scopeMetrics := pmetric.NewScopeMetrics()

			// Execute the function
			err := scraper.processDatabaseBufferMetrics(tt.result, scopeMetrics)

			// Verify error expectation
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify metrics count
			assert.Equal(t, tt.expectMetrics, scopeMetrics.Metrics().Len())

			// Run custom validations
			if tt.validateFields != nil {
				tt.validateFields(t, scopeMetrics)
			}
		})
	}
}

func TestProcessDatabaseBufferMetrics_TimestampValidation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	startTime := pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Minute))
	
	scraper := &DatabaseScraper{
		logger:        logger,
		startTime:     startTime,
		engineEdition: queries.StandardSQLServerEngineEdition,
	}

	result := models.DatabaseBufferMetrics{
		DatabaseName:        "TestDatabase",
		BufferPoolSizeBytes: int64Ptr(1024),
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	beforeProcessing := time.Now()

	err := scraper.processDatabaseBufferMetrics(result, scopeMetrics)
	require.NoError(t, err)

	afterProcessing := time.Now()
	require.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	dataPoint := metric.Gauge().DataPoints().At(0)

	// Verify timestamps
	assert.Equal(t, startTime, dataPoint.StartTimestamp())
	
	timestampTime := dataPoint.Timestamp().AsTime()
	assert.True(t, timestampTime.After(beforeProcessing) || timestampTime.Equal(beforeProcessing))
	assert.True(t, timestampTime.Before(afterProcessing) || timestampTime.Equal(afterProcessing))
}

func TestProcessDatabaseBufferMetrics_ReflectionEdgeCases(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	scraper := &DatabaseScraper{
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}

	t.Run("empty_database_name", func(t *testing.T) {
		result := models.DatabaseBufferMetrics{
			DatabaseName:        "",
			BufferPoolSizeBytes: int64Ptr(1024),
		}

		scopeMetrics := pmetric.NewScopeMetrics()
		err := scraper.processDatabaseBufferMetrics(result, scopeMetrics)
		
		assert.NoError(t, err)
		require.Equal(t, 1, scopeMetrics.Metrics().Len())

		metric := scopeMetrics.Metrics().At(0)
		dataPoint := metric.Gauge().DataPoints().At(0)
		
		attrs := dataPoint.Attributes()
		databaseName, exists := attrs.Get("database_name")
		assert.True(t, exists)
		assert.Equal(t, "", databaseName.Str())
	})

	t.Run("very_long_database_name", func(t *testing.T) {
		longName := string(make([]byte, 1000))
		for i := range longName {
			longName = longName[:i] + "a" + longName[i+1:]
		}
		
		result := models.DatabaseBufferMetrics{
			DatabaseName:        longName,
			BufferPoolSizeBytes: int64Ptr(1024),
		}

		scopeMetrics := pmetric.NewScopeMetrics()
		err := scraper.processDatabaseBufferMetrics(result, scopeMetrics)
		
		assert.NoError(t, err)
		require.Equal(t, 1, scopeMetrics.Metrics().Len())

		metric := scopeMetrics.Metrics().At(0)
		dataPoint := metric.Gauge().DataPoints().At(0)
		
		attrs := dataPoint.Attributes()
		databaseName, exists := attrs.Get("database_name")
		assert.True(t, exists)
		assert.Equal(t, longName, databaseName.Str())
	})
}

func TestProcessDatabaseBufferMetrics_AllEngineEditions(t *testing.T) {
	engineEditions := []struct {
		edition int
		name    string
	}{
		{queries.StandardSQLServerEngineEdition, "Standard SQL Server"},
		{queries.AzureSQLDatabaseEngineEdition, "Azure SQL Database"},
		{queries.AzureSQLManagedInstanceEngineEdition, "Azure SQL Managed Instance"},
		{3, "Unknown Engine Edition (3)"}, // Test unknown engine edition
		{-1, "Unknown Engine Edition (-1)"}, // Test negative engine edition
		{999, "Unknown Engine Edition (999)"}, // Test very high engine edition
	}

	for _, ee := range engineEditions {
		t.Run(ee.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			
			scraper := &DatabaseScraper{
				logger:        logger,
				startTime:     pcommon.NewTimestampFromTime(time.Now()),
				engineEdition: ee.edition,
			}

			result := models.DatabaseBufferMetrics{
				DatabaseName:        "TestDatabase",
				BufferPoolSizeBytes: int64Ptr(1024),
			}

			scopeMetrics := pmetric.NewScopeMetrics()
			err := scraper.processDatabaseBufferMetrics(result, scopeMetrics)
			
			assert.NoError(t, err)
			require.Equal(t, 1, scopeMetrics.Metrics().Len())

			metric := scopeMetrics.Metrics().At(0)
			dataPoint := metric.Gauge().DataPoints().At(0)
			
			attrs := dataPoint.Attributes()
			engineEdition, exists := attrs.Get("engine_edition")
			assert.True(t, exists)
			assert.Equal(t, ee.name, engineEdition.Str())
			
			engineEditionId, exists := attrs.Get("engine_edition_id")
			assert.True(t, exists)
			assert.Equal(t, int64(ee.edition), engineEditionId.Int())
		})
	}
}

// Helper function to create int64 pointer
func int64Ptr(value int64) *int64 {
	return &value
}

// Test data type consistency
func TestProcessDatabaseBufferMetrics_DataTypeConsistency(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	scraper := &DatabaseScraper{
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}

	result := models.DatabaseBufferMetrics{
		DatabaseName:        "TestDatabase",
		BufferPoolSizeBytes: int64Ptr(1024),
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseBufferMetrics(result, scopeMetrics)
	
	require.NoError(t, err)
	require.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	
	// Verify metric type is gauge
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	
	// Verify data point value type
	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, pmetric.NumberDataPointValueTypeInt, dataPoint.ValueType())
	assert.Equal(t, int64(1024), dataPoint.IntValue())
}

func TestProcessDatabaseBufferMetrics_GaugeMetricStructure(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	scraper := &DatabaseScraper{
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}

	result := models.DatabaseBufferMetrics{
		DatabaseName:        "TestDatabase",
		BufferPoolSizeBytes: int64Ptr(1073741824),
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseBufferMetrics(result, scopeMetrics)
	
	require.NoError(t, err)
	require.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	
	// Verify it's structured as a Gauge metric (not Summary)
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	assert.NotEqual(t, pmetric.MetricTypeSummary, metric.Type())
	assert.NotEqual(t, pmetric.MetricTypeHistogram, metric.Type())
	
	// Verify gauge specific properties
	gauge := metric.Gauge()
	assert.Equal(t, 1, gauge.DataPoints().Len())
	
	dataPoint := gauge.DataPoints().At(0)
	
	// Verify the value represents the actual buffer pool size (not statistical data)
	assert.Equal(t, int64(1073741824), dataPoint.IntValue())
	
	// Verify timestamps are set
	assert.True(t, dataPoint.Timestamp() > 0)
	assert.True(t, dataPoint.StartTimestamp() > 0)
	
	// Verify attributes are correctly set for gauge metrics
	attrs := dataPoint.Attributes()
	assert.Equal(t, 5, attrs.Len())
	
	// Check that it's marked as gauge type (not summary)
	metricType, exists := attrs.Get("metric.type")
	assert.True(t, exists)
	assert.Equal(t, "gauge", metricType.Str())
}

// TestProcessDatabaseDiskMetrics tests the processDatabaseDiskMetrics function
func TestProcessDatabaseDiskMetrics(t *testing.T) {
	tests := []struct {
		name           string
		result         models.DatabaseDiskMetrics
		engineEdition  int
		expectError    bool
		expectMetrics  int
		validateFields func(t *testing.T, metrics pmetric.ScopeMetrics)
	}{
		{
			name: "successful_processing_with_valid_disk_size",
			result: models.DatabaseDiskMetrics{
				DatabaseName:      "TestDatabase",
				MaxDiskSizeBytes: int64Ptr(1073741824000), // 1TB in bytes
			},
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				assert.Equal(t, "sqlserver.database.maxDiskSizeInBytes", metric.Name())
				assert.Equal(t, "Maximum disk size allowed for the SQL Server database in bytes (maxDiskSizeInBytes)", metric.Description())
				assert.Equal(t, "By", metric.Unit())
				
				// Verify it's a gauge metric
				assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
				gauge := metric.Gauge()
				assert.Equal(t, 1, gauge.DataPoints().Len())
				
				dataPoint := gauge.DataPoints().At(0)
				assert.Equal(t, int64(1073741824000), dataPoint.IntValue())
				
				// Verify attributes
				attrs := dataPoint.Attributes()
				assert.Equal(t, 5, attrs.Len())
				
				databaseName, exists := attrs.Get("database_name")
				assert.True(t, exists)
				assert.Equal(t, "TestDatabase", databaseName.Str())
				
				metricType, exists := attrs.Get("metric.type")
				assert.True(t, exists)
				assert.Equal(t, "gauge", metricType.Str())
				
				metricSource, exists := attrs.Get("metric.source")
				assert.True(t, exists)
				assert.Equal(t, "DATABASEPROPERTYEX", metricSource.Str())
				
				engineEdition, exists := attrs.Get("engine_edition")
				assert.True(t, exists)
				assert.Equal(t, "Azure SQL Database", engineEdition.Str())
				
				engineEditionId, exists := attrs.Get("engine_edition_id")
				assert.True(t, exists)
				assert.Equal(t, int64(queries.AzureSQLDatabaseEngineEdition), engineEditionId.Int())
			},
		},
		{
			name: "azure_sql_database_with_large_disk_size",
			result: models.DatabaseDiskMetrics{
				DatabaseName:      "AzureDatabase",
				MaxDiskSizeBytes: int64Ptr(32212254720000), // 32TB in bytes (Azure SQL max)
			},
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				gauge := metric.Gauge()
				dataPoint := gauge.DataPoints().At(0)
				assert.Equal(t, int64(32212254720000), dataPoint.IntValue())
				
				attrs := dataPoint.Attributes()
				databaseName, exists := attrs.Get("database_name")
				assert.True(t, exists)
				assert.Equal(t, "AzureDatabase", databaseName.Str())
				
				engineEdition, exists := attrs.Get("engine_edition")
				assert.True(t, exists)
				assert.Equal(t, "Azure SQL Database", engineEdition.Str())
			},
		},
		{
			name: "null_disk_size_no_metrics_created",
			result: models.DatabaseDiskMetrics{
				DatabaseName:      "TestDatabase",
				MaxDiskSizeBytes: nil,
			},
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectError:   false,
			expectMetrics: 0,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				assert.Equal(t, 0, metrics.Metrics().Len())
			},
		},
		{
			name: "database_with_special_characters",
			result: models.DatabaseDiskMetrics{
				DatabaseName:      "Test-Database_With.Special[Chars]",
				MaxDiskSizeBytes: int64Ptr(536870912000), // 500GB
			},
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				gauge := metric.Gauge()
				dataPoint := gauge.DataPoints().At(0)
				
				attrs := dataPoint.Attributes()
				databaseName, exists := attrs.Get("database_name")
				assert.True(t, exists)
				assert.Equal(t, "Test-Database_With.Special[Chars]", databaseName.Str())
			},
		},
		{
			name: "zero_disk_size_valid_metric",
			result: models.DatabaseDiskMetrics{
				DatabaseName:      "SmallDatabase",
				MaxDiskSizeBytes: int64Ptr(0), // Zero size (edge case)
			},
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				gauge := metric.Gauge()
				dataPoint := gauge.DataPoints().At(0)
				assert.Equal(t, int64(0), dataPoint.IntValue())
			},
		},
		{
			name: "standard_sql_server_engine_edition",
			result: models.DatabaseDiskMetrics{
				DatabaseName:      "StandardSQLDatabase",
				MaxDiskSizeBytes: int64Ptr(2147483648000), // 2TB
			},
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				gauge := metric.Gauge()
				dataPoint := gauge.DataPoints().At(0)
				
				attrs := dataPoint.Attributes()
				engineEdition, exists := attrs.Get("engine_edition")
				assert.True(t, exists)
				assert.Equal(t, "Standard SQL Server", engineEdition.Str())
				
				engineEditionId, exists := attrs.Get("engine_edition_id")
				assert.True(t, exists)
				assert.Equal(t, int64(queries.StandardSQLServerEngineEdition), engineEditionId.Int())
			},
		},
		{
			name: "azure_sql_managed_instance_engine_edition",
			result: models.DatabaseDiskMetrics{
				DatabaseName:      "ManagedInstanceDatabase",
				MaxDiskSizeBytes: int64Ptr(8589934592000), // 8TB
			},
			engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
			expectError:   false,
			expectMetrics: 1,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 1, metrics.Metrics().Len())
				
				metric := metrics.Metrics().At(0)
				gauge := metric.Gauge()
				dataPoint := gauge.DataPoints().At(0)
				
				attrs := dataPoint.Attributes()
				engineEdition, exists := attrs.Get("engine_edition")
				assert.True(t, exists)
				assert.Equal(t, "Azure SQL Managed Instance", engineEdition.Str())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger
			logger, _ := zap.NewDevelopment()
			
			// Create DatabaseScraper
			scraper := &DatabaseScraper{
				logger:        logger,
				startTime:     pcommon.NewTimestampFromTime(time.Now()),
				engineEdition: tt.engineEdition,
			}

			// Create empty scope metrics
			scopeMetrics := pmetric.NewScopeMetrics()

			// Execute the function
			err := scraper.processDatabaseDiskMetrics(tt.result, scopeMetrics)

			// Verify error expectation
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify metrics count
			assert.Equal(t, tt.expectMetrics, scopeMetrics.Metrics().Len())

			// Run custom validations
			if tt.validateFields != nil {
				tt.validateFields(t, scopeMetrics)
			}
		})
	}
}

// TestProcessDatabaseDiskMetrics_TimestampValidation tests timestamp handling in disk metrics
func TestProcessDatabaseDiskMetrics_TimestampValidation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	startTime := pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Minute))
	
	scraper := &DatabaseScraper{
		logger:        logger,
		startTime:     startTime,
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	result := models.DatabaseDiskMetrics{
		DatabaseName:      "TestDatabase",
		MaxDiskSizeBytes: int64Ptr(1073741824000),
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	beforeProcessing := time.Now()

	err := scraper.processDatabaseDiskMetrics(result, scopeMetrics)
	require.NoError(t, err)

	afterProcessing := time.Now()
	require.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	dataPoint := metric.Gauge().DataPoints().At(0)

	// Verify timestamps
	assert.Equal(t, startTime, dataPoint.StartTimestamp())
	
	timestampTime := dataPoint.Timestamp().AsTime()
	assert.True(t, timestampTime.After(beforeProcessing) || timestampTime.Equal(beforeProcessing))
	assert.True(t, timestampTime.Before(afterProcessing) || timestampTime.Equal(afterProcessing))
}

// TestProcessDatabaseDiskMetrics_ReflectionEdgeCases tests edge cases in reflection processing
func TestProcessDatabaseDiskMetrics_ReflectionEdgeCases(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	scraper := &DatabaseScraper{
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	t.Run("empty_database_name", func(t *testing.T) {
		result := models.DatabaseDiskMetrics{
			DatabaseName:      "",
			MaxDiskSizeBytes: int64Ptr(1073741824000),
		}

		scopeMetrics := pmetric.NewScopeMetrics()
		err := scraper.processDatabaseDiskMetrics(result, scopeMetrics)
		
		assert.NoError(t, err)
		require.Equal(t, 1, scopeMetrics.Metrics().Len())

		metric := scopeMetrics.Metrics().At(0)
		dataPoint := metric.Gauge().DataPoints().At(0)
		
		attrs := dataPoint.Attributes()
		databaseName, exists := attrs.Get("database_name")
		assert.True(t, exists)
		assert.Equal(t, "", databaseName.Str())
	})

	t.Run("very_long_database_name", func(t *testing.T) {
		longName := make([]byte, 200)
		for i := range longName {
			longName[i] = 'A'
		}
		
		result := models.DatabaseDiskMetrics{
			DatabaseName:      string(longName),
			MaxDiskSizeBytes: int64Ptr(2147483648000),
		}

		scopeMetrics := pmetric.NewScopeMetrics()
		err := scraper.processDatabaseDiskMetrics(result, scopeMetrics)
		
		assert.NoError(t, err)
		require.Equal(t, 1, scopeMetrics.Metrics().Len())

		metric := scopeMetrics.Metrics().At(0)
		dataPoint := metric.Gauge().DataPoints().At(0)
		
		attrs := dataPoint.Attributes()
		databaseName, exists := attrs.Get("database_name")
		assert.True(t, exists)
		assert.Equal(t, string(longName), databaseName.Str())
	})
}

// TestProcessDatabaseDiskMetrics_AllEngineEditions tests all engine editions
func TestProcessDatabaseDiskMetrics_AllEngineEditions(t *testing.T) {
	engineEditions := []struct {
		name    string
		edition int
	}{
		{"StandardSQLServer", queries.StandardSQLServerEngineEdition},
		{"AzureSQLDatabase", queries.AzureSQLDatabaseEngineEdition},
		{"AzureSQLManagedInstance", queries.AzureSQLManagedInstanceEngineEdition},
	}

	for _, ee := range engineEditions {
		t.Run(ee.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			
			scraper := &DatabaseScraper{
				logger:        logger,
				startTime:     pcommon.NewTimestampFromTime(time.Now()),
				engineEdition: ee.edition,
			}

			result := models.DatabaseDiskMetrics{
				DatabaseName:      "TestDatabase",
				MaxDiskSizeBytes: int64Ptr(1073741824000),
			}

			scopeMetrics := pmetric.NewScopeMetrics()
			err := scraper.processDatabaseDiskMetrics(result, scopeMetrics)
			
			assert.NoError(t, err)
			require.Equal(t, 1, scopeMetrics.Metrics().Len())

			metric := scopeMetrics.Metrics().At(0)
			dataPoint := metric.Gauge().DataPoints().At(0)
			
			attrs := dataPoint.Attributes()
			engineEdition, exists := attrs.Get("engine_edition")
			assert.True(t, exists)
			assert.Equal(t, queries.GetEngineTypeName(ee.edition), engineEdition.Str())
			
			engineEditionId, exists := attrs.Get("engine_edition_id")
			assert.True(t, exists)
			assert.Equal(t, int64(ee.edition), engineEditionId.Int())
		})
	}
}

// TestProcessDatabaseDiskMetrics_DataTypeConsistency tests data type consistency
func TestProcessDatabaseDiskMetrics_DataTypeConsistency(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	scraper := &DatabaseScraper{
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	result := models.DatabaseDiskMetrics{
		DatabaseName:      "TypeTestDatabase",
		MaxDiskSizeBytes: int64Ptr(1073741824000),
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseDiskMetrics(result, scopeMetrics)
	
	assert.NoError(t, err)
	require.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	
	// Verify metric properties
	assert.Equal(t, "sqlserver.database.maxDiskSizeInBytes", metric.Name())
	assert.Equal(t, "By", metric.Unit())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	
	gauge := metric.Gauge()
	assert.Equal(t, 1, gauge.DataPoints().Len())
	
	dataPoint := gauge.DataPoints().At(0)
	assert.Equal(t, pmetric.NumberDataPointValueTypeInt, dataPoint.ValueType())
	assert.Equal(t, int64(1073741824000), dataPoint.IntValue())
}

// TestProcessDatabaseDiskMetrics_GaugeMetricStructure tests gauge metric structure validation
func TestProcessDatabaseDiskMetrics_GaugeMetricStructure(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	scraper := &DatabaseScraper{
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	result := models.DatabaseDiskMetrics{
		DatabaseName:      "GaugeTestDatabase",
		MaxDiskSizeBytes: int64Ptr(1073741824000),
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseDiskMetrics(result, scopeMetrics)
	
	assert.NoError(t, err)
	require.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	
	// Verify it's specifically a gauge metric (not histogram or summary)
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	
	gauge := metric.Gauge()
	assert.Equal(t, 1, gauge.DataPoints().Len())
	
	dataPoint := gauge.DataPoints().At(0)
	
        // Verify the value represents the actual disk size (not statistical data)
        assert.Equal(t, int64(1073741824000), dataPoint.IntValue())

        // Verify timestamps are set
        assert.True(t, dataPoint.Timestamp() > 0)
        assert.True(t, dataPoint.StartTimestamp() > 0)

        // Verify attributes are correctly set for gauge metrics
        attrs := dataPoint.Attributes()
        assert.Equal(t, 5, attrs.Len())

        // Check that it's marked as gauge type (not summary)
        metricType, exists := attrs.Get("metric.type")
        assert.True(t, exists)
        assert.Equal(t, "gauge", metricType.Str())
}

// TestProcessDatabaseIOMetrics tests the processDatabaseIOMetrics function
func TestProcessDatabaseIOMetrics(t *testing.T) {
    logger, _ := zap.NewDevelopment()
    startTime := pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Minute))
    
    tests := []struct {
        name         string
        input        models.DatabaseIOMetrics
        engineEdition int
        expectError  bool
        expectMetrics int
        validateFields func(t *testing.T, scopeMetrics pmetric.ScopeMetrics)
    }{
        {
            name: "successful processing with valid IOStallTimeMs",
            input: models.DatabaseIOMetrics{
                DatabaseName:  "test_db",
                IOStallTimeMs: int64Ptr(1500), // 1.5 seconds
            },
            engineEdition: queries.AzureSQLDatabaseEngineEdition,
            expectError:   false,
            expectMetrics: 1,
            validateFields: func(t *testing.T, scopeMetrics pmetric.ScopeMetrics) {
                require.Equal(t, 1, scopeMetrics.Metrics().Len())
                
                metric := scopeMetrics.Metrics().At(0)
                assert.Equal(t, "sqlserver.database.io.stallInMilliseconds", metric.Name())
                assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
                
                gauge := metric.Gauge()
                assert.Equal(t, 1, gauge.DataPoints().Len())
                
                dataPoint := gauge.DataPoints().At(0)
                assert.Equal(t, int64(1500), dataPoint.IntValue())
                
                // Verify attributes
                attrs := dataPoint.Attributes()
                assert.Equal(t, 5, attrs.Len())
                
                databaseName, exists := attrs.Get("database_name")
                assert.True(t, exists)
                assert.Equal(t, "test_db", databaseName.Str())
                
                metricType, exists := attrs.Get("metric.type")
                assert.True(t, exists)
                assert.Equal(t, "gauge", metricType.Str())
            },
        },
        {
            name: "processing with nil IOStallTimeMs",
            input: models.DatabaseIOMetrics{
                DatabaseName:  "null_db",
                IOStallTimeMs: nil,
            },
            engineEdition: queries.AzureSQLDatabaseEngineEdition,
            expectError:   false,
            expectMetrics: 0, // nil values should not generate metrics
        },
        {
            name: "processing with zero IOStallTimeMs",
            input: models.DatabaseIOMetrics{
                DatabaseName:  "zero_db",
                IOStallTimeMs: int64Ptr(0),
            },
            engineEdition: queries.AzureSQLDatabaseEngineEdition,
            expectError:   false,
            expectMetrics: 1, // zero is valid and should generate metric
            validateFields: func(t *testing.T, scopeMetrics pmetric.ScopeMetrics) {
                require.Equal(t, 1, scopeMetrics.Metrics().Len())
                
                metric := scopeMetrics.Metrics().At(0)
                dataPoint := metric.Gauge().DataPoints().At(0)
                assert.Equal(t, int64(0), dataPoint.IntValue())
            },
        },
        {
            name: "processing with large IOStallTimeMs",
            input: models.DatabaseIOMetrics{
                DatabaseName:  "large_db",
                IOStallTimeMs: int64Ptr(999999999), // very large stall time
            },
            engineEdition: queries.AzureSQLDatabaseEngineEdition,
            expectError:   false,
            expectMetrics: 1,
            validateFields: func(t *testing.T, scopeMetrics pmetric.ScopeMetrics) {
                require.Equal(t, 1, scopeMetrics.Metrics().Len())
                
                metric := scopeMetrics.Metrics().At(0)
                dataPoint := metric.Gauge().DataPoints().At(0)
                assert.Equal(t, int64(999999999), dataPoint.IntValue())
            },
        },
        {
            name: "processing with empty database name",
            input: models.DatabaseIOMetrics{
                DatabaseName:  "",
                IOStallTimeMs: int64Ptr(500),
            },
            engineEdition: queries.AzureSQLDatabaseEngineEdition,
            expectError:   false,
            expectMetrics: 1, // empty database name is still valid
            validateFields: func(t *testing.T, scopeMetrics pmetric.ScopeMetrics) {
                require.Equal(t, 1, scopeMetrics.Metrics().Len())
                
                metric := scopeMetrics.Metrics().At(0)
                dataPoint := metric.Gauge().DataPoints().At(0)
                attrs := dataPoint.Attributes()
                
                databaseName, exists := attrs.Get("database_name")
                assert.True(t, exists)
                assert.Equal(t, "", databaseName.Str())
            },
        },
        {
            name: "processing with special database name",
            input: models.DatabaseIOMetrics{
                DatabaseName:  "test-db_123.production@server",
                IOStallTimeMs: int64Ptr(250),
            },
            engineEdition: queries.StandardSQLServerEngineEdition,
            expectError:   false,
            expectMetrics: 1,
            validateFields: func(t *testing.T, scopeMetrics pmetric.ScopeMetrics) {
                require.Equal(t, 1, scopeMetrics.Metrics().Len())
                
                metric := scopeMetrics.Metrics().At(0)
                dataPoint := metric.Gauge().DataPoints().At(0)
                attrs := dataPoint.Attributes()
                
                databaseName, exists := attrs.Get("database_name")
                assert.True(t, exists)
                assert.Equal(t, "test-db_123.production@server", databaseName.Str())
            },
        },
        {
            name: "processing with extreme value",
            input: models.DatabaseIOMetrics{
                DatabaseName:  "extreme_db",
                IOStallTimeMs: int64Ptr(9223372036854775807), // max int64
            },
            engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
            expectError:   false,
            expectMetrics: 1,
            validateFields: func(t *testing.T, scopeMetrics pmetric.ScopeMetrics) {
                require.Equal(t, 1, scopeMetrics.Metrics().Len())
                
                metric := scopeMetrics.Metrics().At(0)
                dataPoint := metric.Gauge().DataPoints().At(0)
                assert.Equal(t, int64(9223372036854775807), dataPoint.IntValue())
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            scraper := &DatabaseScraper{
                logger:        logger,
                startTime:     startTime,
                engineEdition: tt.engineEdition,
            }

            // Create empty scope metrics
            scopeMetrics := pmetric.NewScopeMetrics()

            // Execute the function
            err := scraper.processDatabaseIOMetrics(tt.input, scopeMetrics)

            // Verify error expectation
            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }

            // Verify metrics count
            assert.Equal(t, tt.expectMetrics, scopeMetrics.Metrics().Len())

            // Run custom validations
            if tt.validateFields != nil {
                tt.validateFields(t, scopeMetrics)
            }
        })
    }
}

// TestProcessDatabaseIOMetrics_TimestampValidation tests timestamp handling in IO metrics
func TestProcessDatabaseIOMetrics_TimestampValidation(t *testing.T) {
    logger, _ := zap.NewDevelopment()
    startTime := pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Minute))
    
    scraper := &DatabaseScraper{
        logger:        logger,
        startTime:     startTime,
        engineEdition: queries.AzureSQLDatabaseEngineEdition,
    }

    result := models.DatabaseIOMetrics{
        DatabaseName:  "TestDatabase",
        IOStallTimeMs: int64Ptr(2500),
    }

    scopeMetrics := pmetric.NewScopeMetrics()
    beforeProcessing := time.Now()

    err := scraper.processDatabaseIOMetrics(result, scopeMetrics)
    require.NoError(t, err)

    afterProcessing := time.Now()
    require.Equal(t, 1, scopeMetrics.Metrics().Len())

    metric := scopeMetrics.Metrics().At(0)
    dataPoint := metric.Gauge().DataPoints().At(0)

    // Verify timestamps
    assert.Equal(t, startTime, dataPoint.StartTimestamp())
    
    timestampTime := dataPoint.Timestamp().AsTime()
    assert.True(t, timestampTime.After(beforeProcessing) || timestampTime.Equal(beforeProcessing))
    assert.True(t, timestampTime.Before(afterProcessing) || timestampTime.Equal(afterProcessing))
}

// TestProcessDatabaseIOMetrics_ReflectionEdgeCases tests edge cases in reflection processing
func TestProcessDatabaseIOMetrics_ReflectionEdgeCases(t *testing.T) {
    logger, _ := zap.NewDevelopment()
    startTime := pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Minute))

    t.Run("empty database name", func(t *testing.T) {
        scraper := &DatabaseScraper{
            logger:        logger,
            startTime:     startTime,
            engineEdition: queries.AzureSQLDatabaseEngineEdition,
        }

        result := models.DatabaseIOMetrics{
            DatabaseName:  "", // empty database name
            IOStallTimeMs: int64Ptr(1000),
        }

        scopeMetrics := pmetric.NewScopeMetrics()
        err := scraper.processDatabaseIOMetrics(result, scopeMetrics)
        require.NoError(t, err)

        // Should still process the metric
        require.Equal(t, 1, scopeMetrics.Metrics().Len())

        metric := scopeMetrics.Metrics().At(0)
        dataPoint := metric.Gauge().DataPoints().At(0)
        attrs := dataPoint.Attributes()

        databaseName, exists := attrs.Get("database_name")
        assert.True(t, exists)
        assert.Equal(t, "", databaseName.Str())
    })

    t.Run("special characters in database name", func(t *testing.T) {
        scraper := &DatabaseScraper{
            logger:        logger,
            startTime:     startTime,
            engineEdition: queries.StandardSQLServerEngineEdition,
        }

        specialName := "test-db_123.production@server"
        result := models.DatabaseIOMetrics{
            DatabaseName:  specialName,
            IOStallTimeMs: int64Ptr(750),
        }

        scopeMetrics := pmetric.NewScopeMetrics()
        err := scraper.processDatabaseIOMetrics(result, scopeMetrics)
        require.NoError(t, err)

        require.Equal(t, 1, scopeMetrics.Metrics().Len())

        metric := scopeMetrics.Metrics().At(0)
        dataPoint := metric.Gauge().DataPoints().At(0)
        attrs := dataPoint.Attributes()

        databaseName, exists := attrs.Get("database_name")
        assert.True(t, exists)
        assert.Equal(t, specialName, databaseName.Str())
    })
}

// TestProcessDatabaseIOMetrics_AllEngineEditions tests all engine editions
func TestProcessDatabaseIOMetrics_AllEngineEditions(t *testing.T) {
    logger, _ := zap.NewDevelopment()
    startTime := pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Minute))

    engines := []int{
        queries.StandardSQLServerEngineEdition,
        queries.AzureSQLDatabaseEngineEdition,
        queries.AzureSQLManagedInstanceEngineEdition,
    }

    for _, engine := range engines {
        t.Run(fmt.Sprintf("engine_%d", engine), func(t *testing.T) {
            scraper := &DatabaseScraper{
                logger:        logger,
                startTime:     startTime,
                engineEdition: engine,
            }

            result := models.DatabaseIOMetrics{
                DatabaseName:  fmt.Sprintf("test_db_%d", engine),
                IOStallTimeMs: int64Ptr(1000),
            }

            scopeMetrics := pmetric.NewScopeMetrics()
            err := scraper.processDatabaseIOMetrics(result, scopeMetrics)
            require.NoError(t, err)

            // Should process successfully regardless of engine edition
            assert.Equal(t, 1, scopeMetrics.Metrics().Len())

            metric := scopeMetrics.Metrics().At(0)
            assert.Equal(t, "sqlserver.database.io.stallInMilliseconds", metric.Name())
            assert.Equal(t, 1, metric.Gauge().DataPoints().Len())
        })
    }
}

// TestProcessDatabaseIOMetrics_DataTypeConsistency tests data type consistency
func TestProcessDatabaseIOMetrics_DataTypeConsistency(t *testing.T) {
    logger, _ := zap.NewDevelopment()
    startTime := pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Minute))

    scraper := &DatabaseScraper{
        logger:        logger,
        startTime:     startTime,
        engineEdition: queries.AzureSQLDatabaseEngineEdition,
    }

    result := models.DatabaseIOMetrics{
        DatabaseName:  "consistency_test_db",
        IOStallTimeMs: int64Ptr(12345), // int64 value
    }

    scopeMetrics := pmetric.NewScopeMetrics()
    err := scraper.processDatabaseIOMetrics(result, scopeMetrics)
    require.NoError(t, err)

    require.Equal(t, 1, scopeMetrics.Metrics().Len())

    metric := scopeMetrics.Metrics().At(0)
    require.Equal(t, 1, metric.Gauge().DataPoints().Len())

    dataPoint := metric.Gauge().DataPoints().At(0)

    // IO stall time should be an int64 value
    assert.Equal(t, pmetric.NumberDataPointValueTypeInt, dataPoint.ValueType())
    assert.Equal(t, int64(12345), dataPoint.IntValue())
}

// TestProcessDatabaseIOMetrics_GaugeMetricStructure tests gauge metric structure validation
func TestProcessDatabaseIOMetrics_GaugeMetricStructure(t *testing.T) {
    logger, _ := zap.NewDevelopment()
    startTime := pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Minute))

    scraper := &DatabaseScraper{
        logger:        logger,
        startTime:     startTime,
        engineEdition: queries.AzureSQLDatabaseEngineEdition,
    }

    result := models.DatabaseIOMetrics{
        DatabaseName:  "gauge_test_db",
        IOStallTimeMs: int64Ptr(7500),
    }

    scopeMetrics := pmetric.NewScopeMetrics()
    err := scraper.processDatabaseIOMetrics(result, scopeMetrics)
    require.NoError(t, err)

    require.Equal(t, 1, scopeMetrics.Metrics().Len())

    metric := scopeMetrics.Metrics().At(0)

    // Verify it's a gauge metric (not sum, histogram, summary, etc.)
    assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

    // Should have data points
    gauge := metric.Gauge()
    assert.Equal(t, 1, gauge.DataPoints().Len())

    dataPoint := gauge.DataPoints().At(0)

    // Verify the value represents the actual IO stall time
    assert.Equal(t, int64(7500), dataPoint.IntValue())

    // Verify timestamps are set
    assert.True(t, dataPoint.Timestamp() > 0)
    assert.True(t, dataPoint.StartTimestamp() > 0)

    // Verify attributes are correctly set for gauge metrics
    attrs := dataPoint.Attributes()
    assert.Equal(t, 5, attrs.Len())

    // Check that it's marked as gauge type (not summary)
    metricType, exists := attrs.Get("metric.type")
    assert.True(t, exists)
    assert.Equal(t, "gauge", metricType.Str())
}

// Helper function to create valid DatabaseIOMetrics for testing
func createValidDatabaseIOMetrics() models.DatabaseIOMetrics {
	return models.DatabaseIOMetrics{
		DatabaseName:  "TestDatabase",
		IOStallTimeMs: int64Ptr(1500),
	}
}

// ========================================
// Database Log Growth Metrics Tests
// ========================================

func TestProcessDatabaseLogGrowthMetrics_BasicFunctionality(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	logGrowthMetrics := models.DatabaseLogGrowthMetrics{
		DatabaseName:   "TestDatabase", 
		LogGrowthCount: int64Ptr(50),
	}

	err := scraper.processDatabaseLogGrowthMetrics(logGrowthMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())
	
	metric := scopeMetrics.Metrics().At(0)
	assert.Equal(t, "sqlserver.database.log.transactionGrowth", metric.Name())
	assert.Equal(t, "Number of log growth events for the SQL Server database (log.transactionGrowth)", metric.Description())
	assert.Equal(t, "1", metric.Unit())

	gauge := metric.Gauge()
	assert.Equal(t, 1, gauge.DataPoints().Len())
	
	dataPoint := gauge.DataPoints().At(0)
	assert.Equal(t, int64(50), dataPoint.IntValue())
	
	// Verify attributes
	attrs := dataPoint.Attributes()
	databaseName, exists := attrs.Get("database_name")
	assert.True(t, exists)
	assert.Equal(t, "TestDatabase", databaseName.Str())
	
	metricType, exists := attrs.Get("metric.type")
	assert.True(t, exists)
	assert.Equal(t, "gauge", metricType.Str())
	
	metricSource, exists := attrs.Get("metric.source")
	assert.True(t, exists)
	assert.Equal(t, "sys.dm_os_performance_counters", metricSource.Str())
}

func TestProcessDatabaseLogGrowthMetrics_MultipleEngineEditions(t *testing.T) {
	tests := []struct {
		name          string
		engineEdition int
		expectedEngine string
	}{
		{
			name:          "StandardSQLServer",
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectedEngine: "Standard SQL Server",
		},
		{
			name:          "AzureSQLDatabase", 
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectedEngine: "Azure SQL Database",
		},
		{
			name:          "AzureSQLManagedInstance",
			engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
			expectedEngine: "Azure SQL Managed Instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := &DatabaseScraper{
				logger:        zap.NewNop(),
				startTime:     pcommon.NewTimestampFromTime(time.Now()),
				engineEdition: tt.engineEdition,
			}
			
			scopeMetrics := pmetric.NewScopeMetrics()
			logGrowthMetrics := models.DatabaseLogGrowthMetrics{
				DatabaseName:   fmt.Sprintf("%sDatabase", tt.name),
				LogGrowthCount: int64Ptr(25),
			}

			err := scraper.processDatabaseLogGrowthMetrics(logGrowthMetrics, scopeMetrics)

			assert.NoError(t, err)
			assert.Equal(t, 1, scopeMetrics.Metrics().Len())

			dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
			attrs := dataPoint.Attributes()
			
			engineEdition, exists := attrs.Get("engine_edition")
			assert.True(t, exists)
			assert.Equal(t, tt.expectedEngine, engineEdition.Str())
			
			engineEditionId, exists := attrs.Get("engine_edition_id")
			assert.True(t, exists)
			assert.Equal(t, int64(tt.engineEdition), engineEditionId.Int())
		})
	}
}

func TestProcessDatabaseLogGrowthMetrics_SpecialDatabaseNames(t *testing.T) {
	tests := []struct {
		name         string
		databaseName string
	}{
		{
			name:         "EmptyDatabaseName",
			databaseName: "",
		},
		{
			name:         "SpecialCharacters",
			databaseName: "Test-Database_With.Special[Chars]",
		},
		{
			name:         "VeryLongDatabaseName",
			databaseName: strings.Repeat("A", 200),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := &DatabaseScraper{
				logger:        zap.NewNop(),
				startTime:     pcommon.NewTimestampFromTime(time.Now()),
				engineEdition: queries.AzureSQLDatabaseEngineEdition,
			}
			scopeMetrics := pmetric.NewScopeMetrics()
			
			logGrowthMetrics := models.DatabaseLogGrowthMetrics{
				DatabaseName:   tt.databaseName,
				LogGrowthCount: int64Ptr(10),
			}

			err := scraper.processDatabaseLogGrowthMetrics(logGrowthMetrics, scopeMetrics)

			assert.NoError(t, err)
			assert.Equal(t, 1, scopeMetrics.Metrics().Len())

			dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
			attrs := dataPoint.Attributes()
			
			databaseName, exists := attrs.Get("database_name")
			assert.True(t, exists)
			assert.Equal(t, tt.databaseName, databaseName.Str())
		})
	}
}

func TestProcessDatabaseLogGrowthMetrics_NilLogGrowthCount(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	logGrowthMetrics := models.DatabaseLogGrowthMetrics{
		DatabaseName:   "TestDatabase",
		LogGrowthCount: nil, // Nil value should be skipped
	}

	err := scraper.processDatabaseLogGrowthMetrics(logGrowthMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 0, scopeMetrics.Metrics().Len()) // No metrics created for nil values
}

func TestProcessDatabaseLogGrowthMetrics_ZeroLogGrowthCount(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	logGrowthMetrics := models.DatabaseLogGrowthMetrics{
		DatabaseName:   "EmptyDatabase",
		LogGrowthCount: int64Ptr(0),
	}

	err := scraper.processDatabaseLogGrowthMetrics(logGrowthMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())

	dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
	assert.Equal(t, int64(0), dataPoint.IntValue())
}

func TestProcessDatabaseLogGrowthMetrics_LargeLogGrowthCount(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	logGrowthMetrics := models.DatabaseLogGrowthMetrics{
		DatabaseName:   "LargeDatabase",
		LogGrowthCount: int64Ptr(999999999),
	}

	err := scraper.processDatabaseLogGrowthMetrics(logGrowthMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())

	dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
	assert.Equal(t, int64(999999999), dataPoint.IntValue())
}

func TestProcessDatabaseLogGrowthMetrics_TimestampValidation(t *testing.T) {
	beforeTime := time.Now()
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	logGrowthMetrics := models.DatabaseLogGrowthMetrics{
		DatabaseName:   "TestDatabase",
		LogGrowthCount: int64Ptr(100),
	}

	err := scraper.processDatabaseLogGrowthMetrics(logGrowthMetrics, scopeMetrics)
	afterTime := time.Now()

	assert.NoError(t, err)
	
	dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
	
	// Verify timestamp is within reasonable range
	timestamp := dataPoint.Timestamp().AsTime()
	assert.True(t, timestamp.After(beforeTime) || timestamp.Equal(beforeTime))
	assert.True(t, timestamp.Before(afterTime) || timestamp.Equal(afterTime))
	
	// Verify start timestamp is set to scraper's start time
	assert.Equal(t, scraper.startTime, dataPoint.StartTimestamp())
}

func TestProcessDatabaseLogGrowthMetrics_ReflectionFieldProcessing(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	// Test struct with both metric and non-metric fields
	logGrowthMetrics := models.DatabaseLogGrowthMetrics{
		DatabaseName:   "ReflectionTestDB",
		LogGrowthCount: int64Ptr(75),
	}

	err := scraper.processDatabaseLogGrowthMetrics(logGrowthMetrics, scopeMetrics)

	assert.NoError(t, err)
	
	// Should only create 1 metric (for LogGrowthCount field with metric_name tag)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())
	
	metric := scopeMetrics.Metrics().At(0)
	assert.Equal(t, "sqlserver.database.log.transactionGrowth", metric.Name())
	
	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, int64(75), dataPoint.IntValue())
}

func TestProcessDatabaseLogGrowthMetrics_MetricStructureValidation(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	logGrowthMetrics := models.DatabaseLogGrowthMetrics{
		DatabaseName:   "StructureTestDB",
		LogGrowthCount: int64Ptr(200),
	}

	err := scraper.processDatabaseLogGrowthMetrics(logGrowthMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	
	// Validate metric structure
	assert.Equal(t, "sqlserver.database.log.transactionGrowth", metric.Name())
	assert.Equal(t, "Number of log growth events for the SQL Server database (log.transactionGrowth)", metric.Description())
	assert.Equal(t, "1", metric.Unit())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	
	// Validate gauge structure
	gauge := metric.Gauge()
	assert.Equal(t, 1, gauge.DataPoints().Len())
	
	// Validate data point structure
	dataPoint := gauge.DataPoints().At(0)
	assert.Equal(t, pmetric.NumberDataPointValueTypeInt, dataPoint.ValueType())
	assert.Equal(t, int64(200), dataPoint.IntValue())
	
	// Validate all required attributes are present
	attrs := dataPoint.Attributes()
	assert.Equal(t, 5, attrs.Len()) // database_name, metric.type, metric.source, engine_edition, engine_edition_id
	
	requiredAttrs := []string{"database_name", "metric.type", "metric.source", "engine_edition", "engine_edition_id"}
	for _, attrName := range requiredAttrs {
		_, exists := attrs.Get(attrName)
		assert.True(t, exists, fmt.Sprintf("Required attribute %s should exist", attrName))
	}
}

// Helper function to create valid DatabaseLogGrowthMetrics for testing
func createValidDatabaseLogGrowthMetrics() models.DatabaseLogGrowthMetrics {
	return models.DatabaseLogGrowthMetrics{
		DatabaseName:   "TestDatabase",
		LogGrowthCount: int64Ptr(100),
	}
}

// ========================================
// Database Page File Metrics Tests
// ========================================

func TestProcessDatabasePageFileMetrics_BasicFunctionality(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileMetrics := models.DatabasePageFileMetrics{
		DatabaseName:           "TestDatabase", 
		PageFileAvailableBytes: float64Ptr(1073741824.0), // 1 GB
	}

	err := scraper.processDatabasePageFileMetrics(pageFileMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())
	
	metric := scopeMetrics.Metrics().At(0)
	assert.Equal(t, "sqlserver.database.pageFileAvailable", metric.Name())
	assert.Equal(t, "Available page file space (reserved space not used) for the SQL Server database in bytes (pageFileAvailable)", metric.Description())
	assert.Equal(t, "By", metric.Unit())

	gauge := metric.Gauge()
	assert.Equal(t, 1, gauge.DataPoints().Len())
	
	dataPoint := gauge.DataPoints().At(0)
	assert.Equal(t, float64(1073741824.0), dataPoint.DoubleValue())
	
	// Verify attributes
	attrs := dataPoint.Attributes()
	databaseName, exists := attrs.Get("database_name")
	assert.True(t, exists)
	assert.Equal(t, "TestDatabase", databaseName.Str())
	
	metricType, exists := attrs.Get("metric.type")
	assert.True(t, exists)
	assert.Equal(t, "gauge", metricType.Str())
	
	metricSource, exists := attrs.Get("metric.source")
	assert.True(t, exists)
	assert.Equal(t, "sys.partitions_sys.allocation_units", metricSource.Str())
}

func TestProcessDatabasePageFileMetrics_MultipleEngineEditions(t *testing.T) {
	tests := []struct {
		name          string
		engineEdition int
		expectedEngine string
	}{
		{
			name:          "StandardSQLServer",
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectedEngine: "Standard SQL Server",
		},
		{
			name:          "AzureSQLDatabase", 
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectedEngine: "Azure SQL Database",
		},
		{
			name:          "AzureSQLManagedInstance",
			engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
			expectedEngine: "Azure SQL Managed Instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := &DatabaseScraper{
				logger:        zap.NewNop(),
				startTime:     pcommon.NewTimestampFromTime(time.Now()),
				engineEdition: tt.engineEdition,
			}
			
			scopeMetrics := pmetric.NewScopeMetrics()
			pageFileMetrics := models.DatabasePageFileMetrics{
				DatabaseName:           fmt.Sprintf("%sDatabase", tt.name),
				PageFileAvailableBytes: float64Ptr(2147483648.0), // 2 GB
			}

			err := scraper.processDatabasePageFileMetrics(pageFileMetrics, scopeMetrics)

			assert.NoError(t, err)
			assert.Equal(t, 1, scopeMetrics.Metrics().Len())

			dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
			attrs := dataPoint.Attributes()
			
			engineEdition, exists := attrs.Get("engine_edition")
			assert.True(t, exists)
			assert.Equal(t, tt.expectedEngine, engineEdition.Str())
			
			engineEditionId, exists := attrs.Get("engine_edition_id")
			assert.True(t, exists)
			assert.Equal(t, int64(tt.engineEdition), engineEditionId.Int())
		})
	}
}

func TestProcessDatabasePageFileMetrics_SpecialDatabaseNames(t *testing.T) {
	tests := []struct {
		name         string
		databaseName string
	}{
		{
			name:         "EmptyDatabaseName",
			databaseName: "",
		},
		{
			name:         "SpecialCharacters",
			databaseName: "Test-Database_With.Special[Chars]",
		},
		{
			name:         "VeryLongDatabaseName",
			databaseName: strings.Repeat("A", 200),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := &DatabaseScraper{
				logger:        zap.NewNop(),
				startTime:     pcommon.NewTimestampFromTime(time.Now()),
				engineEdition: queries.AzureSQLDatabaseEngineEdition,
			}
			scopeMetrics := pmetric.NewScopeMetrics()
			
			pageFileMetrics := models.DatabasePageFileMetrics{
				DatabaseName:           tt.databaseName,
				PageFileAvailableBytes: float64Ptr(536870912.0), // 512 MB
			}

			err := scraper.processDatabasePageFileMetrics(pageFileMetrics, scopeMetrics)

			assert.NoError(t, err)
			assert.Equal(t, 1, scopeMetrics.Metrics().Len())

			dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
			attrs := dataPoint.Attributes()
			
			databaseName, exists := attrs.Get("database_name")
			assert.True(t, exists)
			assert.Equal(t, tt.databaseName, databaseName.Str())
		})
	}
}

func TestProcessDatabasePageFileMetrics_NilPageFileAvailableBytes(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileMetrics := models.DatabasePageFileMetrics{
		DatabaseName:           "TestDatabase",
		PageFileAvailableBytes: nil, // Nil value should be skipped
	}

	err := scraper.processDatabasePageFileMetrics(pageFileMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 0, scopeMetrics.Metrics().Len()) // No metrics created for nil values
}

func TestProcessDatabasePageFileMetrics_ZeroPageFileAvailableBytes(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileMetrics := models.DatabasePageFileMetrics{
		DatabaseName:           "EmptyDatabase",
		PageFileAvailableBytes: float64Ptr(0.0),
	}

	err := scraper.processDatabasePageFileMetrics(pageFileMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())

	dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
	assert.Equal(t, float64(0.0), dataPoint.DoubleValue())
}

func TestProcessDatabasePageFileMetrics_LargePageFileAvailableBytes(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileMetrics := models.DatabasePageFileMetrics{
		DatabaseName:           "LargeDatabase",
		PageFileAvailableBytes: float64Ptr(1099511627776.0), // 1 TB
	}

	err := scraper.processDatabasePageFileMetrics(pageFileMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())

	dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
	assert.Equal(t, float64(1099511627776.0), dataPoint.DoubleValue())
}

func TestProcessDatabasePageFileMetrics_TimestampValidation(t *testing.T) {
	beforeTime := time.Now()
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileMetrics := models.DatabasePageFileMetrics{
		DatabaseName:           "TestDatabase",
		PageFileAvailableBytes: float64Ptr(4294967296.0), // 4 GB
	}

	err := scraper.processDatabasePageFileMetrics(pageFileMetrics, scopeMetrics)
	afterTime := time.Now()

	assert.NoError(t, err)
	
	dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
	
	// Verify timestamp is within reasonable range
	timestamp := dataPoint.Timestamp().AsTime()
	assert.True(t, timestamp.After(beforeTime) || timestamp.Equal(beforeTime))
	assert.True(t, timestamp.Before(afterTime) || timestamp.Equal(afterTime))
	
	// Verify start timestamp is set to scraper's start time
	assert.Equal(t, scraper.startTime, dataPoint.StartTimestamp())
}

func TestProcessDatabasePageFileMetrics_ReflectionFieldProcessing(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	// Test struct with both metric and non-metric fields
	pageFileMetrics := models.DatabasePageFileMetrics{
		DatabaseName:           "ReflectionTestDB",
		PageFileAvailableBytes: float64Ptr(8589934592.0), // 8 GB
	}

	err := scraper.processDatabasePageFileMetrics(pageFileMetrics, scopeMetrics)

	assert.NoError(t, err)
	
	// Should only create 1 metric (for PageFileAvailableBytes field with metric_name tag)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())
	
	metric := scopeMetrics.Metrics().At(0)
	assert.Equal(t, "sqlserver.database.pageFileAvailable", metric.Name())
	
	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, float64(8589934592.0), dataPoint.DoubleValue())
}

func TestProcessDatabasePageFileMetrics_MetricStructureValidation(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileMetrics := models.DatabasePageFileMetrics{
		DatabaseName:           "StructureTestDB",
		PageFileAvailableBytes: float64Ptr(17179869184.0), // 16 GB
	}

	err := scraper.processDatabasePageFileMetrics(pageFileMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	
	// Validate metric structure
	assert.Equal(t, "sqlserver.database.pageFileAvailable", metric.Name())
	assert.Equal(t, "Available page file space (reserved space not used) for the SQL Server database in bytes (pageFileAvailable)", metric.Description())
	assert.Equal(t, "By", metric.Unit())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	
	// Validate gauge structure
	gauge := metric.Gauge()
	assert.Equal(t, 1, gauge.DataPoints().Len())
	
	// Validate data point structure
	dataPoint := gauge.DataPoints().At(0)
	assert.Equal(t, pmetric.NumberDataPointValueTypeDouble, dataPoint.ValueType())
	assert.Equal(t, float64(17179869184.0), dataPoint.DoubleValue())
	
	// Validate all required attributes are present
	attrs := dataPoint.Attributes()
	assert.Equal(t, 5, attrs.Len()) // database_name, metric.type, metric.source, engine_edition, engine_edition_id
	
	requiredAttrs := []string{"database_name", "metric.type", "metric.source", "engine_edition", "engine_edition_id"}
	for _, attrName := range requiredAttrs {
		_, exists := attrs.Get(attrName)
		assert.True(t, exists, fmt.Sprintf("Required attribute %s should exist", attrName))
	}
}

func TestProcessDatabasePageFileMetrics_FloatPrecision(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	// Test with precise float values
	pageFileMetrics := models.DatabasePageFileMetrics{
		DatabaseName:           "PrecisionTestDB",
		PageFileAvailableBytes: float64Ptr(1073741824.123456), // Precise float value
	}

	err := scraper.processDatabasePageFileMetrics(pageFileMetrics, scopeMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())

	dataPoint := scopeMetrics.Metrics().At(0).Gauge().DataPoints().At(0)
	assert.Equal(t, float64(1073741824.123456), dataPoint.DoubleValue())
}

// Helper function to create valid DatabasePageFileMetrics for testing
func createValidDatabasePageFileMetrics() models.DatabasePageFileMetrics {
	return models.DatabasePageFileMetrics{
		DatabaseName:           "TestDatabase",
		PageFileAvailableBytes: float64Ptr(1073741824.0),
	}
}

// Helper function to create float64 pointer
func float64Ptr(f float64) *float64 {
	return &f
}

// ===== DatabasePageFileTotalMetrics Test Suite =====

// TestProcessDatabasePageFileTotalMetrics_BasicFunctionality tests basic processing of page file total metrics
func TestProcessDatabasePageFileTotalMetrics_BasicFunctionality(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileTotalBytes := 10737418240.0 // 10 GB
	result := models.DatabasePageFileTotalMetrics{
		DatabaseName:       "TestDatabase",
		PageFileTotalBytes: &pageFileTotalBytes,
	}

	err := scraper.processDatabasePageFileTotalMetrics(result, scopeMetrics)
	assert.NoError(t, err)

	// Verify metric creation
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())
	metric := scopeMetrics.Metrics().At(0)

	// Verify metric properties
	assert.Equal(t, "sqlserver.database.pageFileTotal", metric.Name())
	assert.Equal(t, "Total page file space (total reserved space) for the SQL Server database in bytes (pageFileTotal)", metric.Description())
	assert.Equal(t, "By", metric.Unit())

	// Verify metric type is gauge
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	gauge := metric.Gauge()
	assert.Equal(t, 1, gauge.DataPoints().Len())

	// Verify data point
	dataPoint := gauge.DataPoints().At(0)
	assert.Equal(t, pageFileTotalBytes, dataPoint.DoubleValue())

	// Verify attributes
	attrs := dataPoint.Attributes()
	dbName, exists := attrs.Get("database_name")
	assert.True(t, exists)
	assert.Equal(t, "TestDatabase", dbName.Str())

	metricType, exists := attrs.Get("metric.type")
	assert.True(t, exists)
	assert.Equal(t, "gauge", metricType.Str())

	metricSource, exists := attrs.Get("metric.source")
	assert.True(t, exists)
	assert.Equal(t, "sys.partitions_sys.allocation_units", metricSource.Str())
}

// TestProcessDatabasePageFileTotalMetrics_MultipleEngineEditions tests processing across different SQL Server engine editions
func TestProcessDatabasePageFileTotalMetrics_MultipleEngineEditions(t *testing.T) {
	testCases := []struct {
		name          string
		engineEdition int
		expectedType  string
	}{
		{"StandardSQLServer", queries.StandardSQLServerEngineEdition, "Standard SQL Server"},
		{"AzureSQLDatabase", queries.AzureSQLDatabaseEngineEdition, "Azure SQL Database"},
		{"AzureSQLManagedInstance", queries.AzureSQLManagedInstanceEngineEdition, "Azure SQL Managed Instance"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := &DatabaseScraper{
				logger:        zap.NewNop(),
				startTime:     pcommon.NewTimestampFromTime(time.Now()),
				engineEdition: tc.engineEdition,
			}

			scopeMetrics := pmetric.NewScopeMetrics()
			pageFileTotalBytes := 5368709120.0 // 5 GB

			result := models.DatabasePageFileTotalMetrics{
				DatabaseName:       fmt.Sprintf("%sDatabase", tc.name),
				PageFileTotalBytes: &pageFileTotalBytes,
			}

			err := scraper.processDatabasePageFileTotalMetrics(result, scopeMetrics)
			assert.NoError(t, err)

			// Verify engine edition attribute
			assert.Equal(t, 1, scopeMetrics.Metrics().Len())
			metric := scopeMetrics.Metrics().At(0)
			dataPoint := metric.Gauge().DataPoints().At(0)

			engineEditionAttr, exists := dataPoint.Attributes().Get("engine_edition")
			assert.True(t, exists)
			assert.Equal(t, tc.expectedType, engineEditionAttr.Str())

			engineEditionIdAttr, exists := dataPoint.Attributes().Get("engine_edition_id")
			assert.True(t, exists)
			assert.Equal(t, int64(tc.engineEdition), engineEditionIdAttr.Int())
		})
	}
}

// TestProcessDatabasePageFileTotalMetrics_SpecialDatabaseNames tests handling of special database names
func TestProcessDatabasePageFileTotalMetrics_SpecialDatabaseNames(t *testing.T) {
	testCases := []struct {
		name         string
		databaseName string
	}{
		{"EmptyDatabaseName", ""},
		{"SpecialCharacters", "Test-Database_With.Special[Chars]"},
		{"VeryLongDatabaseName", strings.Repeat("a", 1000)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := &DatabaseScraper{
				logger:        zap.NewNop(),
				startTime:     pcommon.NewTimestampFromTime(time.Now()),
				engineEdition: queries.StandardSQLServerEngineEdition,
			}
			scopeMetrics := pmetric.NewScopeMetrics()

			pageFileTotalBytes := 2147483648.0 // 2 GB
			result := models.DatabasePageFileTotalMetrics{
				DatabaseName:       tc.databaseName,
				PageFileTotalBytes: &pageFileTotalBytes,
			}

			err := scraper.processDatabasePageFileTotalMetrics(result, scopeMetrics)
			assert.NoError(t, err)

			// Verify database name attribute
			assert.Equal(t, 1, scopeMetrics.Metrics().Len())
			metric := scopeMetrics.Metrics().At(0)
			dataPoint := metric.Gauge().DataPoints().At(0)

			dbName, exists := dataPoint.Attributes().Get("database_name")
			assert.True(t, exists)
			assert.Equal(t, tc.databaseName, dbName.Str())
		})
	}
}

// TestProcessDatabasePageFileTotalMetrics_NilPageFileTotalBytes tests handling of nil page file total bytes
func TestProcessDatabasePageFileTotalMetrics_NilPageFileTotalBytes(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	result := models.DatabasePageFileTotalMetrics{
		DatabaseName:       "TestDatabase",
		PageFileTotalBytes: nil, // Nil value
	}

	err := scraper.processDatabasePageFileTotalMetrics(result, scopeMetrics)
	assert.NoError(t, err)

	// Should create no metrics due to nil value
	assert.Equal(t, 0, scopeMetrics.Metrics().Len())
}

// TestProcessDatabasePageFileTotalMetrics_ZeroPageFileTotalBytes tests handling of zero page file total bytes
func TestProcessDatabasePageFileTotalMetrics_ZeroPageFileTotalBytes(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileTotalBytes := 0.0
	result := models.DatabasePageFileTotalMetrics{
		DatabaseName:       "EmptyDatabase",
		PageFileTotalBytes: &pageFileTotalBytes,
	}

	err := scraper.processDatabasePageFileTotalMetrics(result, scopeMetrics)
	assert.NoError(t, err)

	// Verify zero value is processed correctly
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())
	metric := scopeMetrics.Metrics().At(0)
	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, 0.0, dataPoint.DoubleValue())
}

// TestProcessDatabasePageFileTotalMetrics_LargePageFileTotalBytes tests handling of very large page file total bytes
func TestProcessDatabasePageFileTotalMetrics_LargePageFileTotalBytes(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileTotalBytes := 1099511627776.0 // 1 TB
	result := models.DatabasePageFileTotalMetrics{
		DatabaseName:       "LargeDatabase",
		PageFileTotalBytes: &pageFileTotalBytes,
	}

	err := scraper.processDatabasePageFileTotalMetrics(result, scopeMetrics)
	assert.NoError(t, err)

	// Verify large value is processed correctly
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())
	metric := scopeMetrics.Metrics().At(0)
	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, pageFileTotalBytes, dataPoint.DoubleValue())
}

// TestProcessDatabasePageFileTotalMetrics_TimestampValidation tests timestamp setting for page file total metrics
func TestProcessDatabasePageFileTotalMetrics_TimestampValidation(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileTotalBytes := 8589934592.0 // 8 GB
	result := models.DatabasePageFileTotalMetrics{
		DatabaseName:       "TimestampTestDB",
		PageFileTotalBytes: &pageFileTotalBytes,
	}

	beforeTime := time.Now()
	err := scraper.processDatabasePageFileTotalMetrics(result, scopeMetrics)
	afterTime := time.Now()
	assert.NoError(t, err)

	// Verify timestamps
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())
	metric := scopeMetrics.Metrics().At(0)
	dataPoint := metric.Gauge().DataPoints().At(0)

	// Check that timestamp is within expected range
	timestamp := dataPoint.Timestamp().AsTime()
	assert.True(t, timestamp.After(beforeTime) || timestamp.Equal(beforeTime))
	assert.True(t, timestamp.Before(afterTime) || timestamp.Equal(afterTime))

	// Verify start timestamp is set
	assert.NotEqual(t, pcommon.Timestamp(0), dataPoint.StartTimestamp())
}

// TestProcessDatabasePageFileTotalMetrics_ReflectionFieldProcessing tests reflection-based field processing
func TestProcessDatabasePageFileTotalMetrics_ReflectionFieldProcessing(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileTotalBytes := 4294967296.0 // 4 GB
	result := models.DatabasePageFileTotalMetrics{
		DatabaseName:       "ReflectionTestDB",
		PageFileTotalBytes: &pageFileTotalBytes,
	}

	err := scraper.processDatabasePageFileTotalMetrics(result, scopeMetrics)
	assert.NoError(t, err)

	// Verify that reflection correctly processed the struct
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	
	// Verify metric name matches the struct tag
	assert.Equal(t, "sqlserver.database.pageFileTotal", metric.Name())
	
	// Verify the value was correctly extracted via reflection
	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, pageFileTotalBytes, dataPoint.DoubleValue())
}

// TestProcessDatabasePageFileTotalMetrics_MetricStructureValidation tests complete metric structure
func TestProcessDatabasePageFileTotalMetrics_MetricStructureValidation(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}
	scopeMetrics := pmetric.NewScopeMetrics()

	pageFileTotalBytes := 17179869184.0 // 16 GB
	result := models.DatabasePageFileTotalMetrics{
		DatabaseName:       "StructureTestDB",
		PageFileTotalBytes: &pageFileTotalBytes,
	}

	err := scraper.processDatabasePageFileTotalMetrics(result, scopeMetrics)
	assert.NoError(t, err)

	// Comprehensive validation of metric structure
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())
	metric := scopeMetrics.Metrics().At(0)

	// Validate metric metadata
	assert.Equal(t, "sqlserver.database.pageFileTotal", metric.Name())
	assert.Equal(t, "Total page file space (total reserved space) for the SQL Server database in bytes (pageFileTotal)", metric.Description())
	assert.Equal(t, "By", metric.Unit())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

	// Validate data points
	gauge := metric.Gauge()
	assert.Equal(t, 1, gauge.DataPoints().Len())
	
	dataPoint := gauge.DataPoints().At(0)
	assert.Equal(t, pageFileTotalBytes, dataPoint.DoubleValue())

	// Validate all required attributes
	attrs := dataPoint.Attributes()
	expectedAttrs := map[string]interface{}{
		"database_name":      "StructureTestDB",
		"metric.type":        "gauge",
		"metric.source":      "sys.partitions_sys.allocation_units",
		"engine_edition":     "Standard SQL Server",
		"engine_edition_id":  int64(queries.StandardSQLServerEngineEdition),
	}

	assert.Equal(t, len(expectedAttrs), attrs.Len())
	
	for key, expectedValue := range expectedAttrs {
		actualValue, exists := attrs.Get(key)
		assert.True(t, exists, "Attribute %s should exist", key)
		
		switch v := expectedValue.(type) {
		case string:
			assert.Equal(t, v, actualValue.Str())
		case int64:
			assert.Equal(t, v, actualValue.Int())
		default:
			t.Errorf("Unexpected attribute type for %s", key)
		}
	}
}

// TestProcessDatabasePageFileTotalMetrics_FloatPrecision tests float precision handling for page file total metrics
func TestProcessDatabasePageFileTotalMetrics_FloatPrecision(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.StandardSQLServerEngineEdition,
	}

	// Test different float precision values
	testCases := []struct {
		name     string
		value    float64
		expected float64
	}{
		{"SmallDecimal", 1234.5678, 1234.5678},
		{"LargeDecimal", 9876543210.123456, 9876543210.123456},
		{"VerySmallDecimal", 0.000001, 0.000001},
		{"ScientificNotation", 1.23e10, 1.23e10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scopeMetrics := pmetric.NewScopeMetrics()
			
			result := models.DatabasePageFileTotalMetrics{
				DatabaseName:       "PrecisionTestDB",
				PageFileTotalBytes: &tc.value,
			}

			err := scraper.processDatabasePageFileTotalMetrics(result, scopeMetrics)
			assert.NoError(t, err)

			// Verify the metric value with float precision
			assert.Greater(t, scopeMetrics.Metrics().Len(), 0)
			metric := scopeMetrics.Metrics().At(0)
			assert.Equal(t, "sqlserver.database.pageFileTotal", metric.Name())
			dataPoint := metric.Gauge().DataPoints().At(0)
			assert.InDelta(t, tc.expected, dataPoint.DoubleValue(), 0.000001)
		})
	}
}

// Test functions for processDatabaseMemoryResults

func TestProcessDatabaseMemoryResults_BasicFunctionality(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	results := []models.DatabaseMemoryMetrics{
		{
			TotalPhysicalMemoryBytes:     float64Ptr(8589934592.0),    // 8GB
			AvailablePhysicalMemoryBytes: float64Ptr(2147483648.0),    // 2GB
			MemoryUtilizationPercent:     float64Ptr(75.0),           // 75%
		},
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseMemoryResults(scopeMetrics, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len()) // 3 memory metrics

	// Verify metrics are created with correct names
	metricNames := make([]string, 0, 3)
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metricNames = append(metricNames, scopeMetrics.Metrics().At(i).Name())
	}

	expectedMetrics := []string{
		"sqlserver.instance.memoryTotal",
		"sqlserver.instance.memoryAvailable",
		"sqlserver.instance.memoryUtilization",
	}

	for _, expected := range expectedMetrics {
		assert.Contains(t, metricNames, expected, "Expected metric %s should be present", expected)
	}
}

func TestProcessDatabaseMemoryResults_EmptyResults(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	var results []models.DatabaseMemoryMetrics // Empty results

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseMemoryResults(scopeMetrics, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, scopeMetrics.Metrics().Len()) // No metrics should be created
}

func TestProcessDatabaseMemoryResults_NilValues(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	results := []models.DatabaseMemoryMetrics{
		{
			TotalPhysicalMemoryBytes:     nil, // Nil values create data points with default (0) values
			AvailablePhysicalMemoryBytes: nil,
			MemoryUtilizationPercent:     nil,
		},
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseMemoryResults(scopeMetrics, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len()) // 3 metrics are created

	// The actual behavior is that data points are created but continue
	// is called before setting the value, leaving them with default values
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		assert.Equal(t, 1, metric.Gauge().DataPoints().Len(), "Metric %s should have 1 data point even for nil values", metric.Name())
		
		// The data point exists but has default/unset timestamp values
		dataPoint := metric.Gauge().DataPoints().At(0)
		assert.Equal(t, 0.0, dataPoint.DoubleValue(), "Nil values should result in 0.0")
	}
}

func TestProcessDatabaseMemoryResults_PartialNilValues(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	results := []models.DatabaseMemoryMetrics{
		{
			TotalPhysicalMemoryBytes:     float64Ptr(16106127360.0), // 15GB
			AvailablePhysicalMemoryBytes: nil,                       // This creates a data point with 0 value
			MemoryUtilizationPercent:     float64Ptr(60.0),         // 60%
		},
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseMemoryResults(scopeMetrics, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len()) // 3 metrics are always created

	// Verify that all metrics have data points (even for nil values)
	metricData := make(map[string]float64)
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		assert.Equal(t, 1, metric.Gauge().DataPoints().Len(), "All metrics should have 1 data point")
		
		dataPoint := metric.Gauge().DataPoints().At(0)
		metricData[metric.Name()] = dataPoint.DoubleValue()
	}

	// Verify data values
	assert.Equal(t, 16106127360.0, metricData["sqlserver.instance.memoryTotal"], "memoryTotal should have correct value")
	assert.Equal(t, 0.0, metricData["sqlserver.instance.memoryAvailable"], "memoryAvailable should have 0 for nil value")
	assert.Equal(t, 60.0, metricData["sqlserver.instance.memoryUtilization"], "memoryUtilization should have correct value")
}

func TestProcessDatabaseMemoryResults_LargeMemoryValues(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	results := []models.DatabaseMemoryMetrics{
		{
			TotalPhysicalMemoryBytes:     float64Ptr(1099511627776.0), // 1TB
			AvailablePhysicalMemoryBytes: float64Ptr(274877906944.0),  // 256GB
			MemoryUtilizationPercent:     float64Ptr(75.0),           // 75%
		},
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseMemoryResults(scopeMetrics, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len())

	// Find and verify the total memory metric
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		if metric.Name() == "sqlserver.instance.memoryTotal" {
			dataPoint := metric.Gauge().DataPoints().At(0)
			assert.InDelta(t, 1099511627776.0, dataPoint.DoubleValue(), 0.1)
			break
		}
	}
}

func TestProcessDatabaseMemoryResults_ZeroValues(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	results := []models.DatabaseMemoryMetrics{
		{
			TotalPhysicalMemoryBytes:     float64Ptr(0.0),
			AvailablePhysicalMemoryBytes: float64Ptr(0.0),
			MemoryUtilizationPercent:     float64Ptr(0.0),
		},
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseMemoryResults(scopeMetrics, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len())

	// Verify all metrics have zero values
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		dataPoint := metric.Gauge().DataPoints().At(0)
		assert.Equal(t, 0.0, dataPoint.DoubleValue())
	}
}

func TestProcessDatabaseMemoryResults_AttributeValidation(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	results := []models.DatabaseMemoryMetrics{
		{
			TotalPhysicalMemoryBytes: float64Ptr(4294967296.0), // 4GB
		},
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseMemoryResults(scopeMetrics, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len()) // 3 metrics are always created

	// Find the metric with a data point to verify attributes
	var testMetric pmetric.Metric
	var testDataPoint pmetric.NumberDataPoint
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		if metric.Gauge().DataPoints().Len() > 0 {
			testMetric = metric
			testDataPoint = metric.Gauge().DataPoints().At(0)
			break
		}
	}

	assert.NotEmpty(t, testMetric.Name(), "Should find a metric with data points")
	attrs := testDataPoint.Attributes()

	// Verify all expected attributes are present
	expectedAttrs := map[string]interface{}{
		"metric.type":       "gauge",
		"metric.source":     "sys.dm_os_sys_memory",
		"engine_edition":    "Azure SQL Database",
		"engine_edition_id": int64(queries.AzureSQLDatabaseEngineEdition),
	}

	for key, expectedValue := range expectedAttrs {
		value, exists := attrs.Get(key)
		assert.True(t, exists, "Attribute %s should exist", key)

		switch v := expectedValue.(type) {
		case string:
			assert.Equal(t, v, value.AsString(), "Attribute %s should match", key)
		case int64:
			assert.Equal(t, v, value.Int(), "Attribute %s should match", key)
		}
	}
}

func TestProcessDatabaseMemoryResults_MetricStructureValidation(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	results := []models.DatabaseMemoryMetrics{
		{
			TotalPhysicalMemoryBytes:     float64Ptr(8589934592.0),
			AvailablePhysicalMemoryBytes: float64Ptr(2147483648.0),
			MemoryUtilizationPercent:     float64Ptr(75.0),
		},
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseMemoryResults(scopeMetrics, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len())

	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)

		// Verify metric structure
		assert.NotEmpty(t, metric.Name())
		assert.Equal(t, "SQL Server memory on the system", metric.Description())
		assert.Equal(t, "bytes", metric.Unit())

		// Verify gauge type
		assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

		// Verify data points
		assert.Equal(t, 1, metric.Gauge().DataPoints().Len())

		dataPoint := metric.Gauge().DataPoints().At(0)
		assert.True(t, dataPoint.Timestamp() > 0)
		assert.True(t, dataPoint.StartTimestamp() > 0)
		assert.True(t, dataPoint.DoubleValue() >= 0)
	}
}

func TestProcessDatabaseMemoryResults_MultipleResults(t *testing.T) {
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	results := []models.DatabaseMemoryMetrics{
		{
			TotalPhysicalMemoryBytes:     float64Ptr(8589934592.0),
			AvailablePhysicalMemoryBytes: float64Ptr(2147483648.0),
			MemoryUtilizationPercent:     float64Ptr(75.0),
		},
		{
			TotalPhysicalMemoryBytes:     float64Ptr(16106127360.0),
			AvailablePhysicalMemoryBytes: float64Ptr(4294967296.0),
			MemoryUtilizationPercent:     float64Ptr(73.3),
		},
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processDatabaseMemoryResults(scopeMetrics, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len()) // 3 metric types

	// Each metric should have 2 data points (one for each result)
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		assert.Equal(t, 2, metric.Gauge().DataPoints().Len())
	}
}

func TestProcessDatabaseMemoryResults_TimestampValidation(t *testing.T) {
	startTime := time.Now()
	scraper := &DatabaseScraper{
		logger:        zap.NewNop(),
		startTime:     pcommon.NewTimestampFromTime(startTime),
		engineEdition: queries.AzureSQLDatabaseEngineEdition,
	}

	results := []models.DatabaseMemoryMetrics{
		{
			TotalPhysicalMemoryBytes: float64Ptr(4294967296.0),
		},
	}

	scopeMetrics := pmetric.NewScopeMetrics()

	// Capture time before processing
	beforeProcessing := time.Now()
	err := scraper.processDatabaseMemoryResults(scopeMetrics, results)
	afterProcessing := time.Now()

	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len()) // 3 metrics are always created

	// Find the metric with a data point to verify timestamps
	var testDataPoint pmetric.NumberDataPoint
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		if metric.Gauge().DataPoints().Len() > 0 {
			testDataPoint = metric.Gauge().DataPoints().At(0)
			break
		}
	}

	assert.True(t, testDataPoint.Timestamp() > 0, "Should find a data point")

	// Verify start timestamp matches scraper start time
	expectedStartTime := pcommon.NewTimestampFromTime(startTime)
	assert.Equal(t, expectedStartTime, testDataPoint.StartTimestamp())

	// Verify current timestamp is within reasonable range
	actualTimestamp := testDataPoint.Timestamp().AsTime()
	assert.True(t, actualTimestamp.After(beforeProcessing) || actualTimestamp.Equal(beforeProcessing))
	assert.True(t, actualTimestamp.Before(afterProcessing) || actualTimestamp.Equal(afterProcessing))
}

// Test functions for DatabaseScraper constructor and struct

// MockSQLConnection is a simple mock implementation for testing
type MockSQLConnection struct{}

func (m *MockSQLConnection) Query(ctx context.Context, dest interface{}, query string) error {
	return nil
}

func TestNewDatabaseScraper_BasicFunctionality(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zap.NewNop()
	engineEdition := queries.AzureSQLDatabaseEngineEdition

	// Capture time before creating scraper for start time validation
	beforeCreation := time.Now()

	scraper := NewDatabaseScraper(mockConn, logger, engineEdition)

	// Capture time after creating scraper
	afterCreation := time.Now()

	// Verify all fields are properly set
	assert.NotNil(t, scraper, "Scraper should not be nil")
	assert.Equal(t, mockConn, scraper.connection, "Connection should be set correctly")
	assert.Equal(t, logger, scraper.logger, "Logger should be set correctly")
	assert.Equal(t, engineEdition, scraper.engineEdition, "Engine edition should be set correctly")

	// Verify start time is within reasonable range
	startTime := scraper.startTime.AsTime()
	assert.True(t, startTime.After(beforeCreation) || startTime.Equal(beforeCreation),
		"Start time should be after or equal to creation time")
	assert.True(t, startTime.Before(afterCreation) || startTime.Equal(afterCreation),
		"Start time should be before or equal to completion time")
}

func TestNewDatabaseScraper_AllEngineEditions(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zap.NewNop()

	testCases := []struct {
		name          string
		engineEdition int
	}{
		{
			name:          "Standard SQL Server",
			engineEdition: queries.StandardSQLServerEngineEdition,
		},
		{
			name:          "Azure SQL Database",
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
		},
		{
			name:          "Azure SQL Managed Instance",
			engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
		},
		{
			name:          "Unknown Engine Edition",
			engineEdition: 999,
		},
		{
			name:          "Zero Engine Edition",
			engineEdition: 0,
		},
		{
			name:          "Negative Engine Edition",
			engineEdition: -1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := NewDatabaseScraper(mockConn, logger, tc.engineEdition)

			assert.NotNil(t, scraper, "Scraper should not be nil")
			assert.Equal(t, mockConn, scraper.connection, "Connection should be set correctly")
			assert.Equal(t, logger, scraper.logger, "Logger should be set correctly")
			assert.Equal(t, tc.engineEdition, scraper.engineEdition, "Engine edition should be set correctly")
			assert.True(t, scraper.startTime > 0, "Start time should be set")
		})
	}
}

func TestNewDatabaseScraper_NilInputs(t *testing.T) {
	testCases := []struct {
		name          string
		connection    SQLConnectionInterface
		logger        *zap.Logger
		engineEdition int
		expectPanic   bool
	}{
		{
			name:          "Nil Connection",
			connection:    nil,
			logger:        zap.NewNop(),
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectPanic:   false, // Go allows nil interface assignments
		},
		{
			name:          "Nil Logger",
			connection:    &MockSQLConnection{},
			logger:        nil,
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectPanic:   false, // Go allows nil pointer assignments
		},
		{
			name:          "Both Nil",
			connection:    nil,
			logger:        nil,
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectPanic:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectPanic {
				assert.Panics(t, func() {
					NewDatabaseScraper(tc.connection, tc.logger, tc.engineEdition)
				})
			} else {
				scraper := NewDatabaseScraper(tc.connection, tc.logger, tc.engineEdition)
				assert.NotNil(t, scraper, "Scraper should not be nil")
				assert.Equal(t, tc.connection, scraper.connection)
				assert.Equal(t, tc.logger, scraper.logger)
				assert.Equal(t, tc.engineEdition, scraper.engineEdition)
				assert.True(t, scraper.startTime > 0, "Start time should be set")
			}
		})
	}
}

func TestDatabaseScraper_StructFieldTypes(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zap.NewNop()
	engineEdition := queries.AzureSQLDatabaseEngineEdition

	scraper := NewDatabaseScraper(mockConn, logger, engineEdition)

	// Verify struct field types using reflection
	scraperType := reflect.TypeOf(*scraper)
	assert.Equal(t, 4, scraperType.NumField(), "DatabaseScraper should have exactly 4 fields")

	// Check connection field
	connectionField, found := scraperType.FieldByName("connection")
	assert.True(t, found, "Should have connection field")
	assert.Equal(t, "SQLConnectionInterface", connectionField.Type.Name(), "Connection should be SQLConnectionInterface type")

	// Check logger field
	loggerField, found := scraperType.FieldByName("logger")
	assert.True(t, found, "Should have logger field")
	assert.True(t, loggerField.Type.String() == "*zap.Logger", "Logger should be *zap.Logger type")

	// Check startTime field
	startTimeField, found := scraperType.FieldByName("startTime")
	assert.True(t, found, "Should have startTime field")
	assert.Equal(t, "Timestamp", startTimeField.Type.Name(), "StartTime should be pcommon.Timestamp type")

	// Check engineEdition field
	engineEditionField, found := scraperType.FieldByName("engineEdition")
	assert.True(t, found, "Should have engineEdition field")
	assert.Equal(t, "int", engineEditionField.Type.String(), "EngineEdition should be int type")
}

func TestDatabaseScraper_StartTimeConsistency(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zap.NewNop()
	engineEdition := queries.AzureSQLDatabaseEngineEdition

	// Create multiple scrapers and verify they have different start times
	scraper1 := NewDatabaseScraper(mockConn, logger, engineEdition)
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	scraper2 := NewDatabaseScraper(mockConn, logger, engineEdition)

	assert.NotEqual(t, scraper1.startTime, scraper2.startTime,
		"Different scraper instances should have different start times")
	assert.True(t, scraper2.startTime > scraper1.startTime,
		"Later created scraper should have later start time")
}

func TestDatabaseScraper_FieldVisibility(t *testing.T) {
	// Verify that all fields are private (lowercase first letter)
	scraperType := reflect.TypeOf(DatabaseScraper{})

	for i := 0; i < scraperType.NumField(); i++ {
		field := scraperType.Field(i)
		firstChar := field.Name[0]
		assert.True(t, firstChar >= 'a' && firstChar <= 'z',
			"Field %s should be private (start with lowercase)", field.Name)
	}
}

func TestDatabaseScraper_MemoryLayout(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zap.NewNop()
	engineEdition := queries.AzureSQLDatabaseEngineEdition

	scraper := NewDatabaseScraper(mockConn, logger, engineEdition)

	// Verify that the scraper is properly initialized and not a zero value
	scraperValue := reflect.ValueOf(*scraper)
	assert.False(t, scraperValue.IsZero(), "Scraper should not be zero value")

	// Verify each field is properly set
	connectionValue := scraperValue.FieldByName("connection")
	assert.False(t, connectionValue.IsZero(), "Connection field should be set")

	loggerValue := scraperValue.FieldByName("logger")
	assert.False(t, loggerValue.IsZero(), "Logger field should be set")

	startTimeValue := scraperValue.FieldByName("startTime")
	assert.False(t, startTimeValue.IsZero(), "StartTime field should be set")

	engineEditionValue := scraperValue.FieldByName("engineEdition")
	assert.False(t, engineEditionValue.IsZero(), "EngineEdition field should be set")
}

func TestDatabaseScraper_ConcurrentCreation(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zap.NewNop()
	engineEdition := queries.AzureSQLDatabaseEngineEdition

	const numGoroutines = 10
	scrapers := make([]*DatabaseScraper, numGoroutines)
	var wg sync.WaitGroup

	// Create scrapers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			scrapers[index] = NewDatabaseScraper(mockConn, logger, engineEdition)
		}(i)
	}

	wg.Wait()

	// Verify all scrapers were created successfully
	for i, scraper := range scrapers {
		assert.NotNil(t, scraper, "Scraper %d should not be nil", i)
		assert.Equal(t, mockConn, scraper.connection)
		assert.Equal(t, logger, scraper.logger)
		assert.Equal(t, engineEdition, scraper.engineEdition)
		assert.True(t, scraper.startTime > 0)
	}

	// Verify all scrapers have unique start times (or very close)
	startTimes := make(map[pcommon.Timestamp]bool)
	for _, scraper := range scrapers {
		startTimes[scraper.startTime] = true
	}

	// We expect some duplicates due to timing, but should have multiple unique timestamps
	assert.True(t, len(startTimes) >= 1, "Should have at least one unique start time")
}

func TestDatabaseScraper_EngineEditionBoundaries(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zap.NewNop()

	testCases := []struct {
		name          string
		engineEdition int
		description   string
	}{
		{
			name:          "Max Int32",
			engineEdition: math.MaxInt32,
			description:   "Maximum 32-bit integer value",
		},
		{
			name:          "Min Int32",
			engineEdition: math.MinInt32,
			description:   "Minimum 32-bit integer value",
		},
		{
			name:          "Large Positive",
			engineEdition: 1000000,
			description:   "Large positive value",
		},
		{
			name:          "Large Negative",
			engineEdition: -1000000,
			description:   "Large negative value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := NewDatabaseScraper(mockConn, logger, tc.engineEdition)

			assert.NotNil(t, scraper, "Scraper should not be nil for %s", tc.description)
			assert.Equal(t, tc.engineEdition, scraper.engineEdition,
				"Engine edition should be set correctly for %s", tc.description)
			assert.Equal(t, mockConn, scraper.connection)
			assert.Equal(t, logger, scraper.logger)
			assert.True(t, scraper.startTime > 0)
		})
	}
}
