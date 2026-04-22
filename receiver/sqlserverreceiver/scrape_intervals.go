// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import "time"

type metricScrape struct {
	enabled bool
	iv      time.Duration
}

func mi(enabled bool, iv time.Duration) metricScrape {
	return metricScrape{enabled: enabled, iv: iv}
}

// minMetricScrapeInterval returns the smallest effective scrape interval among enabled
// metrics in a shared SQL query bucket. Enabled metrics with collection_interval zero use
// the receiver-level collection_interval. The result is normalized so values equal to the
// receiver default are returned as zero (letting scraperhelper apply the default).
func minMetricScrapeInterval(cfg *Config, metrics ...metricScrape) time.Duration {
	var min time.Duration
	var has bool
	for _, m := range metrics {
		if !m.enabled {
			continue
		}
		cand := m.iv
		if cand <= 0 {
			cand = cfg.ControllerConfig.CollectionInterval
		}
		if !has || cand < min {
			min = cand
			has = true
		}
	}
	if !has {
		return 0
	}
	return normalizeScraperInterval(cfg, min)
}

func normalizeScraperInterval(cfg *Config, d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	if d == cfg.ControllerConfig.CollectionInterval {
		return 0
	}
	return d
}

func metricsScrapeIntervalForQuery(cfg *Config, query string) time.Duration {
	m := &cfg.Metrics
	switch query {
	case getSQLServerDatabaseIOQuery(cfg.InstanceName):
		return minMetricScrapeInterval(cfg,
			mi(m.SqlserverDatabaseLatency.Enabled, m.SqlserverDatabaseLatency.CollectionInterval),
			mi(m.SqlserverDatabaseOperations.Enabled, m.SqlserverDatabaseOperations.CollectionInterval),
			mi(m.SqlserverDatabaseIo.Enabled, m.SqlserverDatabaseIo.CollectionInterval),
		)
	case getSQLServerPerformanceCounterQuery(cfg.InstanceName):
		return minMetricScrapeInterval(cfg,
			mi(m.SqlserverBatchRequestRate.Enabled, m.SqlserverBatchRequestRate.CollectionInterval),
			mi(m.SqlserverBatchSQLCompilationRate.Enabled, m.SqlserverBatchSQLCompilationRate.CollectionInterval),
			mi(m.SqlserverBatchSQLRecompilationRate.Enabled, m.SqlserverBatchSQLRecompilationRate.CollectionInterval),
			mi(m.SqlserverDatabaseBackupOrRestoreRate.Enabled, m.SqlserverDatabaseBackupOrRestoreRate.CollectionInterval),
			mi(m.SqlserverDatabaseExecutionErrors.Enabled, m.SqlserverDatabaseExecutionErrors.CollectionInterval),
			mi(m.SqlserverDatabaseFullScanRate.Enabled, m.SqlserverDatabaseFullScanRate.CollectionInterval),
			mi(m.SqlserverDatabaseTempdbSpace.Enabled, m.SqlserverDatabaseTempdbSpace.CollectionInterval),
			mi(m.SqlserverDatabaseTempdbVersionStoreSize.Enabled, m.SqlserverDatabaseTempdbVersionStoreSize.CollectionInterval),
			mi(m.SqlserverDeadlockRate.Enabled, m.SqlserverDeadlockRate.CollectionInterval),
			mi(m.SqlserverIndexSearchRate.Enabled, m.SqlserverIndexSearchRate.CollectionInterval),
			mi(m.SqlserverLockTimeoutRate.Enabled, m.SqlserverLockTimeoutRate.CollectionInterval),
			mi(m.SqlserverLockWaitCount.Enabled, m.SqlserverLockWaitCount.CollectionInterval),
			mi(m.SqlserverLockWaitRate.Enabled, m.SqlserverLockWaitRate.CollectionInterval),
			mi(m.SqlserverLoginRate.Enabled, m.SqlserverLoginRate.CollectionInterval),
			mi(m.SqlserverLogoutRate.Enabled, m.SqlserverLogoutRate.CollectionInterval),
			mi(m.SqlserverMemoryGrantsPendingCount.Enabled, m.SqlserverMemoryGrantsPendingCount.CollectionInterval),
			mi(m.SqlserverMemoryUsage.Enabled, m.SqlserverMemoryUsage.CollectionInterval),
			mi(m.SqlserverPageBufferCacheFreeListStallsRate.Enabled, m.SqlserverPageBufferCacheFreeListStallsRate.CollectionInterval),
			mi(m.SqlserverPageBufferCacheHitRatio.Enabled, m.SqlserverPageBufferCacheHitRatio.CollectionInterval),
			mi(m.SqlserverPageLookupRate.Enabled, m.SqlserverPageLookupRate.CollectionInterval),
			mi(m.SqlserverProcessesBlocked.Enabled, m.SqlserverProcessesBlocked.CollectionInterval),
			mi(m.SqlserverReplicaDataRate.Enabled, m.SqlserverReplicaDataRate.CollectionInterval),
			mi(m.SqlserverResourcePoolDiskThrottledReadRate.Enabled, m.SqlserverResourcePoolDiskThrottledReadRate.CollectionInterval),
			mi(m.SqlserverResourcePoolDiskOperations.Enabled, m.SqlserverResourcePoolDiskOperations.CollectionInterval),
			mi(m.SqlserverResourcePoolDiskThrottledWriteRate.Enabled, m.SqlserverResourcePoolDiskThrottledWriteRate.CollectionInterval),
			mi(m.SqlserverTableCount.Enabled, m.SqlserverTableCount.CollectionInterval),
			mi(m.SqlserverTransactionDelay.Enabled, m.SqlserverTransactionDelay.CollectionInterval),
			mi(m.SqlserverTransactionMirrorWriteRate.Enabled, m.SqlserverTransactionMirrorWriteRate.CollectionInterval),
			mi(m.SqlserverUserConnectionCount.Enabled, m.SqlserverUserConnectionCount.CollectionInterval),
		)
	case getSQLServerPropertiesQuery(cfg.InstanceName):
		return minMetricScrapeInterval(cfg,
			mi(m.SqlserverDatabaseCount.Enabled, m.SqlserverDatabaseCount.CollectionInterval),
			mi(m.SqlserverCPUCount.Enabled, m.SqlserverCPUCount.CollectionInterval),
			mi(m.SqlserverComputerUptime.Enabled, m.SqlserverComputerUptime.CollectionInterval),
		)
	case getSQLServerWaitStatsQuery(cfg.InstanceName):
		return minMetricScrapeInterval(cfg,
			mi(m.SqlserverOsWaitDuration.Enabled, m.SqlserverOsWaitDuration.CollectionInterval),
		)
	default:
		return 0
	}
}

func logsScrapeIntervalForQuery(cfg *Config, query string) time.Duration {
	switch query {
	case getSQLServerQueryTextAndPlanQuery():
		d := cfg.Events.DbServerTopQuery.CollectionInterval
		if d <= 0 {
			d = cfg.TopQueryCollection.CollectionInterval
		}
		return normalizeScraperInterval(cfg, d)
	case getSQLServerQuerySamplesQuery():
		return normalizeScraperInterval(cfg, cfg.Events.DbServerQuerySample.CollectionInterval)
	default:
		return 0
	}
}
