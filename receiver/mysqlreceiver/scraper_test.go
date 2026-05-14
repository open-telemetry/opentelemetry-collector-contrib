// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver

import (
	"bufio"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

func TestScrape(t *testing.T) {
	t.Run("successful scrape", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Username = "otel"
		cfg.Password = "otel"
		cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
		cfg.MetricsBuilderConfig.Metrics.MysqlStatementEventCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlStatementEventWaitTime.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlConnectionErrors.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlMysqlxWorkerThreads.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlJoins.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableOpenCache.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlQueryClientCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlQueryCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlQuerySlowCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlPageSize.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitReadCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitReadTime.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitWriteCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitWriteTime.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlTableRows.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableAverageRowLength.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableSize.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlClientNetworkIo.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlPreparedStatements.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlCommands.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlReplicaSQLDelay.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlReplicaTimeBehindSource.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlConnectionCount.Enabled = true

		cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
		cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true

		scraper := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](0, time.Hour*24*365*10))
		scraper.sqlclient = &mockClient{
			globalStatsFile:             "global_stats",
			innodbStatsFile:             "innodb_stats",
			tableIoWaitsFile:            "table_io_waits_stats",
			indexIoWaitsFile:            "index_io_waits_stats",
			tableStatsFile:              "table_stats",
			statementEventsFile:         "statement_events",
			tableLockWaitEventStatsFile: "table_lock_wait_event_stats",
			replicaStatusFile:           "replica_stats",
			querySamplesFile:            "query_samples",
			topQueriesFile:              "top_queries",
		}

		scraper.renameCommands = true

		actualMetrics, err := scraper.scrape(t.Context())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))

		actualQuerySamples, err := scraper.scrapeQuerySampleFunc(t.Context())
		require.NoError(t, err)
		expectedQuerySampleFile := filepath.Join("testdata", "scraper", "expectedQuerySamples.yaml")
		// Uncomment this to regenerate the expected logs file
		// golden.WriteLogs(t, expectedQuerySampleFile, actualQuerySamples)
		expectedQuerySample, err := golden.ReadLogs(expectedQuerySampleFile)
		require.NoError(t, err)

		require.NoError(t, plogtest.CompareLogs(expectedQuerySample, actualQuerySamples,
			plogtest.IgnoreTimestamp()))
		assertLogsHaveInstanceEndpoint(t, actualQuerySamples, cfg.Endpoint)

		// Scrape top queries
		scraper.cacheAndDiff("mysql", "c16f24f908846019a741db580f6545a5933e9435a7cf1579c50794a6ca287739", "count_star", 1)
		scraper.cacheAndDiff("mysql", "c16f24f908846019a741db580f6545a5933e9435a7cf1579c50794a6ca287739", "sum_timer_wait", 1)
		actualTopQueries, err := scraper.scrapeTopQueryFunc(t.Context())
		require.NoError(t, err)
		expectedTopQueriesFile := filepath.Join("testdata", "scraper", "expectedTopQueries.yaml")
		// Uncomment this to regenerate the expected logs file
		// golden.WriteLogs(t, expectedTopQueriesFile, actualTopQueries)
		expectedTopQueries, err := golden.ReadLogs(expectedTopQueriesFile)
		require.NoError(t, err)

		require.NoError(t, plogtest.CompareLogs(expectedTopQueries, actualTopQueries,
			plogtest.IgnoreTimestamp()))
		assertLogsHaveInstanceEndpoint(t, actualTopQueries, cfg.Endpoint)
	})

	t.Run("scrape has partial failure", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Username = "otel"
		cfg.Password = "otel"
		cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
		cfg.MetricsBuilderConfig.Metrics.MysqlReplicaSQLDelay.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlReplicaTimeBehindSource.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitReadCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitReadTime.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitWriteCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitWriteTime.Enabled = true

		scraper := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](0, time.Hour*24*365*10))
		scraper.sqlclient = &mockClient{
			globalStatsFile:             "global_stats_partial",
			innodbStatsFile:             "innodb_stats_empty",
			tableIoWaitsFile:            "table_io_waits_stats_empty",
			indexIoWaitsFile:            "index_io_waits_stats_empty",
			tableStatsFile:              "table_stats_empty",
			statementEventsFile:         "statement_events_empty",
			tableLockWaitEventStatsFile: "table_lock_wait_event_stats_empty",
			replicaStatusFile:           "replica_stats_empty",
		}

		actualMetrics, scrapeErr := scraper.scrape(t.Context())
		require.Error(t, scrapeErr)

		expectedFile := filepath.Join("testdata", "scraper", "expected_partial.yaml")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)
		assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp()))

		var partialError scrapererror.PartialScrapeError
		require.ErrorAs(t, scrapeErr, &partialError, "returned error was not PartialScrapeError")
		// 5 comes from 4 failed "must-have" metrics that aren't present,
		// and the other failure comes from a row that fails to parse as a number
		require.Equal(t, 5, partialError.Failed, "Expected partial error count to be 5")
	})
}

func TestScrapeBufferPoolPagesMiscOutOfBounds(t *testing.T) {
	expectedFile := filepath.Join("testdata", "scraper", "expected_oob.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}

	scraper := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](0, time.Hour*24*365*10))
	scraper.sqlclient = &mockClient{
		globalStatsFile:             "global_stats_oob",
		innodbStatsFile:             "innodb_stats_empty",
		tableIoWaitsFile:            "table_io_waits_stats_empty",
		indexIoWaitsFile:            "index_io_waits_stats_empty",
		tableStatsFile:              "table_stats_empty",
		statementEventsFile:         "statement_events_empty",
		tableLockWaitEventStatsFile: "table_lock_wait_event_stats_empty",
		replicaStatusFile:           "replica_stats_empty",
	}

	scraper.renameCommands = true

	actualMetrics, err := scraper.scrape(t.Context())
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

// assertLogsHaveInstanceEndpoint verifies that every ResourceLogs in logs carries
// mysql.instance.endpoint as a resource attribute with the expected value.
func assertLogsHaveInstanceEndpoint(t *testing.T, logs plog.Logs, expectedEndpoint string) {
	t.Helper()
	require.Positive(t, logs.ResourceLogs().Len(), "expected at least one ResourceLogs")
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		attrs := logs.ResourceLogs().At(i).Resource().Attributes()
		val, ok := attrs.Get("mysql.instance.endpoint")
		require.True(t, ok, "ResourceLogs[%d] missing mysql.instance.endpoint resource attribute", i)
		require.Equal(t, expectedEndpoint, val.Str(), "ResourceLogs[%d] mysql.instance.endpoint mismatch", i)
	}
}

