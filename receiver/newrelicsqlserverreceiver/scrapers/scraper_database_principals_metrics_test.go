// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

// MockSQLConnectionInterface provides a mock implementation for testing
type MockSQLConnectionInterface struct {
	queryResults map[string]interface{}
	queryError   error
}

func (m *MockSQLConnectionInterface) Query(ctx context.Context, dest interface{}, query string) error {
	if m.queryError != nil {
		return m.queryError
	}

	// Set mock results based on query type
	if results, exists := m.queryResults[query]; exists {
		switch v := dest.(type) {
		case *[]models.DatabasePrincipalsMetrics:
			if mockResults, ok := results.([]models.DatabasePrincipalsMetrics); ok {
				*v = mockResults
			}
		case *[]models.DatabasePrincipalsSummary:
			if mockResults, ok := results.([]models.DatabasePrincipalsSummary); ok {
				*v = mockResults
			}
		case *[]models.DatabasePrincipalActivity:
			if mockResults, ok := results.([]models.DatabasePrincipalActivity); ok {
				*v = mockResults
			}
		case *[]models.DatabaseRoleMembershipMetrics:
			if mockResults, ok := results.([]models.DatabaseRoleMembershipMetrics); ok {
				*v = mockResults
			}
		case *[]models.DatabaseRoleMembershipSummary:
			if mockResults, ok := results.([]models.DatabaseRoleMembershipSummary); ok {
				*v = mockResults
			}
		case *[]models.DatabaseRoleHierarchy:
			if mockResults, ok := results.([]models.DatabaseRoleHierarchy); ok {
				*v = mockResults
			}
		case *[]models.DatabaseRoleActivity:
			if mockResults, ok := results.([]models.DatabaseRoleActivity); ok {
				*v = mockResults
			}
		case *[]models.DatabaseRolePermissionMatrix:
			if mockResults, ok := results.([]models.DatabaseRolePermissionMatrix); ok {
				*v = mockResults
			}
		case *[]models.FailoverClusterReplicaMetrics:
			if mockResults, ok := results.([]models.FailoverClusterReplicaMetrics); ok {
				*v = mockResults
			}
		case *[]models.FailoverClusterReplicaStateMetrics:
			if mockResults, ok := results.([]models.FailoverClusterReplicaStateMetrics); ok {
				*v = mockResults
			}
		}
	}

	return nil
}

func (m *MockSQLConnectionInterface) Ping(ctx context.Context) error {
	return nil
}

func (m *MockSQLConnectionInterface) Close() error {
	return nil
}

// Helper function to create test time
func testTime() *time.Time {
	t := time.Date(2023, 10, 15, 10, 30, 0, 0, time.UTC)
	return &t
}

// Helper function to create int64 pointer
func testInt64Ptr(val int64) *int64 {
	return &val
}

func TestNewDatabasePrincipalsScraper(t *testing.T) {
	mockConn := &MockSQLConnectionInterface{}
	logger := zap.NewNop()
	engineEdition := queries.StandardSQLServerEngineEdition

	scraper := NewDatabasePrincipalsScraper(mockConn, logger, engineEdition)

	assert.NotNil(t, scraper)
	assert.Equal(t, mockConn, scraper.connection)
	assert.Equal(t, logger, scraper.logger)
	assert.Equal(t, engineEdition, scraper.engineEdition)
	assert.False(t, scraper.startTime.AsTime().IsZero())
}

func TestDatabasePrincipalsScraper_ProcessDatabasePrincipalsMetrics(t *testing.T) {
	tests := []struct {
		name            string
		result          models.DatabasePrincipalsMetrics
		expectedMetrics int
	}{
		{
			name: "Principal with create date",
			result: models.DatabasePrincipalsMetrics{
				PrincipalName: "TestUser",
				TypeDesc:      "SQL_USER",
				CreateDate:    testTime(),
				DatabaseName:  "TestDB",
			},
			expectedMetrics: 1,
		},
		{
			name: "Principal with nil create date",
			result: models.DatabasePrincipalsMetrics{
				PrincipalName: "TestUser",
				TypeDesc:      "SQL_USER",
				CreateDate:    nil,
				DatabaseName:  "TestDB",
			},
			expectedMetrics: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := NewDatabasePrincipalsScraper(nil, zap.NewNop(), queries.StandardSQLServerEngineEdition)

			metrics := pmetric.NewMetrics()
			scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

			err := scraper.processDatabasePrincipalsMetrics(tt.result, scopeMetrics)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedMetrics, scopeMetrics.Metrics().Len())
		})
	}
}

