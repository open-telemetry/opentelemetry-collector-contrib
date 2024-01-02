// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snowflakereceiver

import (
	"context"
	"database/sql/driver"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestScraper(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Account = "account"
	cfg.Username = "uname"
	cfg.Password = "pwd"
	cfg.Warehouse = "warehouse"
	err := component.ValidateConfig(cfg)
	if err != nil {
		t.Fatal("an error ocured when validating config", err)
	}

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal("an error ocured when opening mock db", err)
	}
	defer db.Close()

	mockDB := mockDB{mock}
	mockDB.initMockDB()

	scraper := newSnowflakeMetricsScraper(receivertest.NewNopCreateSettings(), cfg)

	// by default our scraper does not start with a client. the client we use must contain
	// the mock database
	scraperClient := snowflakeClient{
		client: db,
		logger: receivertest.NewNopCreateSettings().Logger,
	}
	scraper.client = &scraperClient

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err, "error scraping metrics from mocdb")

	expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")

	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err, "error reading expected metrics")

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp()))
}

func TestStart(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Account = "account"
	cfg.Username = "uname"
	cfg.Password = "pwd"
	cfg.Warehouse = "warehouse"
	require.NoError(t, component.ValidateConfig(cfg))

	scraper := newSnowflakeMetricsScraper(receivertest.NewNopCreateSettings(), cfg)
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "Problem starting scraper")
}

// wrapper type for convenience
type mockDB struct {
	mock sqlmock.Sqlmock
}

func (m *mockDB) initMockDB() {
	testDB := []struct {
		query   string
		columns []string
		params  []driver.Value
	}{
		{
			query:   billingMetricsQuery,
			columns: []string{"service_type", "service_name", "virtualwarehouse_credit", "cloud_service", "totalcredit"},
			params:  []driver.Value{"t", "n", 1.0, 2.0, 3.0},
		},
		{
			query:   warehouseBillingMetricsQuery,
			columns: []string{"wh_name", "virtual_wh", "cloud_service", "credit"},
			params:  []driver.Value{"n", 1.0, 2.0, 3.0},
		},
		{
			query:   loginMetricsQuery,
			columns: []string{"username", "error_message", "client_type", "is_success", "login_total"},
			params:  []driver.Value{"t", "n", "m", "l", 1},
		},
		{
			query:   highLevelQueryMetricsQuery,
			columns: []string{"wh_name", "query_executed", "queue_overload", "queue_provision", "query_blocked"},
			params:  []driver.Value{"t", 0.0, 1.0, 2.0, 3.0},
		},
		{
			query: dbMetricsQuery,
			columns: []string{"schemaname", "execution_status", "error_message",
				"query_type", "wh_name", "db_name", "wh_size", "username",
				"count_queryid", "queued_overload", "queued_repair", "queued_provision",
				"total_elapsed", "execution_time", "comp_time", "bytes_scanned",
				"bytes_written", "bytes_deleted", "bytes_spilled_local", "bytes_spilled_remote",
				"percentage_cache", "partitions_scanned", "rows_unloaded", "rows_deleted",
				"rows_updated", "rows_inserted", "rows_produced"},
			params: []driver.Value{"a", "b", "c", "d", "e", "f", "g", "h", 1, 2.0, 3.0, 4.0, 5.0, 6.0,
				7.0, 8.0, 9.0, 10.0, 11.0, 12.0, 13.0, 14.0, 15.0, 16.0, 17.0, 18.0, 19.0},
		},
		{
			query:   sessionMetricsQuery,
			columns: []string{"username", "disctinct_id"},
			params:  []driver.Value{"t", 3.0},
		},
		{
			query:   snowpipeMetricsQuery,
			columns: []string{"pipe_name", "credits_used", "bytes_inserted", "files_inserted"},
			params:  []driver.Value{"t", 1.0, 2.0, 3.0},
		},
		{
			query:   storageMetricsQuery,
			columns: []string{"storage_bytes", "stage_bytes", "failsafe_bytes"},
			params:  []driver.Value{1, 2, 3},
		},
	}

	m.mock.MatchExpectationsInOrder(false)
	for _, test := range testDB {
		rows := m.mock.NewRows(test.columns).AddRow(test.params...)
		m.mock.ExpectQuery(test.query).WillReturnRows(rows)
	}
}
