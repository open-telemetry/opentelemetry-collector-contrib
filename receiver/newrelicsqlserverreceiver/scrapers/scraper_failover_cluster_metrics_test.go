// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

func TestNewFailoverClusterScraper(t *testing.T) {
	tests := []struct {
		name          string
		engineEdition int
		expectNil     bool
	}{
		{
			name:          "standard_sql_server",
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectNil:     false,
		},
		{
			name:          "azure_sql_database",
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectNil:     false,
		},
		{
			name:          "azure_sql_managed_instance",
			engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
			expectNil:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockConn := &MockSQLConnection{}

			scraper := NewFailoverClusterScraper(mockConn, logger, tt.engineEdition)

			if tt.expectNil {
				assert.Nil(t, scraper)
			} else {
				require.NotNil(t, scraper)
				assert.Equal(t, tt.engineEdition, scraper.engineEdition)
				assert.NotNil(t, scraper.connection)
				assert.NotNil(t, scraper.logger)
				assert.True(t, scraper.startTime > 0)
			}
		})
	}
}

func TestFailoverClusterScraper_ProcessFailoverClusterReplicaMetrics(t *testing.T) {
	tests := []struct {
		name           string
		result         models.FailoverClusterReplicaMetrics
		engineEdition  int
		expectError    bool
		expectMetrics  int
		validateFields func(t *testing.T, metrics pmetric.ScopeMetrics)
	}{
		{
			name: "all_metrics_available",
			result: models.FailoverClusterReplicaMetrics{
				LogBytesReceivedPerSec: int64Ptr(1048576), // 1MB/sec
				TransactionDelayMs:     int64Ptr(50),      // 50ms delay
				FlowControlTimeMs:      int64Ptr(10),      // 10ms/sec flow control
			},
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectError:   false,
			expectMetrics: 3,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 3, metrics.Metrics().Len())

				metricNames := make(map[string]bool)
				for i := 0; i < metrics.Metrics().Len(); i++ {
					metric := metrics.Metrics().At(i)
					metricNames[metric.Name()] = true

					// Verify gauge type
					assert.True(t, metric.Type() == pmetric.MetricTypeGauge)

					// Verify data points
					gauge := metric.Gauge()
					require.Equal(t, 1, gauge.DataPoints().Len())

					dataPoint := gauge.DataPoints().At(0)
					assert.True(t, dataPoint.Timestamp() > 0)
					assert.True(t, dataPoint.StartTimestamp() > 0)

					// Verify attributes
					attrs := dataPoint.Attributes()
					metricType, exists := attrs.Get("metric.type")
					assert.True(t, exists)
					assert.Equal(t, "gauge", metricType.Str())

					metricSource, exists := attrs.Get("metric.source")
					assert.True(t, exists)
					assert.Equal(t, "sys.dm_os_performance_counters", metricSource.Str())

					metricCategory, exists := attrs.Get("metric.category")
					assert.True(t, exists)
					assert.Equal(t, "always_on_availability_group", metricCategory.Str())
				}

				// Verify all expected metrics are present
				expectedMetrics := []string{
					"sqlserver.failover_cluster.log_bytes_received_per_sec",
					"sqlserver.failover_cluster.transaction_delay_ms",
					"sqlserver.failover_cluster.flow_control_time_ms",
				}

				for _, expectedMetric := range expectedMetrics {
					assert.True(t, metricNames[expectedMetric], "Expected metric %s not found", expectedMetric)
				}
			},
		},
		{
			name: "partial_metrics_available",
			result: models.FailoverClusterReplicaMetrics{
				LogBytesReceivedPerSec: int64Ptr(524288), // 512KB/sec
				TransactionDelayMs:     nil,              // No transaction delay data
				FlowControlTimeMs:      int64Ptr(5),      // 5ms/sec flow control
			},
			engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
			expectError:   false,
			expectMetrics: 2, // Only non-nil metrics should be processed
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 2, metrics.Metrics().Len())

				// Verify that nil metrics are skipped
				metricNames := make(map[string]bool)
				for i := 0; i < metrics.Metrics().Len(); i++ {
					metric := metrics.Metrics().At(i)
					metricNames[metric.Name()] = true
				}

				assert.True(t, metricNames["sqlserver.failover_cluster.log_bytes_received_per_sec"])
				assert.True(t, metricNames["sqlserver.failover_cluster.flow_control_time_ms"])
				assert.False(t, metricNames["sqlserver.failover_cluster.transaction_delay_ms"])
			},
		},
		{
			name: "zero_values_valid",
			result: models.FailoverClusterReplicaMetrics{
				LogBytesReceivedPerSec: int64Ptr(0), // No log traffic
				TransactionDelayMs:     int64Ptr(0), // No delay
				FlowControlTimeMs:      int64Ptr(0), // No flow control
			},
			engineEdition: queries.StandardSQLServerEngineEdition,
			expectError:   false,
			expectMetrics: 3,
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				require.Equal(t, 3, metrics.Metrics().Len())

				for i := 0; i < metrics.Metrics().Len(); i++ {
					metric := metrics.Metrics().At(i)
					gauge := metric.Gauge()
					dataPoint := gauge.DataPoints().At(0)
					assert.Equal(t, int64(0), dataPoint.IntValue())
				}
			},
		},
		{
			name: "all_metrics_nil",
			result: models.FailoverClusterReplicaMetrics{
				LogBytesReceivedPerSec: nil,
				TransactionDelayMs:     nil,
				FlowControlTimeMs:      nil,
			},
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			expectError:   false,
			expectMetrics: 0, // No metrics should be created for all nil values
			validateFields: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				assert.Equal(t, 0, metrics.Metrics().Len())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockConn := &MockSQLConnection{}
			scraper := NewFailoverClusterScraper(mockConn, logger, tt.engineEdition)

			// Create scope metrics
			scopeMetrics := pmetric.NewScopeMetrics()

			// Process the metrics
			err := scraper.processFailoverClusterReplicaMetrics(tt.result, scopeMetrics)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Validate the number of metrics created
			assert.Equal(t, tt.expectMetrics, scopeMetrics.Metrics().Len())

			// Run additional validation if provided
			if tt.validateFields != nil {
				tt.validateFields(t, scopeMetrics)
			}
		})
	}
}

