// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestScrape(t *testing.T) {
	t.Run("successful scrape", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Username = "otel"
		cfg.Password = "otel"
		cfg.NetAddr = confignet.NetAddr{Endpoint: "localhost:3306"}
		cfg.MetricsBuilderConfig.Metrics.MysqlStatementEventCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlStatementEventWaitTime.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlConnectionErrors.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlMysqlxWorkerThreads.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlJoins.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableOpenCache.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlQueryClientCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlQueryCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlQuerySlowCount.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitReadCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitReadTime.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitWriteCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitWriteTime.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlClientNetworkIo.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlPreparedStatements.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlCommands.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlReplicaSQLDelay.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlReplicaTimeBehindSource.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlConnectionCount.Enabled = true

		scraper := newMySQLScraper(receivertest.NewNopCreateSettings(), cfg)
		scraper.sqlclient = &mockClient{
			globalStatsFile:             "global_stats",
			innodbStatsFile:             "innodb_stats",
			tableIoWaitsFile:            "table_io_waits_stats",
			indexIoWaitsFile:            "index_io_waits_stats",
			statementEventsFile:         "statement_events",
			tableLockWaitEventStatsFile: "table_lock_wait_event_stats",
			replicaStatusFile:           "replica_stats",
		}

		scraper.renameCommands = true

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(actualMetrics, expectedMetrics,
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	})

	t.Run("scrape has partial failure", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Username = "otel"
		cfg.Password = "otel"
		cfg.NetAddr = confignet.NetAddr{Endpoint: "localhost:3306"}
		cfg.MetricsBuilderConfig.Metrics.MysqlReplicaSQLDelay.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlReplicaTimeBehindSource.Enabled = true

		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitReadCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitReadTime.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitWriteCount.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.MysqlTableLockWaitWriteTime.Enabled = true

		scraper := newMySQLScraper(receivertest.NewNopCreateSettings(), cfg)
		scraper.sqlclient = &mockClient{
			globalStatsFile:             "global_stats_partial",
			innodbStatsFile:             "innodb_stats_empty",
			tableIoWaitsFile:            "table_io_waits_stats_empty",
			indexIoWaitsFile:            "index_io_waits_stats_empty",
			statementEventsFile:         "statement_events_empty",
			tableLockWaitEventStatsFile: "table_lock_wait_event_stats_empty",
			replicaStatusFile:           "replica_stats_empty",
		}

		actualMetrics, scrapeErr := scraper.scrape(context.Background())
		require.Error(t, scrapeErr)

		expectedFile := filepath.Join("testdata", "scraper", "expected_partial.yaml")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)
		assert.NoError(t, pmetrictest.CompareMetrics(actualMetrics, expectedMetrics,
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp()))

		var partialError scrapererror.PartialScrapeError
		require.True(t, errors.As(scrapeErr, &partialError), "returned error was not PartialScrapeError")
		// 5 comes from 4 failed "must-have" metrics that aren't present,
		// and the other failure comes from a row that fails to parse as a number
		require.Equal(t, partialError.Failed, 5, "Expected partial error count to be 5")
	})

}

var _ client = (*mockClient)(nil)

type mockClient struct {
	globalStatsFile             string
	innodbStatsFile             string
	tableIoWaitsFile            string
	indexIoWaitsFile            string
	statementEventsFile         string
	tableLockWaitEventStatsFile string
	replicaStatusFile           string
}

func readFile(fname string) (map[string]string, error) {
	var stats = map[string]string{}
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

func (c *mockClient) Connect() error {
	return nil
}

func (c *mockClient) getVersion() (string, error) {
	return "8.0.27", nil
}

func (c *mockClient) getGlobalStats() (map[string]string, error) {
	return readFile(c.globalStatsFile)
}

func (c *mockClient) getInnodbStats() (map[string]string, error) {
	return readFile(c.innodbStatsFile)
}

func (c *mockClient) getTableIoWaitsStats() ([]TableIoWaitsStats, error) {
	var stats []TableIoWaitsStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.tableIoWaitsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s TableIoWaitsStats
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

func (c *mockClient) getIndexIoWaitsStats() ([]IndexIoWaitsStats, error) {
	var stats []IndexIoWaitsStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.indexIoWaitsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s IndexIoWaitsStats
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

func (c *mockClient) getStatementEventsStats() ([]StatementEventStats, error) {
	var stats []StatementEventStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.statementEventsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s StatementEventStats
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

func (c *mockClient) getReplicaStatusStats() ([]ReplicaStatusStats, error) {
	var stats []ReplicaStatusStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.replicaStatusFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s ReplicaStatusStats
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
		s.replicateIgnoreServerIds = text[38]
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

func (c *mockClient) Close() error {
	return nil
}
