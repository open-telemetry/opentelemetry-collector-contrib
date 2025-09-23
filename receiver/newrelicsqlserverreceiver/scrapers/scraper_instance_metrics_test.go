// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

// MockSQLConnection is a mock implementation of SQLConnectionInterface
type MockSQLConnection struct {
	mock.Mock
}

func (m *MockSQLConnection) Query(ctx context.Context, dest interface{}, query string) error {
	args := m.Called(ctx, dest, query)
	if args.Get(0) != nil {
		// Simulate successful query result
		if results, ok := dest.(*[]models.InstanceMemoryDefinitionsModel); ok {
			*results = args.Get(0).([]models.InstanceMemoryDefinitionsModel)
		} else if results, ok := dest.(*[]models.InstanceStatsModel); ok {
			*results = args.Get(0).([]models.InstanceStatsModel)
		}
	}
	return args.Error(1)
}

// Helper function to create test memory results
func createTestMemoryResults() []models.InstanceMemoryDefinitionsModel {
	totalMemory := 8589934592.0     // 8GB in bytes
	availableMemory := 2147483648.0 // 2GB in bytes
	utilization := 75.0             // 75%

	return []models.InstanceMemoryDefinitionsModel{
		{
			TotalPhysicalMemory:     &totalMemory,
			AvailablePhysicalMemory: &availableMemory,
			MemoryUtilization:       &utilization,
		},
	}
}

// Helper function to create test stats results
func createTestStatsResults() []models.InstanceStatsModel {
	return []models.InstanceStatsModel{
		{
			SQLCompilations:            int64Ptr(1500),
			SQLRecompilations:          int64Ptr(25),
			UserConnections:            int64Ptr(42),
			LockWaitTimeMs:             int64Ptr(150),
			PageSplitsSec:              int64Ptr(10),
			CheckpointPagesSec:         int64Ptr(5),
			DeadlocksSec:               int64Ptr(0),
			UserErrors:                 int64Ptr(2),
			KillConnectionErrors:       int64Ptr(0),
			BatchRequestSec:            int64Ptr(2500),
			PageLifeExpectancySec:      float64Ptr(30000.0),
			TransactionsSec:            int64Ptr(1200),
			ForcedParameterizationsSec: int64Ptr(0),
		},
	}
}

// Helper functions for pointer creation
func int64Ptr(v int64) *int64 {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}

func TestNewInstanceScraper(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t)
	engineEdition := 0

	scraper := NewInstanceScraper(mockConn, logger, engineEdition)

	assert.NotNil(t, scraper)
	assert.Equal(t, mockConn, scraper.connection)
	assert.Equal(t, logger, scraper.logger)
	assert.Equal(t, engineEdition, scraper.engineEdition)
	assert.NotZero(t, scraper.startTime)
}

func TestInstanceScraper_GetQueryForMetric(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))

	tests := []struct {
		name          string
		engineEdition int
		metricName    string
		expectFound   bool
		expectQuery   bool
	}{
		{
			name:          "Memory metrics for standard SQL Server",
			engineEdition: queries.StandardSQLServerEngineEdition,
			metricName:    "sqlserver.instance.memory_metrics",
			expectFound:   true,
			expectQuery:   true,
		},
		{
			name:          "Comprehensive stats for standard SQL Server",
			engineEdition: queries.StandardSQLServerEngineEdition,
			metricName:    "sqlserver.instance.comprehensive_stats",
			expectFound:   true,
			expectQuery:   true,
		},
		{
			name:          "Memory metrics for Azure SQL Managed Instance",
			engineEdition: queries.AzureSQLManagedInstanceEngineEdition,
			metricName:    "sqlserver.instance.memory_metrics",
			expectFound:   true,
			expectQuery:   true,
		},
		{
			name:          "Non-existent metric",
			engineEdition: queries.StandardSQLServerEngineEdition,
			metricName:    "sqlserver.instance.nonexistent",
			expectFound:   false,
			expectQuery:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := NewInstanceScraper(mockConn, logger, tt.engineEdition)

			query, found := scraper.getQueryForMetric(tt.metricName)

			assert.Equal(t, tt.expectFound, found)
			if tt.expectQuery {
				assert.NotEmpty(t, query)
			} else {
				assert.Empty(t, query)
			}
		})
	}
}

