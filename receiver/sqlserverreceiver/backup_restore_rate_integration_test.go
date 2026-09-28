// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package sqlserverreceiver

import (
	"database/sql"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// backupRateDataPoints collects every sqlserver.database.backup_or_restore.rate datapoint in a
// scrape, keyed by the database name carried on its resource.
func backupRateDataPoints(t *testing.T, m pmetric.Metrics) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		rm := m.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			for k := 0; k < rm.ScopeMetrics().At(j).Metrics().Len(); k++ {
				metric := rm.ScopeMetrics().At(j).Metrics().At(k)
				if metric.Name() != "sqlserver.database.backup_or_restore.rate" {
					continue
				}

				require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
				require.Equal(t, "By/s", metric.Unit())

				dbName, ok := rm.Resource().Attributes().Get("sqlserver.database.name")
				require.True(t, ok, "datapoints must carry sqlserver.database.name so they do not collide")
				require.NotEqual(t, "Total", dbName.Str(), "the Total aggregate row must not be emitted")

				dp := metric.Gauge().DataPoints().At(0)
				require.Equal(t, pmetric.NumberDataPointValueTypeDouble, dp.ValueType())

				_, dup := out[dbName.Str()]
				require.False(t, dup, "duplicate datapoint for database %s", dbName.Str())
				out[dbName.Str()] = dp.DoubleValue()
			}
		}
	}
	return out
}

// TestBackupRestoreRateScraper asserts, against a live SQL Server, that the backup/restore
// counter is turned into a genuine per-second byte rate: suppressed on the first scrape for lack
// of a baseline, non-zero while a backup is running, and back to zero once backups stop.
func TestBackupRestoreRateScraper(t *testing.T) {
	ci, err := setupContainer()
	require.NoError(t, err)
	require.NoError(t, ci.Start(t.Context()))
	defer testcontainers.CleanupContainer(t, ci)

	p, err := ci.MappedPort(t.Context(), "1433")
	require.NoError(t, err)
	portNumber, err := strconv.Atoi(p.Port())
	require.NoError(t, err)

	dsn := fmt.Sprintf("sqlserver://sa:%%5Eotelcol1234@localhost:%d?encrypt=disable", portNumber)
	db, err := sql.Open("sqlserver", dsn)
	require.NoError(t, err)
	defer db.Close()

	for _, stmt := range []string{
		"CREATE DATABASE bkprate",
		"CREATE TABLE bkprate.dbo.filler (id INT IDENTITY, pad CHAR(4000) NOT NULL)",
		"INSERT INTO bkprate.dbo.filler (pad) SELECT TOP 5000 REPLICATE('x',4000) FROM sys.all_columns a CROSS JOIN sys.all_columns b",
	} {
		_, err = db.Exec(stmt)
		require.NoError(t, err, stmt)
	}

	cfg := basicConfig(uint(portNumber))
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseBackupOrRestoreRate.Enabled = true

	settings := receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.Must(zap.NewProduction())},
	}

	scrapers, provider := setupSQLServerScrapers(settings, cfg)
	require.NotEmpty(t, scrapers)
	require.NotNil(t, provider)

	var perfScraper *sqlServerScraperHelper
	for _, s := range scrapers {
		if s.sqlQuery == getSQLServerPerformanceCounterQuery(cfg.InstanceName) {
			perfScraper = s
			break
		}
	}
	require.NotNil(t, perfScraper, "performance counter scraper not found")
	require.NoError(t, perfScraper.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, perfScraper.Shutdown(t.Context()))
		// The receiver owns the shared pool; close it once at the end.
		assert.NoError(t, provider.close())
	}()

	// First scrape establishes the baseline and must not emit a rate.
	first, err := perfScraper.ScrapeMetrics(t.Context())
	require.NoError(t, err)
	require.Empty(t, backupRateDataPoints(t, first),
		"no rate can be derived from a single sample, so nothing should be emitted")

	// Back up twice between scrapes, then confirm the rate is positive for that database only.
	for i := 1; i <= 2; i++ {
		_, err = db.Exec(fmt.Sprintf("BACKUP DATABASE bkprate TO DISK='/var/opt/mssql/data/r%d.bak' WITH INIT", i))
		require.NoError(t, err)
	}

	second, err := perfScraper.ScrapeMetrics(t.Context())
	require.NoError(t, err)
	active := backupRateDataPoints(t, second)
	require.NotEmpty(t, active, "a rate should be emitted once a baseline exists")
	t.Logf("during backups: %v", active)
	require.Positive(t, active["bkprate"], "the backed-up database should show a positive byte rate")
	for name, v := range active {
		if name != "bkprate" {
			require.Zero(t, v, "database %s had no backup activity and should report zero", name)
		}
	}

	// With no further backups the counter stops advancing, so the rate must fall back to zero.
	third, err := perfScraper.ScrapeMetrics(t.Context())
	require.NoError(t, err)
	idle := backupRateDataPoints(t, third)
	require.NotEmpty(t, idle)
	t.Logf("after backups stopped: %v", idle)
	require.Zero(t, idle["bkprate"], "rate must return to zero once backups stop, unlike the raw counter")
}