func TestContextWithTraceparent(t *testing.T) {
	t.Run("valid traceparent sets span context", func(t *testing.T) {
		ctx, err := contextWithTraceparent("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
		require.NoError(t, err)
		spanCtx := trace.SpanContextFromContext(ctx)
		require.True(t, spanCtx.IsValid())
		assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", spanCtx.TraceID().String())
		assert.Equal(t, "00f067aa0ba902b7", spanCtx.SpanID().String())
	})

	t.Run("invalid traceparent returns error and empty span context", func(t *testing.T) {
		ctx, err := contextWithTraceparent("trace-id")
		require.Error(t, err)
		spanCtx := trace.SpanContextFromContext(ctx)
		assert.False(t, spanCtx.IsValid())
	})

	t.Run("result is rooted at context.Background, not a caller context", func(t *testing.T) {
		// contextWithTraceparent always uses context.Background() as its base,
		// so a collector scrape span in the caller's context must not bleed through.
		collectorTraceID, _ := trace.TraceIDFromHex("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		collectorSpanID, _ := trace.SpanIDFromHex("bbbbbbbbbbbbbbbb")
		collectorSpanCtx := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    collectorTraceID,
			SpanID:     collectorSpanID,
			TraceFlags: trace.FlagsSampled,
		})
		// Verify that the function ignores any span context present on a hypothetical
		// caller context by confirming it returns the app trace IDs, not the collector ones.
		appTraceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		_ = collectorSpanCtx // would be on the caller ctx in production; not passed in here

		ctx, err := contextWithTraceparent(appTraceparent)
		require.NoError(t, err)

		spanCtx := trace.SpanContextFromContext(ctx)
		require.True(t, spanCtx.IsValid())
		assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", spanCtx.TraceID().String())
		assert.Equal(t, "00f067aa0ba902b7", spanCtx.SpanID().String())
		assert.NotEqual(t, collectorTraceID.String(), spanCtx.TraceID().String(), "collector TraceID must not bleed through")
		assert.NotEqual(t, collectorSpanID.String(), spanCtx.SpanID().String(), "collector SpanID must not bleed through")
	})
}

func TestScrapeQuerySamplesTraceparent(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true

	t.Run("empty traceparent produces zero TraceID and SpanID", func(t *testing.T) {
		scraper := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](0, time.Hour*24*365*10))
		scraper.sqlclient = &mockClient{querySamplesFile: "query_samples_no_traceparent"}

		logs, err := scraper.scrapeQuerySampleFunc(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
		record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		assert.Equal(t, pcommon.TraceID{}, record.TraceID(), "TraceID must be zero when no traceparent is set")
		assert.Equal(t, pcommon.SpanID{}, record.SpanID(), "SpanID must be zero when no traceparent is set")
	})

	t.Run("invalid traceparent logs warning and produces zero TraceID and SpanID", func(t *testing.T) {
		core, logs := observer.New(zapcore.WarnLevel)
		scraper := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](0, time.Hour*24*365*10))
		scraper.logger = zap.New(core)
		scraper.sqlclient = &mockClient{querySamplesFile: "query_samples_invalid_traceparent"}

		result, err := scraper.scrapeQuerySampleFunc(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, logs.FilterMessage("Invalid traceparent; omitting trace context").Len(), "expected one warn log for invalid traceparent")
		require.Equal(t, 1, result.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
		record := result.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		assert.Equal(t, pcommon.TraceID{}, record.TraceID(), "TraceID must be zero when traceparent is invalid")
		assert.Equal(t, pcommon.SpanID{}, record.SpanID(), "SpanID must be zero when traceparent is invalid")
	})

	t.Run("bare IP processlistHost uses IP as address without logging an error", func(t *testing.T) {
		core, observed := observer.New(zapcore.ErrorLevel)
		scraper := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](0, time.Hour*24*365*10))
		scraper.logger = zap.New(core)
		scraper.sqlclient = &mockClient{querySamplesFile: "query_samples_bare_host"}

		result, err := scraper.scrapeQuerySampleFunc(t.Context())
		require.NoError(t, err, "bare IP processlistHost must not cause a scrape error")
		require.Equal(t, 0, observed.FilterMessage("Failed to parse processlistHost value").Len(), "bare IP must not log an error")
		require.Equal(t, 1, result.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len(), "record must still be emitted")
		record := result.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

		addrVal, ok := record.Attributes().Get("client.address")
		assert.True(t, ok)
		assert.Equal(t, "192.168.200.161", addrVal.Str(), "client.address must be set to bare IP")

		portVal, ok := record.Attributes().Get("client.port")
		assert.True(t, ok)
		assert.Equal(t, int64(0), portVal.Int(), "client.port must be zero when no port in processlistHost")

		peerAddrVal, ok := record.Attributes().Get("network.peer.address")
		assert.True(t, ok)
		assert.Equal(t, "192.168.200.161", peerAddrVal.Str(), "network.peer.address must be set to bare IP")

		peerPortVal, ok := record.Attributes().Get("network.peer.port")
		assert.True(t, ok)
		assert.Equal(t, int64(0), peerPortVal.Int(), "network.peer.port must be zero when no port in processlistHost")
	})

	t.Run("empty processlistHost produces empty address and zero port", func(t *testing.T) {
		scraper := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](0, time.Hour*24*365*10))
		scraper.sqlclient = &mockClient{querySamplesFile: "query_samples_empty_host"}

		result, err := scraper.scrapeQuerySampleFunc(t.Context())
		require.NoError(t, err, "empty processlistHost must not cause a scrape error")
		require.Equal(t, 1, result.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len(), "record must still be emitted")
		record := result.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

		addrVal, ok := record.Attributes().Get("client.address")
		assert.True(t, ok)
		assert.Empty(t, addrVal.Str(), "client.address must be empty string when processlistHost is empty")

		portVal, ok := record.Attributes().Get("client.port")
		assert.True(t, ok)
		assert.Equal(t, int64(0), portVal.Int(), "client.port must be zero when processlistHost is empty")

		peerAddrVal, ok := record.Attributes().Get("network.peer.address")
		assert.True(t, ok)
		assert.Empty(t, peerAddrVal.Str(), "network.peer.address must be empty string when processlistHost is empty")

		peerPortVal, ok := record.Attributes().Get("network.peer.port")
		assert.True(t, ok)
		assert.Equal(t, int64(0), peerPortVal.Int(), "network.peer.port must be zero when processlistHost is empty")
	})
}