func TestInstanceScraper_ScrapeInstanceMemoryMetrics_Success(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	ctx := context.Background()
	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock expectation
	testResults := createTestMemoryResults()
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceMemoryDefinitionsModel"), mock.AnythingOfType("string")).
		Return(testResults, nil)

	// Execute
	err := scraper.ScrapeInstanceMemoryMetrics(ctx, scopeMetrics)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len()) // 3 memory metrics
	mockConn.AssertExpectations(t)

	// Verify metrics
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		assert.NotEmpty(t, metric.Name())
		assert.NotEmpty(t, metric.Description())
		assert.NotEmpty(t, metric.Unit())

		// Check that it's a gauge metric
		assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
		assert.Equal(t, 1, metric.Gauge().DataPoints().Len())

		dataPoint := metric.Gauge().DataPoints().At(0)
		assert.NotZero(t, dataPoint.Timestamp())
		assert.NotZero(t, dataPoint.StartTimestamp())

		// Check attributes
		attrs := dataPoint.Attributes()
		metricType, exists := attrs.Get("metric.type")
		assert.True(t, exists)
		assert.Equal(t, "gauge", metricType.Str())
	}
}

func TestInstanceScraper_ScrapeInstanceMemoryMetrics_QueryError(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t)
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	ctx := context.Background()
	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock to return error
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceMemoryDefinitionsModel"), mock.AnythingOfType("string")).
		Return(nil, errors.New("connection failed"))

	// Execute
	err := scraper.ScrapeInstanceMemoryMetrics(ctx, scopeMetrics)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute instance memory query")
	assert.Equal(t, 0, scopeMetrics.Metrics().Len())
	mockConn.AssertExpectations(t)
}

func TestInstanceScraper_ScrapeInstanceMemoryMetrics_NoResults(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t)
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	ctx := context.Background()
	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock to return empty results
	emptyResults := []models.InstanceMemoryDefinitionsModel{}
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceMemoryDefinitionsModel"), mock.AnythingOfType("string")).
		Return(emptyResults, nil)

	// Execute
	err := scraper.ScrapeInstanceMemoryMetrics(ctx, scopeMetrics)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no results returned from instance memory query")
	mockConn.AssertExpectations(t)
}

func TestInstanceScraper_ScrapeInstanceMemoryMetrics_MultipleResults(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	ctx := context.Background()
	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock to return multiple results
	testResults := append(createTestMemoryResults(), createTestMemoryResults()...)
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceMemoryDefinitionsModel"), mock.AnythingOfType("string")).
		Return(testResults, nil)

	// Execute
	err := scraper.ScrapeInstanceMemoryMetrics(ctx, scopeMetrics)

	// Assert - should succeed but log warning
	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len())
	mockConn.AssertExpectations(t)
}

func TestInstanceScraper_ScrapeInstanceMemoryMetrics_NilValues(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t)
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	ctx := context.Background()
	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock to return results with all nil values
	nilResults := []models.InstanceMemoryDefinitionsModel{
		{
			TotalPhysicalMemory:     nil,
			AvailablePhysicalMemory: nil,
			MemoryUtilization:       nil,
		},
	}
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceMemoryDefinitionsModel"), mock.AnythingOfType("string")).
		Return(nilResults, nil)

	// Execute
	err := scraper.ScrapeInstanceMemoryMetrics(ctx, scopeMetrics)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all memory metrics are null")
	mockConn.AssertExpectations(t)
}

func TestInstanceScraper_ScrapeInstanceStats_Success(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	ctx := context.Background()
	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock expectation
	testResults := createTestStatsResults()
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceStatsModel"), mock.AnythingOfType("string")).
		Return(testResults, nil)

	// Execute
	err := scraper.ScrapeInstanceStats(ctx, scopeMetrics)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 13, scopeMetrics.Metrics().Len()) // 13 comprehensive stats metrics
	mockConn.AssertExpectations(t)

	// Verify metrics have correct structure
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		assert.NotEmpty(t, metric.Name())
		assert.NotEmpty(t, metric.Description())
		assert.NotEmpty(t, metric.Unit())

		// Check metric type - could be either gauge or sum based on source_type
		metricType := metric.Type()
		assert.True(t, metricType == pmetric.MetricTypeGauge || metricType == pmetric.MetricTypeSum,
			"Metric type should be either gauge or sum, got %s", metricType.String())

		if metricType == pmetric.MetricTypeGauge {
			assert.Equal(t, 1, metric.Gauge().DataPoints().Len())
			dataPoint := metric.Gauge().DataPoints().At(0)
			assert.NotZero(t, dataPoint.Timestamp())
			assert.NotZero(t, dataPoint.StartTimestamp())
		} else if metricType == pmetric.MetricTypeSum {
			assert.Equal(t, 1, metric.Sum().DataPoints().Len())
			dataPoint := metric.Sum().DataPoints().At(0)
			assert.NotZero(t, dataPoint.Timestamp())
			assert.NotZero(t, dataPoint.StartTimestamp())
		}
	}
}