func TestDatabasePrincipalsScraper_ProcessDatabasePrincipalsSummaryMetrics(t *testing.T) {
	tests := []struct {
		name            string
		result          models.DatabasePrincipalsSummary
		expectedMetrics int
	}{
		{
			name: "Summary with all fields",
			result: models.DatabasePrincipalsSummary{
				DatabaseName:         "TestDB",
				TotalPrincipals:      testInt64Ptr(10),
				UserCount:            testInt64Ptr(6),
				RoleCount:            testInt64Ptr(4),
				SQLUserCount:         testInt64Ptr(3),
				WindowsUserCount:     testInt64Ptr(2),
				ApplicationRoleCount: testInt64Ptr(1),
			},
			expectedMetrics: 6,
		},
		{
			name: "Summary with some nil fields",
			result: models.DatabasePrincipalsSummary{
				DatabaseName:    "TestDB",
				TotalPrincipals: testInt64Ptr(5),
				UserCount:       nil,
				RoleCount:       testInt64Ptr(2),
			},
			expectedMetrics: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := NewDatabasePrincipalsScraper(nil, zap.NewNop(), queries.StandardSQLServerEngineEdition)

			metrics := pmetric.NewMetrics()
			scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

			err := scraper.processDatabasePrincipalsSummaryMetrics(tt.result, scopeMetrics)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedMetrics, scopeMetrics.Metrics().Len())
		})
	}
}

func TestDatabasePrincipalsScraper_getQueryForMetric(t *testing.T) {
	tests := []struct {
		name          string
		engineEdition int
		metricName    string
		wantFound     bool
	}{
		{
			name:          "Standard SQL Server principals query",
			engineEdition: queries.StandardSQLServerEngineEdition,
			metricName:    "database_principals",
			wantFound:     true,
		},
		{
			name:          "Azure SQL Database summary query",
			engineEdition: queries.AzureSQLDatabaseEngineEdition,
			metricName:    "database_principals_summary",
			wantFound:     true,
		},
		{
			name:          "Invalid metric name",
			engineEdition: queries.StandardSQLServerEngineEdition,
			metricName:    "invalid_metric",
			wantFound:     false,
		},
		{
			name:          "Invalid engine edition",
			engineEdition: 999,
			metricName:    "database_principals",
			wantFound:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := NewDatabasePrincipalsScraper(nil, zap.NewNop(), tt.engineEdition)

			_, found := scraper.getQueryForMetric(tt.metricName)

			assert.Equal(t, tt.wantFound, found)
		})
	}
}

func TestDatabasePrincipalsScraper_createTimestampMetric(t *testing.T) {
	scraper := NewDatabasePrincipalsScraper(nil, zap.NewNop(), queries.StandardSQLServerEngineEdition)

	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

	testTime := time.Date(2023, 10, 15, 10, 30, 0, 0, time.UTC)
	err := scraper.createTimestampMetric(testTime, "test.metric", "TestDB", "TestUser", "SQL_USER", scopeMetrics)

	require.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	assert.Equal(t, "test.metric", metric.Name())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, testTime.Unix(), dataPoint.IntValue())

	// Check attributes
	attrs := dataPoint.Attributes()
	dbName, exists := attrs.Get("database_name")
	assert.True(t, exists)
	assert.Equal(t, "TestDB", dbName.Str())

	principalName, exists := attrs.Get("principal_name")
	assert.True(t, exists)
	assert.Equal(t, "TestUser", principalName.Str())

	principalType, exists := attrs.Get("principal_type")
	assert.True(t, exists)
	assert.Equal(t, "SQL_USER", principalType.Str())
}

func TestDatabasePrincipalsScraper_createGaugeMetric(t *testing.T) {
	scraper := NewDatabasePrincipalsScraper(nil, zap.NewNop(), queries.StandardSQLServerEngineEdition)

	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

	// Test with non-nil int64 pointer
	value := int64(42)
	field := reflect.ValueOf(&value)

	err := scraper.createGaugeMetric(field, "test.gauge.metric", "TestDB", scopeMetrics)

	require.NoError(t, err)
	assert.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	assert.Equal(t, "test.gauge.metric", metric.Name())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, int64(42), dataPoint.IntValue())

	// Check database name attribute
	attrs := dataPoint.Attributes()
	dbName, exists := attrs.Get("database_name")
	assert.True(t, exists)
	assert.Equal(t, "TestDB", dbName.Str())
}

func TestDatabasePrincipalsScraper_ErrorCases(t *testing.T) {
	t.Run("Unsupported engine edition", func(t *testing.T) {
		scraper := NewDatabasePrincipalsScraper(nil, zap.NewNop(), 999)

		metrics := pmetric.NewMetrics()
		scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

		err := scraper.ScrapeDatabasePrincipalsMetrics(context.Background(), scopeMetrics)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database principals query available for engine edition 999")
	})

	t.Run("Query error", func(t *testing.T) {
		mockConn := &MockSQLConnectionInterface{
			queryError: assert.AnError,
		}

		scraper := NewDatabasePrincipalsScraper(mockConn, zap.NewNop(), queries.StandardSQLServerEngineEdition)

		metrics := pmetric.NewMetrics()
		scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

		err := scraper.ScrapeDatabasePrincipalsMetrics(context.Background(), scopeMetrics)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to query database principals")
	})
}