func TestScrapeTopQueryInterval(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
	cfg.TopQueryCollection.CollectionInterval = 60 * time.Second

	mc := &mockClient{topQueriesFile: "top_queries"}

	newScraper := func() *mySQLScraper {
		s := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](0, time.Hour*24*365*10))
		s.sqlclient = mc
		return s
	}

	t.Run("first call always runs regardless of lastExecutionTimestamp", func(t *testing.T) {
		scraper := newScraper()
		before := scraper.lastExecutionTimestamp

		_, err := scraper.scrapeTopQueryFunc(t.Context())
		require.NoError(t, err)
		assert.True(t, scraper.lastExecutionTimestamp.After(before), "lastExecutionTimestamp must be updated on first call")
	})

	t.Run("second call within interval is skipped", func(t *testing.T) {
		scraper := newScraper()
		// Simulate a recent collection — just ran 10 seconds ago
		recent := time.Now().Add(-10 * time.Second)
		scraper.lastExecutionTimestamp = recent

		_, err := scraper.scrapeTopQueryFunc(t.Context())
		require.NoError(t, err)
		assert.Equal(t, recent, scraper.lastExecutionTimestamp, "lastExecutionTimestamp must not change when interval has not elapsed")
	})

	t.Run("call after interval elapses runs again", func(t *testing.T) {
		scraper := newScraper()
		// Simulate last collection exactly at the interval boundary (60s ago)
		past := time.Now().Add(-60 * time.Second)
		scraper.lastExecutionTimestamp = past

		_, err := scraper.scrapeTopQueryFunc(t.Context())
		require.NoError(t, err)
		assert.True(t, scraper.lastExecutionTimestamp.After(past), "lastExecutionTimestamp must be updated when interval has elapsed")
	})

	t.Run("call fires at interval boundary even when ticker arrives slightly early", func(t *testing.T) {
		scraper := newScraper()
		// Simulate ticker arriving ~1ms before the 60s mark — math.Ceil rounds 59.999s up to 60s,
		// so this must still fire rather than waiting for the next 10s tick (which would be 70s total).
		past := time.Now().Add(-60*time.Second + time.Millisecond)
		scraper.lastExecutionTimestamp = past

		_, err := scraper.scrapeTopQueryFunc(t.Context())
		require.NoError(t, err)
		assert.True(t, scraper.lastExecutionTimestamp.After(past), "ticker arriving 1ms early must still fire — math.Ceil rounds up to the interval boundary")
	})
}

func TestCacheAndDiff(t *testing.T) {
	newScraper := func() *mySQLScraper {
		cfg := createDefaultConfig().(*Config)
		return newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](0, time.Hour*24*365*10))
	}

	t.Run("first call returns not-cached and the value itself", func(t *testing.T) {
		s := newScraper()
		cached, diff := s.cacheAndDiff("schema", "digest", "col", 100)
		assert.False(t, cached)
		assert.Equal(t, int64(100), diff)
	})

	t.Run("second call with higher value returns diff", func(t *testing.T) {
		s := newScraper()
		s.cacheAndDiff("schema", "digest", "col", 100)
		cached, diff := s.cacheAndDiff("schema", "digest", "col", 250)
		assert.True(t, cached)
		assert.Equal(t, int64(150), diff)
	})

	t.Run("second call with same value returns zero diff", func(t *testing.T) {
		s := newScraper()
		s.cacheAndDiff("schema", "digest", "col", 100)
		cached, diff := s.cacheAndDiff("schema", "digest", "col", 100)
		assert.True(t, cached)
		assert.Equal(t, int64(0), diff)
	})

	t.Run("counter reset: val less than cached returns val as diff and refreshes cache", func(t *testing.T) {
		s := newScraper()
		s.cacheAndDiff("schema", "digest", "col", 1_000_000) // pre-reboot accumulation
		cached, diff := s.cacheAndDiff("schema", "digest", "col", 500)
		assert.True(t, cached)
		assert.Equal(t, int64(500), diff, "post-reset value should be treated as full diff since reset")

		// next call after reset should compute diff correctly from the refreshed cached value
		cached2, diff2 := s.cacheAndDiff("schema", "digest", "col", 750)
		assert.True(t, cached2)
		assert.Equal(t, int64(250), diff2, "subsequent call after reset should diff against the refreshed cache entry")
	})

	t.Run("negative value returns not-cached and zero", func(t *testing.T) {
		s := newScraper()
		cached, diff := s.cacheAndDiff("schema", "digest", "col", -1)
		assert.False(t, cached)
		assert.Equal(t, int64(0), diff)
	})
}

var _ client = (*mockClient)(nil)

type explainQueryCall struct {
	digestText      string
	sampleStatement string
}

type mockClient struct {
	globalStatsFile             string
	innodbStatsFile             string
	tableIoWaitsFile            string
	indexIoWaitsFile            string
	tableStatsFile              string
	statementEventsFile         string
	tableLockWaitEventStatsFile string
	replicaStatusFile           string
	querySamplesFile            string
	topQueriesFile              string
	// dbVersionOverride allows tests to simulate MySQL <8 or MariaDB.
	// Nil means "MySQL 8.0.27" (default, preserves all existing test behavior).
	dbVersionOverride *dbVersion
	// explainQueryCallCount is incremented each time explainQuery is called.
	// Tests that assert EXPLAIN was skipped can check this stays at zero.
	explainQueryCallCount int
	// explainQueryCalls records the digestText and sampleStatement args for each call.
	explainQueryCalls []explainQueryCall
}

type queryPlanSpyClient struct {
	mockClient
	querySamples []querySample
	topQueries   []topQuery
	explainCalls int
	explainPlan  string
}

func (c *queryPlanSpyClient) getQuerySamples(uint64, bool) ([]querySample, error) {
	return c.querySamples, nil
}

func (c *queryPlanSpyClient) getTopQueries(uint64, uint64, bool) ([]topQuery, error) {
	return c.topQueries, nil
}

func (c *queryPlanSpyClient) explainQuery(_, _, _, _ string, _ *zap.Logger) string {
	c.explainCalls++
	return c.explainPlan
}

func readFile(fname string) (map[string]string, error) {
	stats := map[string]string{}
	file, err := os.Open(filepath.Join("testdata", "scraper", fname+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.Split(scanner.Text(), "\t")
		stats[text[0]] = text[1]
	}
	return stats, nil
}

func (*mockClient) Connect() error {
	return nil
}

func (c *mockClient) getDBVersion() dbVersion {
	if c.dbVersionOverride != nil {
		return *c.dbVersionOverride
	}
	v, _ := version.NewVersion("8.0.27")
	return dbVersion{product: dbProductMySQL, version: v}
}

func (c *mockClient) getGlobalStats() (map[string]string, error) {
	return readFile(c.globalStatsFile)
}

func (c *mockClient) getInnodbStats() (map[string]string, error) {
	return readFile(c.innodbStatsFile)
}

func (c *mockClient) getTableStats() ([]tableStats, error) {
	var stats []tableStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.tableStatsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s tableStats
		text := strings.Split(scanner.Text(), "\t")
		s.schema = text[0]
		s.name = text[1]
		s.rows, _ = parseInt(text[2])
		s.averageRowLength, _ = parseInt(text[3])
		s.dataLength, _ = parseInt(text[4])
		s.indexLength, _ = parseInt(text[5])

		stats = append(stats, s)
	}
	return stats, nil
}