func TestInstanceScraper_ScrapeInstanceStats_QueryError(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t)
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	ctx := context.Background()
	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock to return error
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceStatsModel"), mock.AnythingOfType("string")).
		Return(nil, errors.New("database connection failed"))

	// Execute
	err := scraper.ScrapeInstanceStats(ctx, scopeMetrics)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute instance stats query")
	assert.Equal(t, 0, scopeMetrics.Metrics().Len())
	mockConn.AssertExpectations(t)
}

func TestInstanceScraper_ScrapeInstanceStats_NoResults(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t)
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	ctx := context.Background()
	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock to return empty results
	emptyResults := []models.InstanceStatsModel{}
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceStatsModel"), mock.AnythingOfType("string")).
		Return(emptyResults, nil)

	// Execute
	err := scraper.ScrapeInstanceStats(ctx, scopeMetrics)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no results returned from instance stats query")
	mockConn.AssertExpectations(t)
}

func TestInstanceScraper_ScrapeInstanceStats_WithNilValues(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	ctx := context.Background()
	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock to return results with some nil values
	statsResults := []models.InstanceStatsModel{
		{
			SQLCompilations:            int64Ptr(1500),
			SQLRecompilations:          nil, // nil value
			UserConnections:            int64Ptr(42),
			LockWaitTimeMs:             nil, // nil value
			PageSplitsSec:              int64Ptr(10),
			CheckpointPagesSec:         int64Ptr(5),
			DeadlocksSec:               int64Ptr(0),
			UserErrors:                 nil, // nil value
			KillConnectionErrors:       int64Ptr(0),
			BatchRequestSec:            int64Ptr(2500),
			PageLifeExpectancySec:      float64Ptr(30000.0),
			TransactionsSec:            int64Ptr(1200),
			ForcedParameterizationsSec: nil, // nil value
		},
	}
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceStatsModel"), mock.AnythingOfType("string")).
		Return(statsResults, nil)

	// Execute
	err := scraper.ScrapeInstanceStats(ctx, scopeMetrics)

	// Assert - should succeed, skipping nil values
	assert.NoError(t, err)
	// Should have fewer metrics due to nil values being skipped
	assert.Greater(t, scopeMetrics.Metrics().Len(), 0)
	assert.Less(t, scopeMetrics.Metrics().Len(), 13)
	mockConn.AssertExpectations(t)
}

func TestInstanceScraper_NoQueryAvailable(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t)
	// Use an unknown engine edition that will fall back to default query
	scraper := NewInstanceScraper(mockConn, logger, 999) // Unknown engine edition

	ctx := context.Background()
	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock to simulate query failure for fallback scenario
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceStatsModel"), mock.AnythingOfType("string")).
		Return(nil, errors.New("query not supported for this engine"))

	// Execute ScrapeInstanceStats
	err := scraper.ScrapeInstanceStats(ctx, scopeMetrics)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute instance stats query")
	mockConn.AssertExpectations(t)
}

func TestInstanceScraper_ContextTimeout(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t)
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	// Create a context that will timeout immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond) // Ensure context is expired

	scopeMetrics := pmetric.NewScopeMetrics()

	// Setup mock to simulate slow query
	mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceMemoryDefinitionsModel"), mock.AnythingOfType("string")).
		Return(nil, context.DeadlineExceeded)

	// Execute
	err := scraper.ScrapeInstanceMemoryMetrics(ctx, scopeMetrics)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute instance memory query")
	mockConn.AssertExpectations(t)
}

func TestInstanceScraper_ProcessInstanceStatsMetrics_UnsupportedFieldType(t *testing.T) {
	// This test would require creating a model with unsupported field types
	// For now, we'll test the current supported types work correctly
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	scopeMetrics := pmetric.NewScopeMetrics()
	testResults := createTestStatsResults()

	// Execute the processing directly
	err := scraper.processInstanceStatsMetrics(testResults[0], scopeMetrics)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 13, scopeMetrics.Metrics().Len())
}

