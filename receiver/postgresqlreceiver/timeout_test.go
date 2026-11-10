// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

func newTimeoutTestScraper(t *testing.T, cfg *Config) (*postgreSQLScraper, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	})
	// Disable cache expiration: its permanent cleanup goroutine cannot outlive
	// a synctest bubble, and cache expiration is unrelated to scrape timeouts.
	s, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg,
		mockSimpleClientFactory{db: db}, newCache(1), newTTLCache[string](1, 0))
	require.NoError(t, err)
	return s, mock
}

func TestScrapeTimeoutUnblocksShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.ControllerConfig.InitialDelay = 0
		cfg.MetricsBuilderConfig.Metrics = metadata.MetricsConfig{}
		cfg.MetricsBuilderConfig.Metrics.PostgresqlDatabaseCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.PostgresqlConnectionMax.Enabled = true
		s, mock := newTimeoutTestScraper(t, cfg)
		mock.ExpectQuery("SELECT datname FROM pg_database").
			WillReturnRows(sqlmock.NewRows([]string{"datname"}))
		// Keep the mock finite so the test can clean up if the timeout regresses.
		mock.ExpectQuery("SHOW max_connections").WillDelayFor(2 * time.Minute).
			WillReturnRows(sqlmock.NewRows([]string{"max_connections"}).AddRow(100))

		type scrapeResult struct {
			err        error
			contextErr error
		}
		results := make(chan scrapeResult, 1)
		sc, err := scraper.NewMetrics(func(ctx context.Context) (pmetric.Metrics, error) {
			md, scrapeErr := s.scrape(ctx)
			results <- scrapeResult{err: scrapeErr, contextErr: ctx.Err()}
			return md, scrapeErr
		}, scraper.WithStart(s.start), scraper.WithShutdown(s.shutdown))
		require.NoError(t, err)
		sink := new(consumertest.MetricsSink)
		ctrl, err := scraperhelper.NewMetricsController(&cfg.ControllerConfig,
			receivertest.NewNopSettings(metadata.Type), sink,
			scraperhelper.AddMetricsScraper(metadata.Type, sc),
			scraperhelper.WithTickerChannel(make(chan time.Time)))
		require.NoError(t, err)
		require.NoError(t, ctrl.Start(t.Context(), componenttest.NewNopHost()))
		synctest.Wait()
		require.Empty(t, results, "the query must still be blocked")

		stopped := make(chan error, 1)
		go func() { stopped <- ctrl.Shutdown(t.Context()) }()
		defer func() {
			if len(stopped) == 0 {
				time.Sleep(time.Minute)
				synctest.Wait()
			}
			<-stopped
		}()
		synctest.Wait()
		require.Empty(t, stopped, "shutdown must wait for the active scrape")

		time.Sleep(time.Minute)
		synctest.Wait()
		require.Len(t, results, 1, "the default timeout must cancel the blocked query")
		result := <-results
		require.ErrorIs(t, result.contextErr, context.DeadlineExceeded)
		require.ErrorContains(t, result.err, sqlmock.ErrCancelled.Error())
		require.True(t, scrapererror.IsPartialScrapeError(result.err))
		require.Equal(t, 1, sink.DataPointCount(), "partial metrics must reach the consumer")
		require.Len(t, stopped, 1, "shutdown must finish after query cancellation")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