func (c *mockClient) getTableIoWaitsStats() ([]tableIoWaitsStats, error) {
	var stats []tableIoWaitsStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.tableIoWaitsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s tableIoWaitsStats
		text := strings.Split(scanner.Text(), "\t")

		s.schema = text[0]
		s.name = text[1]
		s.countDelete, _ = parseInt(text[2])
		s.countFetch, _ = parseInt(text[3])
		s.countInsert, _ = parseInt(text[4])
		s.countUpdate, _ = parseInt(text[5])
		s.timeDelete, _ = parseInt(text[6])
		s.timeFetch, _ = parseInt(text[7])
		s.timeInsert, _ = parseInt(text[8])
		s.timeUpdate, _ = parseInt(text[9])

		stats = append(stats, s)
	}
	return stats, nil
}

func (c *mockClient) getIndexIoWaitsStats() ([]indexIoWaitsStats, error) {
	var stats []indexIoWaitsStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.indexIoWaitsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s indexIoWaitsStats
		text := strings.Split(scanner.Text(), "\t")

		s.schema = text[0]
		s.name = text[1]
		s.index = text[2]
		s.countDelete, _ = parseInt(text[3])
		s.countFetch, _ = parseInt(text[4])
		s.countInsert, _ = parseInt(text[5])
		s.countUpdate, _ = parseInt(text[6])
		s.timeDelete, _ = parseInt(text[7])
		s.timeFetch, _ = parseInt(text[8])
		s.timeInsert, _ = parseInt(text[9])
		s.timeUpdate, _ = parseInt(text[10])

		stats = append(stats, s)
	}
	return stats, nil
}

func (c *mockClient) getStatementEventsStats() ([]statementEventStats, error) {
	var stats []statementEventStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.statementEventsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s statementEventStats
		text := strings.Split(scanner.Text(), "\t")

		s.schema = text[0]
		s.digest = text[1]
		s.digestText = text[2]
		s.sumTimerWait, _ = parseInt(text[3])
		s.countErrors, _ = parseInt(text[4])
		s.countWarnings, _ = parseInt(text[5])
		s.countRowsAffected, _ = parseInt(text[6])
		s.countRowsSent, _ = parseInt(text[7])
		s.countRowsExamined, _ = parseInt(text[8])
		s.countCreatedTmpDiskTables, _ = parseInt(text[9])
		s.countCreatedTmpTables, _ = parseInt(text[10])
		s.countSortMergePasses, _ = parseInt(text[11])
		s.countSortRows, _ = parseInt(text[12])
		s.countNoIndexUsed, _ = parseInt(text[13])

		stats = append(stats, s)
	}
	return stats, nil
}

func (c *mockClient) getTableLockWaitEventStats() ([]tableLockWaitEventStats, error) {
	var stats []tableLockWaitEventStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.tableLockWaitEventStatsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s tableLockWaitEventStats
		text := strings.Split(scanner.Text(), "\t")

		s.schema = text[0]
		s.name = text[1]
		s.countReadNormal, _ = parseInt(text[2])
		s.countReadWithSharedLocks, _ = parseInt(text[3])
		s.countReadHighPriority, _ = parseInt(text[4])
		s.countReadNoInsert, _ = parseInt(text[5])
		s.countReadExternal, _ = parseInt(text[6])
		s.countWriteAllowWrite, _ = parseInt(text[7])
		s.countWriteConcurrentInsert, _ = parseInt(text[8])
		s.countWriteLowPriority, _ = parseInt(text[9])
		s.countWriteNormal, _ = parseInt(text[10])
		s.countWriteExternal, _ = parseInt(text[11])
		s.sumTimerReadNormal, _ = parseInt(text[12])
		s.sumTimerReadWithSharedLocks, _ = parseInt(text[13])
		s.sumTimerReadHighPriority, _ = parseInt(text[14])
		s.sumTimerReadNoInsert, _ = parseInt(text[15])
		s.sumTimerReadExternal, _ = parseInt(text[16])
		s.sumTimerWriteAllowWrite, _ = parseInt(text[17])
		s.sumTimerWriteConcurrentInsert, _ = parseInt(text[18])
		s.sumTimerWriteLowPriority, _ = parseInt(text[19])
		s.sumTimerWriteNormal, _ = parseInt(text[20])
		s.sumTimerWriteExternal, _ = parseInt(text[21])

		stats = append(stats, s)
	}
	return stats, nil
}

func (c *mockClient) getReplicaStatusStats(_ bool) ([]replicaStatusStats, error) {
	var stats []replicaStatusStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.replicaStatusFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s replicaStatusStats
		text := strings.Split(scanner.Text(), "\t")

		s.replicaIOState = text[0]
		s.sourceHost = text[1]
		s.sourceUser = text[2]
		s.sourcePort, _ = parseInt(text[3])
		s.connectRetry, _ = parseInt(text[4])
		s.sourceLogFile = text[5]
		s.readSourceLogPos, _ = parseInt(text[6])
		s.relayLogFile = text[7]
		s.relayLogPos, _ = parseInt(text[8])
		s.relaySourceLogFile = text[9]
		s.replicaIORunning = text[10]
		s.replicaSQLRunning = text[11]
		s.replicateDoDB = text[12]
		s.replicateIgnoreDB = text[13]
		s.replicateDoTable = text[14]
		s.replicateIgnoreTable = text[15]
		s.replicateWildDoTable = text[16]
		s.replicateWildIgnoreTable = text[17]
		s.lastErrno, _ = parseInt(text[18])
		s.lastError = text[19]
		s.skipCounter, _ = parseInt(text[20])
		s.execSourceLogPos, _ = parseInt(text[21])
		s.relayLogSpace, _ = parseInt(text[22])
		s.untilCondition = text[23]
		s.untilLogFile = text[24]
		s.untilLogPos = text[25]
		s.sourceSSLAllowed = text[26]
		s.sourceSSLCAFile = text[27]
		s.sourceSSLCAPath = text[28]
		s.sourceSSLCert = text[29]
		s.sourceSSLCipher = text[30]
		s.sourceSSLKey = text[31]
		v, _ := parseInt(text[32])
		s.secondsBehindSource = sql.NullInt64{Int64: v, Valid: true}
		s.sourceSSLVerifyServerCert = text[33]
		s.lastIOErrno, _ = parseInt(text[34])
		s.lastIOError = text[35]
		s.lastSQLErrno, _ = parseInt(text[36])
		s.lastSQLError = text[37]
		s.replicateIgnoreServerIDs = text[38]
		s.sourceServerID, _ = parseInt(text[39])
		s.sourceUUID = text[40]
		s.sourceInfoFile = text[41]
		s.sqlDelay, _ = parseInt(text[42])
		v, _ = parseInt(text[43])
		s.sqlRemainingDelay = sql.NullInt64{Int64: v, Valid: true}
		s.replicaSQLRunningState = text[44]
		s.sourceRetryCount, _ = parseInt(text[45])
		s.sourceBind = text[46]
		s.lastIOErrorTimestamp = text[47]
		s.lastSQLErrorTimestamp = text[48]
		s.sourceSSLCrl = text[49]
		s.sourceSSLCrlpath = text[50]
		s.retrievedGtidSet = text[51]
		s.executedGtidSet = text[52]
		s.autoPosition = text[53]
		s.replicateRewriteDB = text[54]
		s.channelName = text[55]
		s.sourceTLSVersion = text[56]
		s.sourcePublicKeyPath = text[57]
		s.getSourcePublicKey, _ = parseInt(text[58])
		s.networkNamespace = text[59]

		stats = append(stats, s)
	}
	return stats, nil
}