func TestInstanceScraper_DifferentEngineEditions(t *testing.T) {
	engines := []struct {
		name    string
		edition int
	}{
		{"Standard SQL Server", queries.StandardSQLServerEngineEdition},
		{"Azure SQL Database", queries.AzureSQLDatabaseEngineEdition},
		{"Azure SQL Managed Instance", queries.AzureSQLManagedInstanceEngineEdition},
	}

	for _, engine := range engines {
		t.Run(engine.name, func(t *testing.T) {
			mockConn := &MockSQLConnection{}
			logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
			scraper := NewInstanceScraper(mockConn, logger, engine.edition)

			assert.Equal(t, engine.edition, scraper.engineEdition)

			// Test that different engines can attempt to get queries
			_, found := scraper.getQueryForMetric("sqlserver.instance.memory_metrics")
			// The availability depends on the engine edition configuration
			if engine.edition == queries.StandardSQLServerEngineEdition ||
				engine.edition == queries.AzureSQLManagedInstanceEngineEdition {
				assert.True(t, found, "Memory metrics should be available for %s", engine.name)
			}
		})
	}
}

func TestInstanceScraper_ProcessMemoryMetricsDetails(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	scopeMetrics := pmetric.NewScopeMetrics()
	testData := createTestMemoryResults()[0]

	// Execute the processing directly
	err := scraper.processInstanceMemoryMetrics(testData, scopeMetrics)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 3, scopeMetrics.Metrics().Len())

	// Check each metric in detail
	metricNames := make(map[string]bool)
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		metricNames[metric.Name()] = true

		// Verify common properties
		assert.NotEmpty(t, metric.Description())
		assert.NotEmpty(t, metric.Unit())
		assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

		// Verify data point
		dataPoint := metric.Gauge().DataPoints().At(0)
		assert.NotZero(t, dataPoint.Timestamp())
		assert.NotZero(t, dataPoint.StartTimestamp())

		// Verify attributes
		attrs := dataPoint.Attributes()
		metricType, exists := attrs.Get("metric.type")
		assert.True(t, exists)
		assert.Equal(t, "gauge", metricType.Str())

		// Verify values are positive (memory values should be positive)
		switch metric.Name() {
		case "sqlserver.instance.memory.total_physical":
			assert.Greater(t, dataPoint.DoubleValue(), 0.0)
		case "sqlserver.instance.memory.available_physical":
			assert.Greater(t, dataPoint.DoubleValue(), 0.0)
		case "sqlserver.instance.memory.utilization":
			assert.GreaterOrEqual(t, dataPoint.DoubleValue(), 0.0)
			assert.LessOrEqual(t, dataPoint.DoubleValue(), 100.0)
		}
	}

	// Verify all expected metrics are present
	expectedMetrics := []string{
		"sqlserver.instance.memory.total_physical",
		"sqlserver.instance.memory.available_physical",
		"sqlserver.instance.memory.utilization",
	}
	for _, expected := range expectedMetrics {
		assert.True(t, metricNames[expected], "Expected metric %s not found", expected)
	}
}