func TestFailoverClusterScraper_CentralizedQuerySelection(t *testing.T) {
	tests := []struct {
		name          string
		engineEdition int
		metricName    string
		expectFound   bool
		expectQuery   string
	}{
		{
			name:          "standard_sql_server_replica_query",
			engineEdition: queries.StandardSQLServerEngineEdition,
			metricName:    "sqlserver.failover_cluster.replica_metrics",
			expectFound:   true,
			expectQuery:   queries.FailoverClusterReplicaQuery,
		},
		{
			name:          "azure_sql_database_replica_query",
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			metricName:    "sqlserver.failover_cluster.replica_metrics",
			expectFound:   true,
			expectQuery:   queries.FailoverClusterReplicaQueryAzureSQL,
		},
		{
			name:          "azure_sql_managed_instance_replica_query",
			engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
			metricName:    "sqlserver.failover_cluster.replica_metrics",
			expectFound:   true,
			expectQuery:   queries.FailoverClusterReplicaQueryAzureMI,
		},
		{
			name:          "unknown_metric_name",
			engineEdition: queries.StandardSQLServerEngineEdition,
			metricName:    "sqlserver.unknown.metric",
			expectFound:   false,
			expectQuery:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test using centralized query selection
			query, found := queries.GetQueryForMetric(queries.FailoverClusterQueries, tt.metricName, tt.engineEdition)

			assert.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				assert.Equal(t, tt.expectQuery, query)
			}
		})
	}
}

func TestFailoverClusterScraper_TimestampValidation(t *testing.T) {
	logger := zap.NewNop()
	mockConn := &MockSQLConnection{}
	scraper := NewFailoverClusterScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	// Capture time before processing
	beforeProcessing := time.Now()

	result := models.FailoverClusterReplicaMetrics{
		LogBytesReceivedPerSec: int64Ptr(1024),
		TransactionDelayMs:     int64Ptr(25),
		FlowControlTimeMs:      int64Ptr(5),
	}

	scopeMetrics := pmetric.NewScopeMetrics()
	err := scraper.processFailoverClusterReplicaMetrics(result, scopeMetrics)

	// Capture time after processing
	afterProcessing := time.Now()

	require.NoError(t, err)
	require.Equal(t, 3, scopeMetrics.Metrics().Len())

	// Validate timestamps for all metrics
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		gauge := metric.Gauge()
		require.Equal(t, 1, gauge.DataPoints().Len())

		dataPoint := gauge.DataPoints().At(0)

		// Convert timestamps to time.Time for comparison
		timestamp := dataPoint.Timestamp().AsTime()
		startTimestamp := dataPoint.StartTimestamp().AsTime()

		// Verify timestamps are within reasonable bounds
		assert.True(t, timestamp.After(beforeProcessing.Add(-time.Second)), "Timestamp should be recent")
		assert.True(t, timestamp.Before(afterProcessing.Add(time.Second)), "Timestamp should not be in the future")

		// Start timestamp should be the scraper's start time
		assert.Equal(t, scraper.startTime.AsTime().Unix(), startTimestamp.Unix())

		// Timestamp should be after or equal to start timestamp
		assert.True(t, timestamp.After(startTimestamp) || timestamp.Equal(startTimestamp))
	}
}