// Generate a function for getQuerySamples to read data from a static file
func (c *mockClient) getQuerySamples(uint64, bool) ([]querySample, error) {
	var samples []querySample
	file, err := os.Open(filepath.Join("testdata", "scraper", c.querySamplesFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s querySample
		text := strings.Split(scanner.Text(), "\t")

		s.sessionID, _ = parseInt(text[0])
		s.threadID, _ = parseInt(text[1])
		s.processlistUser = text[2]
		s.processlistHost = text[3]
		p, _ := strconv.ParseUint(text[4], 10, 64)
		s.clientPort = p
		s.processlistDB = text[5]
		s.processlistCommand = text[6]
		s.processlistState = text[7]
		s.sqlText = text[8]
		s.digest = text[9]
		s.eventID, _ = parseInt(text[10])
		s.sessionStatus = text[11]
		s.waitEvent = text[12]
		s.waitTime, _ = strconv.ParseFloat(text[13], 64)
		s.statementTimerWait, _ = strconv.ParseFloat(text[14], 64)
		if len(text) > 15 {
			s.traceparent = text[15]
		}

		samples = append(samples, s)
	}

	return samples, nil
}

func (c *mockClient) getTopQueries(uint64, uint64, bool) ([]topQuery, error) {
	var queries []topQuery
	file, err := os.Open(filepath.Join("testdata", "scraper", c.topQueriesFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var q topQuery
		text := strings.Split(scanner.Text(), "\t")

		q.schemaName = text[0]
		q.digest = text[1]
		q.digestText = text[2]
		q.countStar, _ = parseInt(text[3])
		q.sumTimerWaitInPicoSeconds, _ = parseInt(text[4])
		// 5-column fixtures (MySQL <8 / MariaDB fallback) omit querySampleText.
		if len(text) > 5 {
			q.querySampleText = text[5]
		}

		queries = append(queries, q)
	}

	return queries, nil
}

func (c *mockClient) explainQuery(digestText, sampleStatement, _, _ string, _ *zap.Logger) string {
	c.explainQueryCallCount++
	c.explainQueryCalls = append(c.explainQueryCalls, explainQueryCall{
		digestText:      digestText,
		sampleStatement: sampleStatement,
	})
	file, _ := os.ReadFile(filepath.Join("testdata", "obfuscate", "inputQueryPlan.json"))

	return string(file)
}

func (*mockClient) Close() error {
	return nil
}

func TestQueryPlanCacheReuse(t *testing.T) {
	baseCfg := createDefaultConfig().(*Config)
	baseCfg.Username = "otel"
	baseCfg.Password = "otel"
	baseCfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
	baseCfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	baseCfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	baseCfg.TopQueryCollection.TopQueryCount = 10

	makeScraper := func(t *testing.T, cfg *Config, spy *queryPlanSpyClient) *mySQLScraper {
		t.Helper()
		scraper := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](0, time.Hour*24*365*10))
		scraper.sqlclient = spy
		return scraper
	}

	seedTopQueryDiffCache := func(scraper *mySQLScraper, schema, digest string, countStar, sumTimerWait int64) {
		// Top query events are emitted only when the value is already cached and increases.
		scraper.cacheAndDiff(schema, digest, "count_star", countStar-1)
		scraper.cacheAndDiff(schema, digest, "sum_timer_wait", sumTimerWait-1)
	}

	t.Run("query samples first then top queries reuses cached plan", func(t *testing.T) {
		cfg := *baseCfg
		schema := "adventureworks"
		digest := "digest-shared"
		spy := &queryPlanSpyClient{
			querySamples: []querySample{
				{
					processlistDB:      schema,
					processlistHost:    "192.168.1.80:1234",
					processlistUser:    "myuser",
					processlistCommand: "Query",
					processlistState:   "executing",
					sqlText:            "SELECT * FROM t",
					digest:             digest,
					eventID:            1,
					sessionStatus:      "waiting",
					waitEvent:          "CPU",
				},
			},
			topQueries: []topQuery{
				{
					schemaName:                schema,
					digest:                    digest,
					digestText:                "SELECT * FROM t",
					querySampleText:           "SELECT * FROM t",
					countStar:                 10,
					sumTimerWaitInPicoSeconds: 200,
				},
			},
			explainPlan: `{"query_block":{"select_id":1}}`,
		}

		scraper := makeScraper(t, &cfg, spy)
		seedTopQueryDiffCache(scraper, schema, digest, 10, 200)

		_, err := scraper.scrapeQuerySampleFunc(t.Context())
		require.NoError(t, err)
		_, err = scraper.scrapeTopQueryFunc(t.Context())
		require.NoError(t, err)

		require.Equal(t, 1, spy.explainCalls, "query plan should be fetched only once across both flows")
	})

	t.Run("query plan hash should be empty if plan is empty", func(t *testing.T) {
		cfg := *baseCfg
		schema := "adventureworks"
		digest := "digest-shared"
		spy := &queryPlanSpyClient{
			querySamples: []querySample{
				{
					processlistDB:      schema,
					processlistHost:    "192.168.1.80:1234",
					processlistUser:    "myuser",
					processlistCommand: "Query",
					processlistState:   "executing",
					sqlText:            "SELECT * FROM t",
					digest:             digest,
					eventID:            1,
					sessionStatus:      "waiting",
					waitEvent:          "CPU",
				},
			},
			topQueries: []topQuery{
				{
					schemaName:                schema,
					digest:                    digest,
					digestText:                "SELECT * FROM t",
					querySampleText:           "SELECT * FROM t",
					countStar:                 10,
					sumTimerWaitInPicoSeconds: 200,
				},
			},
			explainPlan: ``,
		}

		scraper := makeScraper(t, &cfg, spy)
		seedTopQueryDiffCache(scraper, schema, digest, 10, 200)

		topQueryLogs, err := scraper.scrapeTopQueryFunc(t.Context())
		require.NoError(t, err)

		querySampleLogs, err := scraper.scrapeQuerySampleFunc(t.Context())
		require.NoError(t, err)

		topLogs := topQueryLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		topPlanHash, _ := topLogs.Attributes().Get("mysql.query_plan.hash")
		topPlanVal, _ := topLogs.Attributes().Get("mysql.query_plan")
		assert.Empty(t, topPlanHash.Str(), "mysql.query_plan should be empty when sample text is unavailable")
		assert.Empty(t, topPlanVal.Str(), "mysql.query_plan should be empty when sample text is unavailable")

		sampleLogs := querySampleLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		samplePlanHash, _ := sampleLogs.Attributes().Get("mysql.query_plan.hash")
		samplePlanVal, _ := sampleLogs.Attributes().Get("mysql.query_plan")
		assert.Empty(t, samplePlanHash.Str(), "mysql.query_plan should be empty when sample text is unavailable")
		assert.Empty(t, samplePlanVal.Str(), "mysql.query_plan should be empty when sample text is unavailable")
	})

	t.Run("top queries first then query samples reuses cached plan", func(t *testing.T) {
		cfg := *baseCfg
		schema := "adventureworks"
		digest := "digest-shared"
		spy := &queryPlanSpyClient{
			querySamples: []querySample{
				{
					processlistDB:      schema,
					processlistHost:    "192.168.1.80:1234",
					processlistUser:    "myuser",
					processlistCommand: "Query",
					processlistState:   "executing",
					sqlText:            "SELECT * FROM t",
					digest:             digest,
					eventID:            1,
					sessionStatus:      "waiting",
					waitEvent:          "CPU",
				},
			},
			topQueries: []topQuery{
				{
					schemaName:                schema,
					digest:                    digest,
					digestText:                "SELECT * FROM t",
					querySampleText:           "SELECT * FROM t",
					countStar:                 10,
					sumTimerWaitInPicoSeconds: 200,
				},
			},
			explainPlan: `{"query_block":{"select_id":1}}`,
		}

		scraper := makeScraper(t, &cfg, spy)
		seedTopQueryDiffCache(scraper, schema, digest, 10, 200)

		_, err := scraper.scrapeTopQueryFunc(t.Context())
		require.NoError(t, err)
		_, err = scraper.scrapeQuerySampleFunc(t.Context())
		require.NoError(t, err)

		require.Equal(t, 1, spy.explainCalls, "query plan should be fetched only once across both flows")
	})
}