func TestInstanceScraper_ProcessStatsMetricsDetails(t *testing.T) {
	mockConn := &MockSQLConnection{}
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

	scopeMetrics := pmetric.NewScopeMetrics()
	testData := createTestStatsResults()[0]

	// Execute the processing directly
	err := scraper.processInstanceStatsMetrics(testData, scopeMetrics)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 13, scopeMetrics.Metrics().Len())

	// Verify all metrics have correct structure and values
	metricNames := make(map[string]pmetric.Metric)
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		metricNames[metric.Name()] = metric

		// Verify common properties
		assert.NotEmpty(t, metric.Description())
		assert.NotEmpty(t, metric.Unit())

		// Verify metric type and data point based on actual type
		metricType := metric.Type()
		assert.True(t, metricType == pmetric.MetricTypeGauge || metricType == pmetric.MetricTypeSum,
			"Metric type should be either gauge or sum")

		if metricType == pmetric.MetricTypeGauge {
			dataPoint := metric.Gauge().DataPoints().At(0)
			assert.NotZero(t, dataPoint.Timestamp())
			assert.NotZero(t, dataPoint.StartTimestamp())
		} else if metricType == pmetric.MetricTypeSum {
			dataPoint := metric.Sum().DataPoints().At(0)
			assert.NotZero(t, dataPoint.Timestamp())
			assert.NotZero(t, dataPoint.StartTimestamp())
		}
	}

	// Test specific metrics with expected values
	expectedMetricsWithValues := map[string]float64{
		"sqlserver.stats.sql_compilations_per_sec":            1500.0,
		"sqlserver.stats.sql_recompilations_per_sec":          25.0,
		"sqlserver.stats.connections":                         42.0,
		"sqlserver.stats.lock_waits_per_sec":                  150.0,
		"sqlserver.access.page_splits_per_sec":                10.0,
		"sqlserver.buffer.checkpoint_pages_per_sec":           5.0,
		"sqlserver.stats.deadlocks_per_sec":                   0.0,
		"sqlserver.stats.user_errors_per_sec":                 2.0,
		"sqlserver.stats.kill_connection_errors_per_sec":      0.0,
		"sqlserver.bufferpool.batch_requests_per_sec":         2500.0,
		"sqlserver.bufferpool.page_life_expectancy_ms":        30000.0,
		"sqlserver.instance.transactions_per_sec":             1200.0,
		"sqlserver.instance.forced_parameterizations_per_sec": 0.0,
	}

	for metricName, expectedValue := range expectedMetricsWithValues {
		metric, found := metricNames[metricName]
		assert.True(t, found, "Expected metric %s not found", metricName)
		if found {
			var actualValue float64
			if metric.Type() == pmetric.MetricTypeGauge {
				dataPoint := metric.Gauge().DataPoints().At(0)
				actualValue = dataPoint.DoubleValue()
			} else if metric.Type() == pmetric.MetricTypeSum {
				dataPoint := metric.Sum().DataPoints().At(0)
				actualValue = dataPoint.DoubleValue()
			}
			assert.Equal(t, expectedValue, actualValue, "Unexpected value for metric %s", metricName)
		}
	}
}

func TestInstanceScraper_EdgeCases(t *testing.T) {
	t.Run("All nil stats values", func(t *testing.T) {
		mockConn := &MockSQLConnection{}
		logger := zaptest.NewLogger(t)
		scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

		ctx := context.Background()
		scopeMetrics := pmetric.NewScopeMetrics()

		// Setup mock to return results with all nil values
		nilResults := []models.InstanceStatsModel{
			{
				SQLCompilations:            nil,
				SQLRecompilations:          nil,
				UserConnections:            nil,
				LockWaitTimeMs:             nil,
				PageSplitsSec:              nil,
				CheckpointPagesSec:         nil,
				DeadlocksSec:               nil,
				UserErrors:                 nil,
				KillConnectionErrors:       nil,
				BatchRequestSec:            nil,
				PageLifeExpectancySec:      nil,
				TransactionsSec:            nil,
				ForcedParameterizationsSec: nil,
			},
		}
		mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceStatsModel"), mock.AnythingOfType("string")).
			Return(nilResults, nil)

		// Execute
		err := scraper.ScrapeInstanceStats(ctx, scopeMetrics)

		// Assert - should error because all values are nil
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "all comprehensive stats are null")
		mockConn.AssertExpectations(t)
	})

	t.Run("Partial nil memory values", func(t *testing.T) {
		mockConn := &MockSQLConnection{}
		logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
		scraper := NewInstanceScraper(mockConn, logger, queries.StandardSQLServerEngineEdition)

		ctx := context.Background()
		scopeMetrics := pmetric.NewScopeMetrics()

		totalMemory := 8589934592.0 // 8GB in bytes
		// Setup mock to return results with some nil values
		partialResults := []models.InstanceMemoryDefinitionsModel{
			{
				TotalPhysicalMemory:     &totalMemory,
				AvailablePhysicalMemory: nil, // nil value
				MemoryUtilization:       nil, // nil value
			},
		}
		mockConn.On("Query", ctx, mock.AnythingOfType("*[]models.InstanceMemoryDefinitionsModel"), mock.AnythingOfType("string")).
			Return(partialResults, nil)

		// Execute
		err := scraper.ScrapeInstanceMemoryMetrics(ctx, scopeMetrics)

		// Assert - should succeed, only creating metrics for non-nil values
		assert.NoError(t, err)
		assert.Equal(t, 1, scopeMetrics.Metrics().Len()) // Only total memory
		mockConn.AssertExpectations(t)
	})
}
