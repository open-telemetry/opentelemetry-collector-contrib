// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

// Helper function to create int64 pointers for test data
func roleMembershipInt64Ptr(v int64) *int64 {
	return &v
}

// TestNewDatabaseRoleMembershipScraper tests the constructor for the database role membership scraper
func TestNewDatabaseRoleMembershipScraper(t *testing.T) {
	logger := zap.NewNop()
	mockConnection := &MockSQLConnectionInterface{}
	engineEdition := queries.StandardSQLServerEngineEdition

	scraper := NewDatabaseRoleMembershipScraper(logger, mockConnection, engineEdition)

	assert.NotNil(t, scraper)
	assert.Equal(t, logger, scraper.logger)
	assert.Equal(t, mockConnection, scraper.connection)
	assert.Equal(t, engineEdition, scraper.engineEdition)
	assert.NotZero(t, scraper.startTime)
}

// TestDatabaseRoleMembershipScraper_ScrapeDatabaseRoleMembershipMetrics tests the primary role membership metrics scraping
func TestDatabaseRoleMembershipScraper_ScrapeDatabaseRoleMembershipMetrics(t *testing.T) {
	tests := []struct {
		name            string
		mockResults     []models.DatabaseRoleMembershipMetrics
		mockError       error
		expectError     bool
		expectedMetrics int
	}{
		{
			name: "successful_role_membership_scraping",
			mockResults: []models.DatabaseRoleMembershipMetrics{
				{
					RoleName:         "db_datareader",
					MemberName:       "AppUser1",
					DatabaseName:     "TestDB",
					RoleType:         "DATABASE_ROLE",
					MemberType:       "SQL_USER",
					MembershipActive: roleMembershipInt64Ptr(1),
				},
				{
					RoleName:         "CustomRole",
					MemberName:       "AppUser2",
					DatabaseName:     "TestDB",
					RoleType:         "DATABASE_ROLE",
					MemberType:       "SQL_USER",
					MembershipActive: roleMembershipInt64Ptr(1),
				},
			},
			mockError:       nil,
			expectError:     false,
			expectedMetrics: 2,
		},
		{
			name:            "query_execution_error",
			mockResults:     nil,
			mockError:       errors.New("database connection failed"),
			expectError:     true,
			expectedMetrics: 0,
		},
		{
			name:            "empty_results",
			mockResults:     []models.DatabaseRoleMembershipMetrics{},
			mockError:       nil,
			expectError:     false,
			expectedMetrics: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConnection := &MockSQLConnectionInterface{
				queryResults: map[string]interface{}{
					queries.DatabaseRoleMembershipMetricsQuery: tt.mockResults,
				},
				queryError: tt.mockError,
			}

			logger := zap.NewNop()
			scraper := NewDatabaseRoleMembershipScraper(logger, mockConnection, queries.StandardSQLServerEngineEdition)

			metrics := pmetric.NewMetrics()
			scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

			err := scraper.ScrapeDatabaseRoleMembershipMetrics(context.Background(), scopeMetrics)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedMetrics, scopeMetrics.Metrics().Len())
		})
	}
}

// TestDatabaseRoleMembershipScraper_MetricNaming tests that metrics are created with correct names and attributes
func TestDatabaseRoleMembershipScraper_MetricNaming(t *testing.T) {
	mockResults := []models.DatabaseRoleMembershipMetrics{
		{
			RoleName:         "TestRole",
			MemberName:       "TestMember",
			DatabaseName:     "TestDB",
			RoleType:         "DATABASE_ROLE",
			MemberType:       "SQL_USER",
			MembershipActive: roleMembershipInt64Ptr(1),
		},
	}

	mockConnection := &MockSQLConnectionInterface{
		queryResults: map[string]interface{}{
			queries.DatabaseRoleMembershipMetricsQuery: mockResults,
		},
		queryError: nil,
	}

	logger := zap.NewNop()
	scraper := NewDatabaseRoleMembershipScraper(logger, mockConnection, queries.StandardSQLServerEngineEdition)

	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

	err := scraper.ScrapeDatabaseRoleMembershipMetrics(context.Background(), scopeMetrics)
	require.NoError(t, err)

	require.Equal(t, 1, scopeMetrics.Metrics().Len())

	metric := scopeMetrics.Metrics().At(0)
	assert.Equal(t, "sqlserver.database.role.membership.active", metric.Name())
	assert.Equal(t, "Database role membership active status", metric.Description())

	gauge := metric.Gauge()
	require.Equal(t, 1, gauge.DataPoints().Len())

	dataPoint := gauge.DataPoints().At(0)
	assert.Equal(t, int64(1), dataPoint.IntValue())

	// Check attributes
	attributes := dataPoint.Attributes()
	databaseName, found := attributes.Get("sqlserver.database.name")
	require.True(t, found)
	assert.Equal(t, "TestDB", databaseName.Str())

	roleName, found := attributes.Get("sqlserver.database.role.name")
	require.True(t, found)
	assert.Equal(t, "TestRole", roleName.Str())

	memberName, found := attributes.Get("sqlserver.database.role.member.name")
	require.True(t, found)
	assert.Equal(t, "TestMember", memberName.Str())

	roleType, found := attributes.Get("sqlserver.database.role.type")
	require.True(t, found)
	assert.Equal(t, "DATABASE_ROLE", roleType.Str())

	memberType, found := attributes.Get("sqlserver.database.role.member.type")
	require.True(t, found)
	assert.Equal(t, "SQL_USER", memberType.Str())
}