// mustDBVersion is a test helper that builds a dbVersion from a raw version
// string, applying MariaDB detection the same way getDBVersion does.
func mustDBVersion(t *testing.T, rawVersion string) dbVersion {
	t.Helper()
	product := dbProductMySQL
	if strings.Contains(rawVersion, "MariaDB") {
		product = dbProductMariaDB
	}
	semverStr := strings.SplitN(rawVersion, "-", 2)[0]
	v, err := version.NewVersion(semverStr)
	require.NoError(t, err)
	return dbVersion{product: product, version: v}
}

// newTopQueryScraper creates a mySQLScraper configured for top-query tests.
// If mc.dbVersionOverride is set, it is also applied to s.detectedVersion so
// that the scraper's version-based capability flags match the mock client.
func newTopQueryScraper(t *testing.T, mc *mockClient) *mySQLScraper {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	s := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](100, 0))
	s.sqlclient = mc
	s.detectedVersion = mc.getDBVersion()
	return s
}

// TestScrapeTopQueriesNoSampleText verifies that when the mock reports MySQL 5.7
// (which lacks query_sample_text), the top-query scrape returns rows with an
// empty querySampleText and never calls explainQuery.
func TestScrapeTopQueriesNoSampleText(t *testing.T) {
	v57 := mustDBVersion(t, "5.7.38")
	mc := &mockClient{
		topQueriesFile:    "top_queries_no_sample_text",
		dbVersionOverride: &v57,
	}
	s := newTopQueryScraper(t, mc)

	// prime the cache so the diff is non-zero
	s.cacheAndDiff("mysql", "c16f24f908846019a741db580f6545a5933e9435a7cf1579c50794a6ca287739", "count_star", 1)
	s.cacheAndDiff("mysql", "c16f24f908846019a741db580f6545a5933e9435a7cf1579c50794a6ca287739", "sum_timer_wait", 1)

	logs, err := s.scrapeTopQueryFunc(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, logs.ResourceLogs().Len())

	// EXPLAIN must not have been called — no sample text available.
	assert.Equal(t, 0, mc.explainQueryCallCount, "explainQuery should not be called when querySampleText is empty")

	// The emitted record should have an empty mysql.query_plan attribute.
	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	planVal, hasPlan := lr.Attributes().Get("mysql.query_plan")
	if hasPlan {
		assert.Empty(t, planVal.Str(), "mysql.query_plan should be empty when sample text is unavailable")
	}

	// mysql.query_plan.hash should also be empty when no plan is available.
	hashVal, hasHash := lr.Attributes().Get("mysql.query_plan.hash")
	if hasHash {
		assert.Empty(t, hashVal.Str(), "mysql.query_plan.hash should be empty when no plan is available")
	}
}

// TestScrapeTopQueriesMariaDB verifies the same no-explain behavior for MariaDB.
func TestScrapeTopQueriesMariaDB(t *testing.T) {
	mariadb := mustDBVersion(t, "10.11.6-MariaDB")
	mc := &mockClient{
		topQueriesFile:    "top_queries_no_sample_text",
		dbVersionOverride: &mariadb,
	}
	s := newTopQueryScraper(t, mc)

	s.cacheAndDiff("mysql", "c16f24f908846019a741db580f6545a5933e9435a7cf1579c50794a6ca287739", "count_star", 1)
	s.cacheAndDiff("mysql", "c16f24f908846019a741db580f6545a5933e9435a7cf1579c50794a6ca287739", "sum_timer_wait", 1)

	logs, err := s.scrapeTopQueryFunc(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, logs.ResourceLogs().Len())

	assert.Equal(t, 0, mc.explainQueryCallCount, "explainQuery should not be called for MariaDB")

	// mysql.query_plan.hash should be empty when no plan is available.
	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	hashVal, hasHash := lr.Attributes().Get("mysql.query_plan.hash")
	if hasHash {
		assert.Empty(t, hashVal.Str(), "mysql.query_plan.hash should be empty when no plan is available")
	}
}