func TestFailoverClusterScraper_ProcessFailoverClusterReplicaStateMetrics(t *testing.T) {
	tests := []struct {
		name            string
		result          models.FailoverClusterReplicaStateMetrics
		expectMetrics   int
		validateMetrics func(t *testing.T, metrics pmetric.ScopeMetrics)
	}{
		{
			name: "all_metrics_available",
			result: models.FailoverClusterReplicaStateMetrics{
				ReplicaServerName: "SQL-AG-01",
				DatabaseName:      "MyDatabase",
				LogSendQueueKB:    int64Ptr(1024),
				RedoQueueKB:       int64Ptr(512),
				RedoRateKBSec:     int64Ptr(256),
			},
			expectMetrics: 3,
			validateMetrics: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				// Verify that all three replica state metrics are created
				assert.Equal(t, 3, metrics.Metrics().Len())

				for i := 0; i < metrics.Metrics().Len(); i++ {
					metric := metrics.Metrics().At(i)
					gauge := metric.Gauge()
					assert.Equal(t, 1, gauge.DataPoints().Len())

					dataPoint := gauge.DataPoints().At(0)

					// Check attributes
					replicaServerName, found := dataPoint.Attributes().Get("replica_server_name")
					require.True(t, found)
					assert.Equal(t, "SQL-AG-01", replicaServerName.Str())

					databaseName, found := dataPoint.Attributes().Get("database_name")
					require.True(t, found)
					assert.Equal(t, "MyDatabase", databaseName.Str())

					metricSource, found := dataPoint.Attributes().Get("metric.source")
					require.True(t, found)
					assert.Equal(t, "sys.dm_hadr_database_replica_states", metricSource.Str())
				}
			},
		},
		{
			name: "partial_metrics_available",
			result: models.FailoverClusterReplicaStateMetrics{
				ReplicaServerName: "SQL-AG-02",
				DatabaseName:      "TestDB",
				LogSendQueueKB:    int64Ptr(2048),
				RedoQueueKB:       nil, // This will be skipped
				RedoRateKBSec:     int64Ptr(128),
			},
			expectMetrics: 2,
			validateMetrics: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				assert.Equal(t, 2, metrics.Metrics().Len())
			},
		},
		{
			name: "zero_values_valid",
			result: models.FailoverClusterReplicaStateMetrics{
				ReplicaServerName: "SQL-AG-03",
				DatabaseName:      "EmptyDB",
				LogSendQueueKB:    int64Ptr(0),
				RedoQueueKB:       int64Ptr(0),
				RedoRateKBSec:     int64Ptr(0),
			},
			expectMetrics: 3,
			validateMetrics: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				assert.Equal(t, 3, metrics.Metrics().Len())

				for i := 0; i < metrics.Metrics().Len(); i++ {
					metric := metrics.Metrics().At(i)
					gauge := metric.Gauge()
					dataPoint := gauge.DataPoints().At(0)
					assert.Equal(t, int64(0), dataPoint.IntValue())
				}
			},
		},
		{
			name: "all_metrics_nil",
			result: models.FailoverClusterReplicaStateMetrics{
				ReplicaServerName: "SQL-AG-04",
				DatabaseName:      "NullDB",
				LogSendQueueKB:    nil,
				RedoQueueKB:       nil,
				RedoRateKBSec:     nil,
			},
			expectMetrics: 0,
			validateMetrics: func(t *testing.T, metrics pmetric.ScopeMetrics) {
				assert.Equal(t, 0, metrics.Metrics().Len())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockConn := &MockSQLConnection{}
			scraper := NewFailoverClusterScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

			metrics := pmetric.NewMetrics()
			scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

			err := scraper.processFailoverClusterReplicaStateMetrics(tt.result, scopeMetrics)
			require.NoError(t, err)

			assert.Equal(t, tt.expectMetrics, scopeMetrics.Metrics().Len())

			if tt.validateMetrics != nil {
				tt.validateMetrics(t, scopeMetrics)
			}
		})
	}
}

func TestFailoverClusterScraper_CentralizedQuerySelection_ReplicaState(t *testing.T) {
	tests := []struct {
		name          string
		engineEdition int
		metricName    string
		expectFound   bool
		expectQuery   string
	}{
		{
			name:          "standard_sql_server_replica_state_query",
			engineEdition: queries.StandardSQLServerEngineEdition,
			metricName:    "sqlserver.failover_cluster.replica_state_metrics",
			expectFound:   true,
			expectQuery:   queries.FailoverClusterReplicaStateQuery,
		},
		{
			name:          "azure_sql_database_replica_state_query",
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			metricName:    "sqlserver.failover_cluster.replica_state_metrics",
			expectFound:   true,
			expectQuery:   queries.FailoverClusterReplicaStateQueryAzureSQL,
		},
		{
			name:          "azure_sql_managed_instance_replica_state_query",
			engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
			metricName:    "sqlserver.failover_cluster.replica_state_metrics",
			expectFound:   true,
			expectQuery:   queries.FailoverClusterReplicaStateQueryAzureMI,
		},
		{
			name:          "unknown_metric_name",
			engineEdition: queries.StandardSQLServerEngineEdition,
			metricName:    "unknown_metric",
			expectFound:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test using centralized query selection
			query, found := queries.GetQueryForMetric(queries.FailoverClusterQueries, tt.metricName, tt.engineEdition)

			assert.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				assert.Equal(t, tt.expectQuery, query)
			} else {
				assert.Empty(t, query)
			}
		})
	}
}