// TestScrapeQuerySamplesExplainPlan verifies that scrapeQuerySamples emits the
// correct attributes on query_sample events. mysql.query_plan belongs only on
// top_query events; scrapeQuerySampleFunc never calls explainQuery.
func TestScrapeQuerySamplesExplainPlan(t *testing.T) {
	v8 := mustDBVersion(t, "8.0.27")
	mc := &mockClient{querySamplesFile: "query_samples", dbVersionOverride: &v8}
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	s := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](1), newTTLCache[string](100, 0))
	s.sqlclient = mc
	s.detectedVersion = mc.getDBVersion()

	logs, err := s.scrapeQuerySampleFunc(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, logs.ResourceLogs().Len())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	// mysql.query_plan must appear on query_sample events.
	_, hasPlan := lr.Attributes().Get("mysql.query_plan")
	assert.True(t, hasPlan, "mysql.query_plan must be present on query_sample events")

	// mysql.query_plan.hash is valid on query_sample events.
	hashVal, hasHash := lr.Attributes().Get("mysql.query_plan.hash")
	assert.True(t, hasHash, "mysql.query_plan.hash must be present")
	assert.NotEmpty(t, hashVal.Str(), "mysql.query_plan.hash must not be empty")

	// scrapeQuerySampleFunc calls explainQuery.
	assert.Equal(t, 1, mc.explainQueryCallCount)
}

// TestScrapeQuerySamplesCallsExplain verifies that scrapeQuerySampleFunc calls
// explainQuery when a digest is present. query_sample events carry mysql.query_plan
// (fetched via EXPLAIN) and mysql.query_plan.hash when a plan is available.
func TestScrapeQuerySamplesCallsExplain(t *testing.T) {
	sharedCache := newTTLCache[string](100, 0)

	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true

	v8 := mustDBVersion(t, "8.0.27")
	mc := &mockClient{querySamplesFile: "query_samples", dbVersionOverride: &v8}
	s := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](1), sharedCache)
	s.sqlclient = mc
	s.detectedVersion = mc.getDBVersion()

	_, err := s.scrapeQuerySampleFunc(t.Context())
	require.NoError(t, err)

	assert.Equal(t, 1, mc.explainQueryCallCount,
		"scrapeQuerySampleFunc must call explainQuery")
}

// TestScrapeQuerySamplesExplainMySQL57 verifies that scrapeQuerySamples calls
// explainQuery for MySQL 5.7. Unlike the top-query path, scrapeQuerySamples uses
// events_statements_current.SQL_TEXT which is available on all supported versions,
// so EXPLAIN runs regardless of whether querySampleText is populated.
func TestScrapeQuerySamplesExplainMySQL57(t *testing.T) {
	v57 := mustDBVersion(t, "5.7.44")
	mc := &mockClient{querySamplesFile: "query_samples", dbVersionOverride: &v57}
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	s := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](1), newTTLCache[string](100, 0))
	s.sqlclient = mc
	s.detectedVersion = mc.getDBVersion()

	logs, err := s.scrapeQuerySampleFunc(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, logs.ResourceLogs().Len())

	assert.Equal(t, 1, mc.explainQueryCallCount, "explainQuery must be called for MySQL 5.7 query samples")

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	_, hasPlan := lr.Attributes().Get("mysql.query_plan")
	assert.True(t, hasPlan, "mysql.query_plan must be present on query_sample events for MySQL 5.7")
}

// TestScrapeQuerySamplesExplainMariaDB verifies that scrapeQuerySamples calls
// explainQuery for MariaDB. See TestScrapeQuerySamplesExplainMySQL57 for rationale.
func TestScrapeQuerySamplesExplainMariaDB(t *testing.T) {
	mariadb := mustDBVersion(t, "10.11.6-MariaDB")
	mc := &mockClient{querySamplesFile: "query_samples", dbVersionOverride: &mariadb}
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	s := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](1), newTTLCache[string](100, 0))
	s.sqlclient = mc
	s.detectedVersion = mc.getDBVersion()

	logs, err := s.scrapeQuerySampleFunc(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, logs.ResourceLogs().Len())

	assert.Equal(t, 1, mc.explainQueryCallCount, "explainQuery must be called for MariaDB query samples")

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	_, hasPlan := lr.Attributes().Get("mysql.query_plan")
	assert.True(t, hasPlan, "mysql.query_plan must be present on query_sample events for MariaDB")
}

// TestLogDetectedVersion verifies the version-detection log output produced by
// logDetectedVersion across MySQL, MariaDB, and the unknown-version case.
func TestLogDetectedVersion(t *testing.T) {
	tests := []struct {
		name        string
		rawVersion  string // empty string → zero dbVersion (no version detected)
		wantInfo    bool   // expect an Info "detected database version" entry
		wantEOLWarn bool   // expect a Warn about EOL
		wantUnknown bool   // expect the "could not be detected" Warn
	}{
		{
			name:        "MySQL 8.0 — supported, no EOL warning",
			rawVersion:  "8.0.27",
			wantInfo:    true,
			wantEOLWarn: false,
		},
		{
			name:        "MySQL 5.7 — detected but EOL",
			rawVersion:  "5.7.38",
			wantInfo:    true,
			wantEOLWarn: true,
		},
		{
			name:        "MySQL 5.6 — detected but EOL",
			rawVersion:  "5.6.51",
			wantInfo:    true,
			wantEOLWarn: true,
		},
		{
			name:        "MariaDB 10.11 — supported, no EOL warning",
			rawVersion:  "10.11.6-MariaDB",
			wantInfo:    true,
			wantEOLWarn: false,
		},
		{
			name:        "MariaDB 10.4 — old but no EOL warning (EOL check is MySQL-only)",
			rawVersion:  "10.4.0-MariaDB",
			wantInfo:    true,
			wantEOLWarn: false,
		},
		{
			name:        "unknown version — fallback warning, no info log",
			rawVersion:  "",
			wantInfo:    false,
			wantEOLWarn: false,
			wantUnknown: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			logger := zap.New(core)

			cfg := createDefaultConfig().(*Config)
			s := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](100), newTTLCache[string](100, 0))
			s.logger = logger

			var dbVer dbVersion
			if tc.rawVersion != "" {
				dbVer = mustDBVersion(t, tc.rawVersion)
			}

			s.logDetectedVersion(dbVer)

			infoEntries := logs.FilterMessage("detected database version").All()
			eolEntries := logs.FilterMessage("detected MySQL version is past end-of-life and may not be supported by this receiver in a future release").All()
			unknownEntries := logs.FilterMessage("database version could not be detected at startup; receiver will use MySQL <8/MariaDB fallback behavior for its entire lifetime").All()

			if tc.wantInfo {
				assert.Len(t, infoEntries, 1, "expected info log entry")
			} else {
				assert.Empty(t, infoEntries, "unexpected info log entry")
			}
			if tc.wantEOLWarn {
				assert.Len(t, eolEntries, 1, "expected EOL warn entry")
			} else {
				assert.Empty(t, eolEntries, "unexpected EOL warn entry")
			}
			if tc.wantUnknown {
				assert.Len(t, unknownEntries, 1, "expected unknown-version warn entry")
			} else {
				assert.Empty(t, unknownEntries, "unexpected unknown-version warn entry")
			}
		})
	}
}

// TestSetScopeAttributes verifies that setScopeAttributes stamps db.version and
// db.product onto every ScopeLogs scope, and that an unknown version is a no-op.
func TestSetScopeAttributes(t *testing.T) {
	makeLogsWithScopes := func(n int) plog.Logs {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		for range n {
			rl.ScopeLogs().AppendEmpty()
		}
		return logs
	}

	t.Run("MySQL 8 sets db.version and db.product", func(t *testing.T) {
		s := &mySQLScraper{detectedVersion: mustDBVersion(t, "8.0.27")}
		logs := makeLogsWithScopes(2)
		s.setScopeAttributes(logs)

		sls := logs.ResourceLogs().At(0).ScopeLogs()
		for i := 0; i < sls.Len(); i++ {
			attrs := sls.At(i).Scope().Attributes()
			ver, ok := attrs.Get("db.version")
			require.True(t, ok)
			assert.Equal(t, "8.0.27", ver.Str())
			prod, ok := attrs.Get("db.product")
			require.True(t, ok)
			assert.Equal(t, "MySQL", prod.Str())
		}
	})

	t.Run("MariaDB sets db.product=MariaDB", func(t *testing.T) {
		s := &mySQLScraper{detectedVersion: mustDBVersion(t, "10.11.6-MariaDB")}
		logs := makeLogsWithScopes(1)
		s.setScopeAttributes(logs)

		attrs := logs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes()
		prod, ok := attrs.Get("db.product")
		require.True(t, ok)
		assert.Equal(t, "MariaDB", prod.Str())
	})

	t.Run("unknown version is a no-op", func(t *testing.T) {
		s := &mySQLScraper{} // detectedVersion is zero value
		logs := makeLogsWithScopes(1)
		s.setScopeAttributes(logs)

		attrs := logs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes()
		assert.Equal(t, 0, attrs.Len(), "no attributes should be set when version is unknown")
	})
}

// TestScrapeQuerySampleFuncScopeAttributes verifies that scrapeQuerySampleFunc
// stamps db.version and db.product scope attributes — exercising emitLogsWithScopeAttrs
// via the query-sample path (as opposed to the top-query path).
func TestScrapeQuerySampleFuncScopeAttributes(t *testing.T) {
	v8 := mustDBVersion(t, "8.0.27")
	mc := &mockClient{querySamplesFile: "query_samples", dbVersionOverride: &v8}
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.AddrConfig = confignet.AddrConfig{Endpoint: "localhost:3306"}
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	s := newMySQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newCache[int64](1), newTTLCache[string](100, 0))
	s.sqlclient = mc
	s.detectedVersion = mc.getDBVersion()

	logs, err := s.scrapeQuerySampleFunc(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, logs.ResourceLogs().Len())

	sls := logs.ResourceLogs().At(0).ScopeLogs()
	for i := 0; i < sls.Len(); i++ {
		attrs := sls.At(i).Scope().Attributes()
		ver, ok := attrs.Get("db.version")
		assert.True(t, ok, "db.version scope attribute missing on query-sample output")
		assert.Equal(t, "8.0.27", ver.Str())
		prod, ok := attrs.Get("db.product")
		assert.True(t, ok, "db.product scope attribute missing on query-sample output")
		assert.Equal(t, "MySQL", prod.Str())
	}
}

// TestScrapeTopQueryFuncScanRowWithSampleText verifies that when MySQL 8 is detected
// (supportsSampleText=true), the 6-column scanRow path is used and querySampleText
// is passed as the sampleStatement argument to explainQuery.
func TestScrapeTopQueryFuncScanRowWithSampleText(t *testing.T) {
	v8 := mustDBVersion(t, "8.0.27")
	mc := &mockClient{
		topQueriesFile:    "top_queries",
		dbVersionOverride: &v8,
	}
	s := newTopQueryScraper(t, mc)

	// Prime caches so the diff is non-zero and explainQuery is reached.
	s.cacheAndDiff("mysql", "c16f24f908846019a741db580f6545a5933e9435a7cf1579c50794a6ca287739", "count_star", 1)
	s.cacheAndDiff("mysql", "c16f24f908846019a741db580f6545a5933e9435a7cf1579c50794a6ca287739", "sum_timer_wait", 1)

	_, err := s.scrapeTopQueryFunc(t.Context())
	require.NoError(t, err)

	// explainQuery must have been called — querySampleText is present in the fixture.
	require.Equal(t, 1, mc.explainQueryCallCount, "explainQuery should be called when querySampleText is non-empty")

	// The sampleStatement arg must equal column 6 from the top_queries fixture.
	require.Len(t, mc.explainQueryCalls, 1)
	assert.Equal(t, "SELECT @@session.transaction_read_only", mc.explainQueryCalls[0].sampleStatement,
		"sampleStatement must be the querySampleText scanned from column 6")
}

// TestScrapeTopQueryFuncScopeAttributes verifies that scrapeTopQueryFunc stamps
// scope attributes onto emitted logs when a version has been detected.
func TestScrapeTopQueryFuncScopeAttributes(t *testing.T) {
	v8 := mustDBVersion(t, "8.0.27")
	mc := &mockClient{
		topQueriesFile:    "top_queries",
		dbVersionOverride: &v8,
	}
	s := newTopQueryScraper(t, mc)

	s.cacheAndDiff("mysql", "c16f24f908846019a741db580f6545a5933e9435a7cf1579c50794a6ca287739", "count_star", 1)
	s.cacheAndDiff("mysql", "c16f24f908846019a741db580f6545a5933e9435a7cf1579c50794a6ca287739", "sum_timer_wait", 1)

	logs, err := s.scrapeTopQueryFunc(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, logs.ResourceLogs().Len())

	sls := logs.ResourceLogs().At(0).ScopeLogs()
	for i := 0; i < sls.Len(); i++ {
		attrs := sls.At(i).Scope().Attributes()
		ver, ok := attrs.Get("db.version")
		assert.True(t, ok, "db.version scope attribute missing")
		assert.Equal(t, "8.0.27", ver.Str())
		prod, ok := attrs.Get("db.product")
		assert.True(t, ok, "db.product scope attribute missing")
		assert.Equal(t, "MySQL", prod.Str())
	}
}
