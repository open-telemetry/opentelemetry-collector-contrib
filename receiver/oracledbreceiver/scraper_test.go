// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sqlcomments"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

func TestScraper_ErrorOnStart(t *testing.T) {
	scrpr := oracleScraper{
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, errors.New("oops")
		},
	}
	err := scrpr.start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
}

var queryResponses = map[string][]metricRow{
	statsSQL: {
		{"NAME": enqueueDeadlocks, "VALUE": "18"},
		{"NAME": exchangeDeadlocks, "VALUE": "88898"},
		{"NAME": executeCount, "VALUE": "178878"},
		{"NAME": parseCountTotal, "VALUE": "1999"},
		{"NAME": parseCountHard, "VALUE": "1"},
		{"NAME": userCommits, "VALUE": "187778888"},
		{"NAME": userRollbacks, "VALUE": "1898979879789"},
		{"NAME": physicalReads, "VALUE": "1887777"},
		{"NAME": physicalReadsDirect, "VALUE": "31337"},
		{"NAME": sessionLogicalReads, "VALUE": "189"},
		{"NAME": cpuTime, "VALUE": "1887"},
		{"NAME": pgaMemory, "VALUE": "1999887"},
		{"NAME": dbBlockGets, "VALUE": "42"},
		{"NAME": consistentGets, "VALUE": "78944"},
		// PR2: I/O performance v$sysstat rows
		{"NAME": physicalReadBytesStat, "VALUE": "1024000"},
		{"NAME": physicalWriteBytesStat, "VALUE": "512000"},
		{"NAME": physicalReadTotalBytesStat, "VALUE": "2048000"},
		{"NAME": physicalWriteTotalBytesStat, "VALUE": "1024000"},
		{"NAME": physicalReadTotalIORequestsStat, "VALUE": "5000"},
		{"NAME": physicalWriteTotalIORequestsStat, "VALUE": "2500"},
		{"NAME": physicalReadMultiBlockReqStat, "VALUE": "1500"},
		{"NAME": physicalWriteMultiBlockReqStat, "VALUE": "750"},
		{"NAME": physicalWritesFromCacheStat, "VALUE": "1800"},
		{"NAME": sqlnetBytesRecvFromClient, "VALUE": "300000"},
		{"NAME": sqlnetBytesSentToClient, "VALUE": "600000"},
		{"NAME": sqlnetBytesRecvFromDBLink, "VALUE": "150000"},
		{"NAME": sqlnetBytesSentToDBLink, "VALUE": "75000"},
		// Workload analysis v$sysstat rows
		{"NAME": tableScansDirectReadStat, "VALUE": "100"},
		{"NAME": tableScansLongTablesStat, "VALUE": "200"},
		{"NAME": tableScansRowidRangesStat, "VALUE": "50"},
		{"NAME": tableScanRowsGottenStat, "VALUE": "999999"},
		{"NAME": indexFastFullScansDirectStat, "VALUE": "10"},
		{"NAME": indexFastFullScansFullStat, "VALUE": "20"},
		{"NAME": indexFastFullScansRowidStat, "VALUE": "5"},
		{"NAME": enqueueConversionsStat, "VALUE": "11"},
		{"NAME": enqueueReleasesStat, "VALUE": "22"},
		{"NAME": enqueueRequestsStat, "VALUE": "33"},
		{"NAME": enqueueTimeoutsStat, "VALUE": "44"},
		{"NAME": enqueueWaitsStat, "VALUE": "55"},
		{"NAME": lobReadsStat, "VALUE": "120"},
		{"NAME": lobWritesStat, "VALUE": "240"},
		{"NAME": parseTimeCPUStat, "VALUE": "1500"},
		{"NAME": parseTimeElapsedStat, "VALUE": "3000"},
		{"NAME": sortsDiskStat, "VALUE": "7"},
		{"NAME": sortsMemoryStat, "VALUE": "777"},
		{"NAME": sortsRowsStat, "VALUE": "888888"},
		{"NAME": sessionCursorCacheHitsStat, "VALUE": "65535"},
		{"NAME": sessionCursorCacheCountStat, "VALUE": "1024"},
		{"NAME": openedCursorsCurrentStat, "VALUE": "128"},
		{"NAME": userCallsStat, "VALUE": "987654"},
		{"NAME": recursiveCallsStat, "VALUE": "123456"},
		{"NAME": recursiveCPUUsageStat, "VALUE": "5000"},
		{"NAME": dbTimeStat, "VALUE": "60000"},
		// Buffer cache and DBWR v$sysstat rows
		{"NAME": dbBlockChanges, "VALUE": "8800000"},
		{"NAME": dbBlockGetsFromCache, "VALUE": "7700000"},
		{"NAME": dbwrCheckpointBuffersWritten, "VALUE": "12000"},
		{"NAME": dbwrCheckpoints, "VALUE": "320"},
		{"NAME": freeBufferRequested, "VALUE": "6100"},
		{"NAME": freeBufferInspected, "VALUE": "55000"},
		{"NAME": dirtyBuffersInspected, "VALUE": "1200"},
		// Redo log v$sysstat rows (redo time values are in centiseconds)
		{"NAME": redoWriteTime, "VALUE": "1500"},
		{"NAME": redoLogSpaceWaitTime, "VALUE": "250"},
		{"NAME": redoSynchTime, "VALUE": "900"},
		{"NAME": redoSize, "VALUE": "104857600"},
		{"NAME": redoWrites, "VALUE": "45000"},
		{"NAME": redoBlocksWritten, "VALUE": "210000"},
		{"NAME": redoBufferAllocRetries, "VALUE": "12"},
		{"NAME": redoLogSpaceRequests, "VALUE": "34"},
	},
	sessionCountSQL: {{"VALUE": "1"}},
	systemResourceLimitsSQL: {
		{"RESOURCE_NAME": "processes", "CURRENT_UTILIZATION": "3", "MAX_UTILIZATION": "10", "INITIAL_ALLOCATION": "100", "LIMIT_VALUE": "100"},
		{"RESOURCE_NAME": "locks", "CURRENT_UTILIZATION": "3", "MAX_UTILIZATION": "10", "INITIAL_ALLOCATION": "-1", "LIMIT_VALUE": "-1"},
	},
	tablespaceUsageSQL:  {{"TABLESPACE_NAME": "SYS", "USED_SPACE": "111288", "TABLESPACE_SIZE": "3518587", "BLOCK_SIZE": "8192"}},
	dataDictHitRatioSQL: {{"DATA_DICTIONARY_HIT_RATIO": "98.75"}},
	osStatSQL: {
		{"STAT_NAME": "NUM_CPUS", "VALUE": "8"},
		{"STAT_NAME": "LOAD", "VALUE": "1.5"},
		{"STAT_NAME": "PHYSICAL_MEMORY_BYTES", "VALUE": "17179869184"},
	},
	recycleBinSizeSQL: {{"RECYCLE_BIN_SIZE_BYTES": "13107200"}},
	sgaInfoSQL: {
		{"NAME": "Fixed SGA Size", "BYTES": "9292416"},
		{"NAME": "Redo Buffers", "BYTES": "14598144"},
		{"NAME": "Buffer Cache Size", "BYTES": "1375731712"},
		{"NAME": "Shared Pool Size", "BYTES": "536870912"},
		{"NAME": "Large Pool Size", "BYTES": "33554432"},
		{"NAME": "Java Pool Size", "BYTES": "0"},
		{"NAME": "Streams Pool Size", "BYTES": "0"},
		{"NAME": "Shared IO Pool Size", "BYTES": "134217728"},
		{"NAME": "Data Transfer Cache Size", "BYTES": "0"},
		{"NAME": "Granule Size", "BYTES": "16777216"},
		{"NAME": "Maximum SGA Size", "BYTES": "2147483648"},
		{"NAME": "Startup overhead in Shared Pool", "BYTES": "209715200"},
		{"NAME": "Free SGA Memory Available", "BYTES": "41943040"},
	},
	storageUsageSQL: {{"USED_DB_SIZE": "5368709120", "ALLOCATED_DB_SIZE": "10737418240"}},
	sysmetricSQL: {
		{"METRIC_NAME": "Buffer Cache Hit Ratio", "VALUE": "98.75"},
		{"METRIC_NAME": "Host CPU Utilization (%)", "VALUE": "12.34"},
		{"METRIC_NAME": "Database CPU Time Ratio", "VALUE": "55.66"},
		{"METRIC_NAME": "Library Cache Hit Ratio", "VALUE": "99.10"},
		{"METRIC_NAME": "Shared Pool Free %", "VALUE": "30.20"},
		{"METRIC_NAME": "Database Wait Time Ratio", "VALUE": "44.55"},
		{"METRIC_NAME": "Soft Parse Ratio", "VALUE": "88.90"},
		{"METRIC_NAME": "SQL Service Response Time", "VALUE": "0.0042"},
		{"METRIC_NAME": "Memory Sorts Ratio", "VALUE": "99.50"},
		{"METRIC_NAME": "Redo Allocation Hit Ratio", "VALUE": "97.80"},
		{"METRIC_NAME": "Parse Failure Count Per Sec", "VALUE": "0.25"},
		{"METRIC_NAME": "Execute Without Parse Ratio", "VALUE": "75.30"},
	},
}

var cacheValue = map[string]int64{
	"APPLICATION_WAIT_TIME":   0,
	"BUFFER_GETS":             3808197,
	"CLUSTER_WAIT_TIME":       1000,
	"CONCURRENCY_WAIT_TIME":   20,
	"CPU_TIME":                29821063,
	"DIRECT_READS":            3,
	"DIRECT_WRITES":           6,
	"DISK_READS":              12,
	"ELAPSED_TIME":            38172810,
	"EXECUTIONS":              200413,
	"PHYSICAL_READ_BYTES":     300,
	"PHYSICAL_READ_REQUESTS":  100,
	"PHYSICAL_WRITE_BYTES":    12,
	"PHYSICAL_WRITE_REQUESTS": 120,
	"ROWS_PROCESSED":          200413,
	"USER_IO_WAIT_TIME":       200,
	"PROCEDURE_EXECUTIONS":    200413,
}

const SQLPlanTable = "V$SQL_PLAN_STATISTICS_ALL"

func TestScraper_Scrape(t *testing.T) {
	tests := []struct {
		name       string
		dbclientFn func(db *sql.DB, s string, logger *zap.Logger) dbClient
		errWanted  string
	}{
		{
			name: "valid",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				return &fakeDbClient{
					Responses: [][]metricRow{
						queryResponses[s],
					},
				}
			},
		},
		{
			name: "bad tablespace usage",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == tablespaceUsageSQL {
					return &fakeDbClient{Responses: [][]metricRow{
						{
							{},
						},
					}}
				}
				return &fakeDbClient{Responses: [][]metricRow{
					queryResponses[s],
				}}
			},
			errWanted: `failed to parse int64 for OracledbTablespaceSizeUsage, value was : strconv.ParseInt: parsing "": invalid syntax`,
		},
		{
			name: "no limit on tablespace",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == tablespaceUsageSQL {
					return &fakeDbClient{Responses: [][]metricRow{
						{
							{"TABLESPACE_NAME": "SYS", "TABLESPACE_SIZE": "1024", "USED_SPACE": "111288", "BLOCK_SIZE": "8192"},
							{"TABLESPACE_NAME": "FOO", "TABLESPACE_SIZE": "", "USED_SPACE": "111288", "BLOCK_SIZE": "8192"},
						},
					}}
				}
				return &fakeDbClient{Responses: [][]metricRow{
					queryResponses[s],
				}}
			},
		},
		{
			name: "bad value on tablespace",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == tablespaceUsageSQL {
					return &fakeDbClient{Responses: [][]metricRow{
						{
							{"TABLESPACE_NAME": "SYS", "TABLESPACE_SIZE": "1024", "USED_SPACE": "111288", "BLOCK_SIZE": "8192"},
							{"TABLESPACE_NAME": "FOO", "TABLESPACE_SIZE": "ert", "USED_SPACE": "111288", "BLOCK_SIZE": "8192"},
						},
					}}
				}
				return &fakeDbClient{Responses: [][]metricRow{
					queryResponses[s],
				}}
			},
			errWanted: `failed to parse int64 for OracledbTablespaceSizeLimit, value was ert: strconv.ParseInt: parsing "ert": invalid syntax`,
		},
		{
			name: "Empty block size",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == tablespaceUsageSQL {
					return &fakeDbClient{Responses: [][]metricRow{
						{
							{"TABLESPACE_NAME": "SYS", "TABLESPACE_SIZE": "1024", "USED_SPACE": "111288", "BLOCK_SIZE": ""},
						},
					}}
				}
				return &fakeDbClient{Responses: [][]metricRow{
					queryResponses[s],
				}}
			},
			errWanted: `failed to parse int64 for OracledbBlockSize, value was : strconv.ParseInt: parsing "": invalid syntax`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := metadata.NewDefaultMetricsBuilderConfig()
			cfg.Metrics.OracledbConsistentGets.Enabled = true
			cfg.Metrics.OracledbDbBlockGets.Enabled = true

			scrpr := oracleScraper{
				logger: zap.NewNop(),
				mb:     metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
				dbProviderFunc: func() (*sql.DB, error) {
					return nil, nil
				},
				clientProviderFunc:   test.dbclientFn,
				id:                   component.ID{},
				metricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
			}
			err := scrpr.start(t.Context(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(t.Context()))
			}()
			require.NoError(t, err)
			m, err := scrpr.scrape(t.Context())
			if test.errWanted != "" {
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.EqualError(t, err, test.errWanted)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 18, m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
			}
			hostName, ok := m.ResourceMetrics().At(0).Resource().Attributes().Get("host.name")
			assert.True(t, ok)
			assert.Empty(t, hostName.Str())
			name, ok := m.ResourceMetrics().At(0).Resource().Attributes().Get("oracledb.instance.name")
			assert.True(t, ok)
			assert.Empty(t, name.Str())
			var found pmetric.Metric
			for i := 0; i < m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len(); i++ {
				metric := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(i)
				if metric.Name() == "oracledb.consistent_gets" {
					found = metric
					break
				}
			}
			assert.Equal(t, int64(78944), found.Sum().DataPoints().At(0).IntValue())
		})
	}
}

func TestScraper_ScrapeOperationalMetrics(t *testing.T) {
	const floatDelta = 0.01

	tests := []struct {
		name       string
		dbclientFn func(db *sql.DB, s string, logger *zap.Logger) dbClient
		errWanted  string
	}{
		{
			name: "valid operational metrics",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				return &fakeDbClient{
					Responses: [][]metricRow{
						queryResponses[s],
					},
				}
			},
		},
		{
			name: "bad data dictionary hit ratio value",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == dataDictHitRatioSQL {
					return &fakeDbClient{Responses: [][]metricRow{
						{{"DATA_DICTIONARY_HIT_RATIO": "not_a_number"}},
					}}
				}
				return &fakeDbClient{Responses: [][]metricRow{
					queryResponses[s],
				}}
			},
			errWanted: `failed to parse float64 for OracledbDataDictionaryHitRatio, value was not_a_number`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := metadata.NewDefaultMetricsBuilderConfig()
			cfg.Metrics.OracledbDataDictionaryHitRatio.Enabled = true
			cfg.Metrics.OracledbRecycleBinLimit.Enabled = true
			cfg.Metrics.OracledbStorageUsage.Enabled = true
			cfg.Metrics.OracledbStorageUtilization.Enabled = true

			scrpr := oracleScraper{
				logger: zap.NewNop(),
				mb:     metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
				dbProviderFunc: func() (*sql.DB, error) {
					return nil, nil
				},
				clientProviderFunc:   test.dbclientFn,
				id:                   component.ID{},
				metricsBuilderConfig: cfg,
			}
			err := scrpr.start(t.Context(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(t.Context()))
			}()
			require.NoError(t, err)
			m, err := scrpr.scrape(t.Context())
			if test.errWanted != "" {
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.Contains(t, err.Error(), test.errWanted)
			} else {
				require.NoError(t, err)
				metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

				metricMap := make(map[string]float64)
				for i := 0; i < metrics.Len(); i++ {
					metric := metrics.At(i)
					if metric.Type() == pmetric.MetricTypeGauge && metric.Gauge().DataPoints().Len() > 0 {
						metricMap[metric.Name()] = metric.Gauge().DataPoints().At(0).DoubleValue()
					}
				}
				assert.InDelta(t, 98.75, metricMap["oracledb.data_dictionary.hit_ratio"], floatDelta)
				assert.InDelta(t, 13107200.0, metricMap["oracledb.recycle_bin.limit"], floatDelta)
				assert.InDelta(t, 5368709120.0, metricMap["oracledb.storage.usage"], floatDelta)
				assert.InDelta(t, 0.5, metricMap["oracledb.storage.utilization"], floatDelta)
			}
		})
	}
}

func TestScraper_ScrapeOSStat(t *testing.T) {
	const floatDelta = 0.01

	tests := []struct {
		name       string
		dbclientFn func(db *sql.DB, s string, logger *zap.Logger) dbClient
		errWanted  string
	}{
		{
			name: "valid os stat metrics",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				return &fakeDbClient{
					Responses: [][]metricRow{
						queryResponses[s],
					},
				}
			},
		},
		{
			name: "bad NUM_CPUS value",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == osStatSQL {
					return &fakeDbClient{Responses: [][]metricRow{
						{{"STAT_NAME": "NUM_CPUS", "VALUE": "bad"}},
					}}
				}
				return &fakeDbClient{Responses: [][]metricRow{queryResponses[s]}}
			},
			errWanted: `failed to parse int64 for OracledbSystemCPUCount, value was bad`,
		},
		{
			name: "bad LOAD value",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == osStatSQL {
					return &fakeDbClient{Responses: [][]metricRow{
						{{"STAT_NAME": "LOAD", "VALUE": "bad"}},
					}}
				}
				return &fakeDbClient{Responses: [][]metricRow{queryResponses[s]}}
			},
			errWanted: `failed to parse float64 for OracledbSystemProcessCount, value was bad`,
		},
		{
			name: "bad PHYSICAL_MEMORY_BYTES value",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == osStatSQL {
					return &fakeDbClient{Responses: [][]metricRow{
						{{"STAT_NAME": "PHYSICAL_MEMORY_BYTES", "VALUE": "bad"}},
					}}
				}
				return &fakeDbClient{Responses: [][]metricRow{queryResponses[s]}}
			},
			errWanted: `failed to parse int64 for OracledbSystemMemoryLimit, value was bad`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := metadata.NewDefaultMetricsBuilderConfig()
			cfg.Metrics.OracledbSystemCPUCount.Enabled = true
			cfg.Metrics.OracledbSystemMemoryLimit.Enabled = true
			cfg.Metrics.OracledbSystemProcessCount.Enabled = true

			scrpr := oracleScraper{
				logger: zap.NewNop(),
				mb:     metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
				dbProviderFunc: func() (*sql.DB, error) {
					return nil, nil
				},
				clientProviderFunc:   test.dbclientFn,
				id:                   component.ID{},
				metricsBuilderConfig: cfg,
			}
			err := scrpr.start(t.Context(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(t.Context()))
			}()
			require.NoError(t, err)
			m, err := scrpr.scrape(t.Context())
			if test.errWanted != "" {
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.Contains(t, err.Error(), test.errWanted)
			} else {
				require.NoError(t, err)
				metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				for i := 0; i < metrics.Len(); i++ {
					metric := metrics.At(i)
					if metric.Type() != pmetric.MetricTypeGauge || metric.Gauge().DataPoints().Len() == 0 {
						continue
					}
					dp := metric.Gauge().DataPoints().At(0)
					switch metric.Name() {
					case "oracledb.system.cpu.count":
						assert.Equal(t, int64(8), dp.IntValue())
					case "oracledb.system.memory.limit":
						assert.Equal(t, int64(17179869184), dp.IntValue())
					case "oracledb.system.process.count":
						assert.InDelta(t, 1.5, dp.DoubleValue(), floatDelta)
					}
				}
			}
		})
	}
}

// scrapeWithConfig starts a scraper over the shared fake responses and returns one scrape.
func scrapeWithConfig(t *testing.T, cfg metadata.MetricsBuilderConfig) pmetric.Metrics {
	t.Helper()
	scrpr := oracleScraper{
		logger: zap.NewNop(),
		mb:     metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, nil
		},
		clientProviderFunc: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
			return &fakeDbClient{Responses: [][]metricRow{queryResponses[s]}}
		},
		id:                   component.ID{},
		metricsBuilderConfig: cfg,
	}
	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { assert.NoError(t, scrpr.shutdown(t.Context())) })
	m, err := scrpr.scrape(t.Context())
	require.NoError(t, err)
	return m
}

func TestScraper_ScrapeIOPerformanceMetrics(t *testing.T) {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.OracledbPhysicalIoCacheWrites.Enabled = true
	cfg.Metrics.OracledbPhysicalIoRequests.Enabled = true
	cfg.Metrics.OracledbPhysicalIoTransferred.Enabled = true
	cfg.Metrics.OracledbSqlnetIoTransferred.Enabled = true

	m := scrapeWithConfig(t, cfg)

	// metricName -> attrSignature -> int value
	got := map[string]map[string]int64{}
	metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < metrics.Len(); i++ {
		me := metrics.At(i)
		if me.Type() != pmetric.MetricTypeSum {
			continue
		}
		dps := me.Sum().DataPoints()
		for j := 0; j < dps.Len(); j++ {
			dp := dps.At(j)
			var keys []string
			dp.Attributes().Range(func(k string, v pcommon.Value) bool {
				keys = append(keys, k+"="+v.AsString())
				return true
			})
			sort.Strings(keys)
			sig := strings.Join(keys, ",")
			if _, ok := got[me.Name()]; !ok {
				got[me.Name()] = map[string]int64{}
			}
			got[me.Name()][sig] = dp.IntValue()
		}
	}

	assert.Equal(t, int64(1024000), got["oracledb.physical_io.transferred"]["disk.io.direction=read,disk.io.type=buffered"])
	assert.Equal(t, int64(512000), got["oracledb.physical_io.transferred"]["disk.io.direction=write,disk.io.type=buffered"])
	assert.Equal(t, int64(2048000), got["oracledb.physical_io.transferred"]["disk.io.direction=read,disk.io.type=total"])
	assert.Equal(t, int64(1024000), got["oracledb.physical_io.transferred"]["disk.io.direction=write,disk.io.type=total"])

	assert.Equal(t, int64(5000), got["oracledb.physical_io.requests"]["disk.io.block_size=all,disk.io.direction=read"])
	assert.Equal(t, int64(2500), got["oracledb.physical_io.requests"]["disk.io.block_size=all,disk.io.direction=write"])
	assert.Equal(t, int64(1500), got["oracledb.physical_io.requests"]["disk.io.block_size=multi,disk.io.direction=read"])
	assert.Equal(t, int64(750), got["oracledb.physical_io.requests"]["disk.io.block_size=multi,disk.io.direction=write"])

	assert.Equal(t, int64(1800), got["oracledb.physical_io.cache_writes"][""])

	assert.Equal(t, int64(300000), got["oracledb.sqlnet.io.transferred"]["destination.type=client,network.io.direction=receive"])
	assert.Equal(t, int64(600000), got["oracledb.sqlnet.io.transferred"]["destination.type=client,network.io.direction=transmit"])
	assert.Equal(t, int64(150000), got["oracledb.sqlnet.io.transferred"]["destination.type=dblink,network.io.direction=receive"])
	assert.Equal(t, int64(75000), got["oracledb.sqlnet.io.transferred"]["destination.type=dblink,network.io.direction=transmit"])
}

func TestScraper_ScrapeWorkloadAnalysisMetrics(t *testing.T) {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.OracledbCallCount.Enabled = true
	cfg.Metrics.OracledbCallRecursiveCPUTime.Enabled = true
	cfg.Metrics.OracledbCursorCacheHits.Enabled = true
	cfg.Metrics.OracledbCursorCacheSize.Enabled = true
	cfg.Metrics.OracledbCursorOpen.Enabled = true
	cfg.Metrics.OracledbDbTime.Enabled = true
	cfg.Metrics.OracledbEnqueueOperations.Enabled = true
	cfg.Metrics.OracledbLobOperations.Enabled = true
	cfg.Metrics.OracledbParseCPUTime.Enabled = true
	cfg.Metrics.OracledbParseElapsedTime.Enabled = true
	cfg.Metrics.OracledbScanCount.Enabled = true
	cfg.Metrics.OracledbScanTableRows.Enabled = true
	cfg.Metrics.OracledbSortOperations.Enabled = true
	cfg.Metrics.OracledbSortRows.Enabled = true

	actualMetrics := scrapeWithConfig(t, cfg)

	expectedFile := filepath.Join("testdata", "expectedWorkloadAnalysisMetrics.yaml")
	// Uncomment the line below to regenerate the expected golden file.
	// require.NoError(t, golden.WriteMetrics(t, expectedFile, actualMetrics))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp()))
}

// Disabling oracledb.enqueue.type collapses its five per-type rows into one summed data point (165).
func TestScraper_WorkloadMetricReaggregatesWhenAttributeDisabled(t *testing.T) {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.OracledbEnqueueOperations.Enabled = true
	cfg.Metrics.OracledbEnqueueOperations.EnabledAttributes = nil

	m := scrapeWithConfig(t, cfg)

	var found bool
	metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < metrics.Len(); i++ {
		me := metrics.At(i)
		if me.Name() != "oracledb.enqueue.operations" {
			continue
		}
		found = true
		dps := me.Sum().DataPoints()
		require.Equal(t, 1, dps.Len(), "disabling the attribute should collapse all types into one data point")
		dp := dps.At(0)
		assert.Equal(t, 0, dp.Attributes().Len(), "no attribute should be emitted when oracledb.enqueue.type is disabled")
		assert.Equal(t, int64(165), dp.IntValue(), "data points should reaggregate by sum (11+22+33+44+55)")
	}
	require.True(t, found, "oracledb.enqueue.operations not found in scrape output")
}

// Disabling oracledb.scan.mode (keeping oracledb.scan.type) reaggregates per type: table=350, index_fast_full=35.
func TestScraper_ScanCountReaggregatesWhenOneAttributeDisabled(t *testing.T) {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.OracledbScanCount.Enabled = true
	cfg.Metrics.OracledbScanCount.EnabledAttributes = []metadata.OracledbScanCountMetricAttributeKey{
		metadata.OracledbScanCountMetricAttributeKeyOracledbScanType,
	}

	m := scrapeWithConfig(t, cfg)

	want := map[string]int64{"table": 350, "index_fast_full": 35}
	got := map[string]int64{}
	var found bool
	metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < metrics.Len(); i++ {
		me := metrics.At(i)
		if me.Name() != "oracledb.scan.count" {
			continue
		}
		found = true
		dps := me.Sum().DataPoints()
		for j := 0; j < dps.Len(); j++ {
			dp := dps.At(j)
			require.Equal(t, 1, dp.Attributes().Len(), "only oracledb.scan.type should remain")
			scanType, ok := dp.Attributes().Get("oracledb.scan.type")
			require.True(t, ok, "oracledb.scan.type should be present")
			got[scanType.Str()] = dp.IntValue()
		}
	}
	require.True(t, found, "oracledb.scan.count not found in scrape output")
	assert.Equal(t, want, got, "scan modes should reaggregate by sum within each scan type")
}

func TestScraper_ScrapeBufferAndCheckpointMetrics(t *testing.T) {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.OracledbBufferCacheBlockChanges.Enabled = true
	cfg.Metrics.OracledbBufferCacheBlockGets.Enabled = true
	cfg.Metrics.OracledbBufferInspected.Enabled = true
	cfg.Metrics.OracledbCheckpointBuffers.Enabled = true
	cfg.Metrics.OracledbCheckpointCompleted.Enabled = true
	cfg.Metrics.OracledbBufferRequests.Enabled = true

	scrpr := oracleScraper{
		logger: zap.NewNop(),
		mb:     metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, nil
		},
		clientProviderFunc: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
			return &fakeDbClient{Responses: [][]metricRow{queryResponses[s]}}
		},
		id:                   component.ID{},
		metricsBuilderConfig: cfg,
	}
	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() { assert.NoError(t, scrpr.shutdown(t.Context())) }()

	m, err := scrpr.scrape(t.Context())
	require.NoError(t, err)

	// Loop the scraped metrics and switch on the metric name; assert each metric's value(s) and
	// attribute(s) in place, using the mdatagen-generated name/attribute constants.
	metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	seen := 0
	for i := 0; i < metrics.Len(); i++ {
		me := metrics.At(i)
		if me.Type() != pmetric.MetricTypeSum {
			continue
		}
		dps := me.Sum().DataPoints()
		switch me.Name() {
		case metadata.MetricsInfo.OracledbBufferCacheBlockChanges.Name:
			assert.Equal(t, int64(8800000), dps.At(0).IntValue())
			seen++
		case metadata.MetricsInfo.OracledbBufferCacheBlockGets.Name:
			assert.Equal(t, int64(7700000), dps.At(0).IntValue())
			seen++
		case metadata.MetricsInfo.OracledbCheckpointBuffers.Name:
			assert.Equal(t, int64(12000), dps.At(0).IntValue())
			seen++
		case metadata.MetricsInfo.OracledbCheckpointCompleted.Name:
			assert.Equal(t, int64(320), dps.At(0).IntValue())
			seen++
		case metadata.MetricsInfo.OracledbBufferRequests.Name:
			assert.Equal(t, int64(6100), dps.At(0).IntValue())
			seen++
		case metadata.MetricsInfo.OracledbBufferInspected.Name:
			// one data point per buffer state.
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				state, _ := dp.Attributes().Get("oracledb.buffer.state")
				switch state.Str() {
				case metadata.AttributeOracledbBufferStateFree.String():
					assert.Equal(t, int64(55000), dp.IntValue())
				case metadata.AttributeOracledbBufferStateDirty.String():
					assert.Equal(t, int64(1200), dp.IntValue())
				default:
					t.Errorf("unexpected oracledb.buffer.state: %q", state.Str())
				}
			}
			seen++
		}
	}
	assert.Equal(t, 6, seen, "expected all 6 buffer/checkpoint metrics to be emitted")
}

func TestScraper_ScrapeRedoMetrics(t *testing.T) {
	const floatDelta = 0.0001

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.OracledbRedoTime.Enabled = true
	cfg.Metrics.OracledbRedoSize.Enabled = true
	cfg.Metrics.OracledbRedoOperations.Enabled = true
	cfg.Metrics.OracledbRedoBlocks.Enabled = true
	cfg.Metrics.OracledbRedoRequests.Enabled = true
	cfg.Metrics.OracledbRedoRetries.Enabled = true

	scrpr := oracleScraper{
		logger: zap.NewNop(),
		mb:     metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, nil
		},
		clientProviderFunc: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
			return &fakeDbClient{Responses: [][]metricRow{queryResponses[s]}}
		},
		id:                   component.ID{},
		metricsBuilderConfig: cfg,
	}
	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() { assert.NoError(t, scrpr.shutdown(t.Context())) }()

	m, err := scrpr.scrape(t.Context())
	require.NoError(t, err)

	metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	seen := 0
	for i := 0; i < metrics.Len(); i++ {
		me := metrics.At(i)
		if me.Type() != pmetric.MetricTypeSum {
			continue
		}
		dps := me.Sum().DataPoints()
		switch me.Name() {
		case metadata.MetricsInfo.OracledbRedoTime.Name:
			// centiseconds converted to seconds (value / 100), one data point per redo.type.
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				redoType, _ := dp.Attributes().Get("oracledb.redo.type")
				switch redoType.Str() {
				case metadata.AttributeOracledbRedoTypeWrite.String():
					assert.InDelta(t, 15.0, dp.DoubleValue(), floatDelta)
				case metadata.AttributeOracledbRedoTypeLogSpaceWait.String():
					assert.InDelta(t, 2.5, dp.DoubleValue(), floatDelta)
				case metadata.AttributeOracledbRedoTypeSync.String():
					assert.InDelta(t, 9.0, dp.DoubleValue(), floatDelta)
				default:
					t.Errorf("unexpected oracledb.redo.type: %q", redoType.Str())
				}
			}
			seen++
		case metadata.MetricsInfo.OracledbRedoSize.Name:
			assert.Equal(t, int64(104857600), dps.At(0).IntValue())
			seen++
		case metadata.MetricsInfo.OracledbRedoOperations.Name:
			dp := dps.At(0)
			dir, _ := dp.Attributes().Get("disk.io.direction")
			assert.Equal(t, metadata.AttributeDiskIoDirectionWrite.String(), dir.Str())
			assert.Equal(t, int64(45000), dp.IntValue())
			seen++
		case metadata.MetricsInfo.OracledbRedoBlocks.Name:
			dp := dps.At(0)
			dir, _ := dp.Attributes().Get("disk.io.direction")
			assert.Equal(t, metadata.AttributeDiskIoDirectionWrite.String(), dir.Str())
			assert.Equal(t, int64(210000), dp.IntValue())
			seen++
		case metadata.MetricsInfo.OracledbRedoRequests.Name:
			dp := dps.At(0)
			reqType, _ := dp.Attributes().Get("oracledb.redo.request.type")
			assert.Equal(t, metadata.AttributeOracledbRedoRequestTypeLogSpace.String(), reqType.Str())
			assert.Equal(t, int64(34), dp.IntValue())
			seen++
		case metadata.MetricsInfo.OracledbRedoRetries.Name:
			dp := dps.At(0)
			retryType, _ := dp.Attributes().Get("oracledb.redo.retry.type")
			assert.Equal(t, metadata.AttributeOracledbRedoRetryTypeBufferAllocation.String(), retryType.Str())
			assert.Equal(t, int64(12), dp.IntValue())
			seen++
		}
	}
	assert.Equal(t, 6, seen, "expected all 6 redo metrics to be emitted")
}

func TestScraper_ScrapeTopNLogs(t *testing.T) {
	var metricRowData []metricRow
	var logRowData []metricRow
	tests := []struct {
		name       string
		dbclientFn func(db *sql.DB, s string, logger *zap.Logger) dbClient
		errWanted  string
	}{
		{
			name: "valid collection",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if strings.Contains(s, SQLPlanTable) {
					metricRowFile := readFile("oracleQueryPlanData.txt")
					unmarshalErr := json.Unmarshal(metricRowFile, &logRowData)
					if unmarshalErr == nil {
						return &fakeDbClient{
							Responses: [][]metricRow{
								logRowData,
							},
						}
					}
				} else {
					metricRowFile := readFile("oracleQueryMetricsData.txt")
					unmarshalErr := json.Unmarshal(metricRowFile, &metricRowData)
					if unmarshalErr == nil {
						return &fakeDbClient{
							Responses: [][]metricRow{
								metricRowData,
							},
						}
					}
				}
				return nil
			},
		}, {
			name: "No metrics collected",
			dbclientFn: func(_ *sql.DB, _ string, _ *zap.Logger) dbClient {
				return &fakeDbClient{
					Responses: [][]metricRow{
						nil,
					},
				}
			},
			errWanted: `no data returned from oracleQueryMetricsClient`,
		}, {
			name: "Error on collecting metrics",
			dbclientFn: func(_ *sql.DB, _ string, _ *zap.Logger) dbClient {
				return &fakeDbClient{
					Responses: [][]metricRow{
						nil,
					},
					Err: errors.New("Mock error"),
				}
			},
			errWanted: "error executing oracleQueryMetricsSQL: Mock error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logsCfg := metadata.DefaultLogsBuilderConfig()
			logsCfg.ResourceAttributes.HostName.Enabled = true
			logsCfg.Events.DbServerTopQuery.Enabled = true
			metricsCfg := metadata.NewDefaultMetricsBuilderConfig()
			lruCache, _ := lru.New[string, map[string]int64](500)
			lruCache.Add("fxk8aq3nds8aw:0", cacheValue)

			scrpr := oracleScraper{
				logger: zap.NewNop(),
				mb:     metadata.NewMetricsBuilder(metricsCfg, receivertest.NewNopSettings(metadata.Type)),
				lb:     metadata.NewLogsBuilder(logsCfg, receivertest.NewNopSettings(metadata.Type)),
				dbProviderFunc: func() (*sql.DB, error) {
					return nil, nil
				},
				clientProviderFunc:   test.dbclientFn,
				id:                   component.ID{},
				metricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				logsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
				metricCache:          lruCache,
				topQueryCollectCfg:   TopQueryCollection{MaxQuerySampleCount: 5000, TopQueryCount: 200},
				instanceName:         "oraclehost:1521/ORCL",
				hostName:             "oraclehost:1521",
				obfuscator:           newObfuscator(),
				serviceInstanceID:    getInstanceID("oraclehost:1521/ORCL", zap.NewNop()),
			}

			scrpr.logsBuilderConfig.Events.DbServerTopQuery.Enabled = true

			err := scrpr.start(t.Context(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(t.Context()))
			}()
			require.NoError(t, err)
			expectedQueryPlanFile := filepath.Join("testdata", "expectedQueryTextAndPlanQuery.yaml")
			assert.True(t, scrpr.lastExecutionTimestamp.IsZero(), "No value exists on lastExecutionTimestamp before any collection.")

			logs, err := scrpr.scrapeLogs(t.Context())

			if test.errWanted != "" {
				require.EqualError(t, err, test.errWanted)
			} else {
				// Uncomment line below to re-generate expected logs.
				// golden.WriteLogs(t, expectedQueryPlanFile, logs)
				expectedLogs, _ := golden.ReadLogs(expectedQueryPlanFile)
				errs := plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreTimestamp())
				assert.NoError(t, errs)
				assert.Equal(t, "db.server.top_query", logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
				assert.NoError(t, errs)

				assert.False(t, scrpr.lastExecutionTimestamp.IsZero(), "lastExecutionTimestamp hasn't set after a successful collection.")
			}
		})
	}
}

var samplesQueryResponses = map[string][]metricRow{
	samplesQuery: {{
		"ACTION": "00-0af7651916cd43dd8448eb211c80319c-a7ad6b7169203331-01", "MACHINE": "TEST-MACHINE", "USERNAME": "ADMIN", "SCHEMANAME": "ADMIN", "SQL_ID": "48bc50b6fuz4y", "WAIT_CLASS": "ONE", "WAIT_TIME_SEC": "0.5", "PROCEDURE_NAME": "BLAH", "CHILD_ADDRESS": "SDF3SDF1234D",
		"SQL_CHILD_NUMBER": "0", "SID": "675", "SERIAL#": "51295", "SQL_FULLTEXT": "test_query", "OSUSER": "test-user", "PROCESS": "1115", "PROCEDURE_TYPE": "PROCEDURE_TYPE-A", "PROCEDURE_ID": "12345",
		"PORT": "54440", "PROGRAM": "Oracle SQL Developer for VS Code", "MODULE": "Oracle SQL Developer for VS Code", "STATUS": "ACTIVE", "STATE": "WAITED KNOWN TIME", "PLAN_HASH_VALUE": "4199919568", "DURATION_SEC": "1", "SERVICE_NAME": "", "DB_NAMESPACE": "",
		"SQL_EXEC_START": "2026-01-01T12:00:00Z", "LOGON_TIME": "2026-01-01T12:00:00Z", "SESSION_DURATION_SEC": "0",
		"BLOCKING_SESSION": "", "FINAL_BLOCKING_SESSION": "", "BLOCKING_SESSION_STATUS": "", "SECONDS_IN_WAIT": "0",
		"BLOCKING_START_TIME": "", "LOCK_TYPE": "", "LOCK_MODE": "", "BLOCKED_OBJECT_OWNER": "", "BLOCKED_OBJECT_NAME": "",
	}},
	"invalidQuery": {{
		"MACHINE": "TEST-MACHINE", "USERNAME": "ADMIN", "SCHEMANAME": "ADMIN", "SQL_ID": "48bc50b6fuz4y",
		"SQL_CHILD_NUMBER": "0", "S.SID": "675", "SERIAL#": "51295", "SQL_FULLTEXT": "test_query", "OSUSER": "test-user", "PROCESS": "1115",
		"PORT": "54440", "PROGRAM": "Oracle SQL Developer for VS Code", "MODULE": "Oracle SQL Developer for VS Code", "STATUS": "ACTIVE", "STATE": "WAITED KNOWN TIME", "PLAN_HASH_VALUE": "4199919568", "DURATION_SEC": "",
		"SQL_EXEC_START": "", "LOGON_TIME": "", "SESSION_DURATION_SEC": "0",
		"BLOCKING_SESSION": "", "FINAL_BLOCKING_SESSION": "", "BLOCKING_SESSION_STATUS": "", "SECONDS_IN_WAIT": "0",
		"BLOCKING_START_TIME": "", "LOCK_TYPE": "", "LOCK_MODE": "", "BLOCKED_OBJECT_OWNER": "", "BLOCKED_OBJECT_NAME": "",
	}},
	"blockedQuery": {{
		// Blocked session waiting on SID 100
		"ACTION": "", "MACHINE": "DB-CLIENT-HOST", "USERNAME": "APP_USER", "SCHEMANAME": "APP_USER", "SQL_ID": "9fkq2mxyzabc1", "WAIT_CLASS": "Application", "PROCEDURE_NAME": "", "CHILD_ADDRESS": "ABCD1234",
		"SQL_CHILD_NUMBER": "0", "SID": "200", "SERIAL#": "12345", "SQL_FULLTEXT": "UPDATE orders SET status = 1 WHERE id = 42", "OSUSER": "oracle", "PROCESS": "9876", "PROCEDURE_TYPE": "", "PROCEDURE_ID": "",
		"PORT": "54441", "PROGRAM": "JDBC Thin Client", "MODULE": "app", "STATUS": "ACTIVE", "STATE": "WAITING", "PLAN_HASH_VALUE": "1234567890", "DURATION_SEC": "15", "SERVICE_NAME": "ORCL", "DB_NAMESPACE": "ORCLPDB1",
		"SQL_EXEC_START": "2026-05-06T09:59:45Z", "LOGON_TIME": "2026-05-06T09:00:00Z", "SESSION_DURATION_SEC": "3600",
		"BLOCKING_SESSION": "100", "FINAL_BLOCKING_SESSION": "100", "BLOCKING_SESSION_STATUS": "VALID", "SECONDS_IN_WAIT": "15",
		"BLOCKING_START_TIME": "2026-05-06T10:00:00Z", "LOCK_TYPE": "TX", "LOCK_MODE": "EXCLUSIVE", "BLOCKED_OBJECT_OWNER": "APP_USER", "BLOCKED_OBJECT_NAME": "ORDERS",
	}},
	"idleBlockerQuery": {{
		// Idle session (blocker) that is holding a lock — no longer ACTIVE but appearing in BLOCKING_SESSION subquery
		"ACTION": "", "MACHINE": "DBA-WORKSTATION", "USERNAME": "DBA_USER", "SCHEMANAME": "DBA_USER", "SQL_ID": "7abc123def456", "WAIT_CLASS": "", "PROCEDURE_NAME": "", "CHILD_ADDRESS": "DEADBEEF",
		"SQL_CHILD_NUMBER": "0", "SID": "100", "SERIAL#": "5678", "SQL_FULLTEXT": "UPDATE orders SET status = 2 WHERE id = 42", "OSUSER": "dba", "PROCESS": "1234", "PROCEDURE_TYPE": "", "PROCEDURE_ID": "",
		"PORT": "54442", "PROGRAM": "SQL*Plus", "MODULE": "", "STATUS": "INACTIVE", "STATE": "WAITED KNOWN TIME", "PLAN_HASH_VALUE": "9876543210", "DURATION_SEC": "120", "SERVICE_NAME": "ORCL", "DB_NAMESPACE": "ORCLPDB1",
		"SQL_EXEC_START": "", "LOGON_TIME": "", "SESSION_DURATION_SEC": "0",
		"BLOCKING_SESSION": "", "FINAL_BLOCKING_SESSION": "", "BLOCKING_SESSION_STATUS": "", "SECONDS_IN_WAIT": "0",
		"BLOCKING_START_TIME": "", "LOCK_TYPE": "", "LOCK_MODE": "", "BLOCKED_OBJECT_OWNER": "", "BLOCKED_OBJECT_NAME": "",
	}},
}

var sessionEventQueryResponses = map[string][]metricRow{
	sessionEventQuery: {
		{
			"SID": "100", "SERIAL#": "12345", "EVENT": "db file sequential read", "WAIT_CLASS": "User I/O",
			"TOTAL_WAITS": "1500", "TOTAL_TIME_WAITED_SECS": "25.5", "DB_NAMESPACE": "ORCL",
		},
		{
			"SID": "101", "SERIAL#": "12346", "EVENT": "log file sync", "WAIT_CLASS": "Commit",
			"TOTAL_WAITS": "800", "TOTAL_TIME_WAITED_SECS": "12.3", "DB_NAMESPACE": "ORCL",
		},
		{
			"SID": "102", "SERIAL#": "12347", "EVENT": "buffer busy waits", "WAIT_CLASS": "Concurrency",
			"TOTAL_WAITS": "50", "TOTAL_TIME_WAITED_SECS": "1.5",
		},
	},
	"invalidSessionEventQuery": {
		{
			"SID": "100", "SERIAL#": "12345", "EVENT": "db file sequential read", "WAIT_CLASS": "User I/O",
			"TOTAL_WAITS": "invalid", "TOTAL_TIME_WAITED_SECS": "25.5", "DB_NAMESPACE": "ORCL",
		},
	},
	"invalidTimeSessionEventQuery": {
		{
			"SID": "100", "SERIAL#": "12345", "EVENT": "db file sequential read", "WAIT_CLASS": "User I/O",
			"TOTAL_WAITS": "1500", "TOTAL_TIME_WAITED_SECS": "invalid", "DB_NAMESPACE": "ORCL",
		},
	},
}

func TestSamplesQuery(t *testing.T) {
	tests := []struct {
		name              string
		dbclientFn        func(db *sql.DB, s string, logger *zap.Logger) dbClient
		errWanted         string
		goldenFile        string
		checkBlockingAttr bool
	}{
		{
			name: "valid",
			dbclientFn: func(_ *sql.DB, _ string, _ *zap.Logger) dbClient {
				return &fakeDbClient{
					Responses: [][]metricRow{
						samplesQueryResponses[samplesQuery],
					},
				}
			},
			goldenFile: filepath.Join("testdata", "expectedSamplesFile.yaml"),
		},
		{
			name: "bad samples data",
			dbclientFn: func(_ *sql.DB, _ string, _ *zap.Logger) dbClient {
				return &fakeDbClient{Responses: [][]metricRow{
					samplesQueryResponses["invalidQuery"],
				}}
			},
			errWanted: `failed to parse int64 for Duration, value was : strconv.ParseFloat: parsing "": invalid syntax`,
		},
		{
			name: "blocked session emits blocking attributes",
			dbclientFn: func(_ *sql.DB, _ string, _ *zap.Logger) dbClient {
				return &fakeDbClient{Responses: [][]metricRow{
					samplesQueryResponses["blockedQuery"],
				}}
			},
			goldenFile:        filepath.Join("testdata", "expectedBlockedSessionFile.yaml"),
			checkBlockingAttr: true,
		},
		{
			name: "idle blocker session emits empty blocking attributes",
			dbclientFn: func(_ *sql.DB, _ string, _ *zap.Logger) dbClient {
				return &fakeDbClient{Responses: [][]metricRow{
					samplesQueryResponses["idleBlockerQuery"],
				}}
			},
			goldenFile: filepath.Join("testdata", "expectedIdleBlockerFile.yaml"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logsCfg := metadata.DefaultLogsBuilderConfig()
			logsCfg.ResourceAttributes.OracledbInstanceName.Enabled = true
			logsCfg.ResourceAttributes.HostName.Enabled = true
			logsCfg.Events.DbServerTopQuery.Enabled = false
			logsCfg.Events.DbServerQuerySample.Enabled = true
			scrpr := oracleScraper{
				logger: zap.NewNop(),
				dbProviderFunc: func() (*sql.DB, error) {
					return nil, nil
				},
				clientProviderFunc: test.dbclientFn,
				id:                 component.ID{},
				lb:                 metadata.NewLogsBuilder(logsCfg, receivertest.NewNopSettings(metadata.Type)),
				logsBuilderConfig:  metadata.DefaultLogsBuilderConfig(),
				obfuscator:         newObfuscator(),
				instanceName:       "oraclehost:1521/ORCL",
				serviceInstanceID:  getInstanceID("oraclehost:1521/ORCL", zap.NewNop()),
			}
			scrpr.logsBuilderConfig.Events.DbServerTopQuery.Enabled = false
			scrpr.logsBuilderConfig.Events.DbServerQuerySample.Enabled = true
			err := scrpr.start(t.Context(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(t.Context()))
			}()
			require.NoError(t, err)
			logs, err := scrpr.scrapeLogs(t.Context())

			if test.errWanted != "" {
				require.EqualError(t, err, test.errWanted)
				return
			}

			require.NoError(t, err)
			require.Positive(t, logs.ResourceLogs().Len())
			assert.Equal(t, "db.server.query_sample", logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())

			if test.checkBlockingAttr {
				lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
				blockerSID, hasBlockerSID := lr.Attributes().Get("oracledb.blocking.blocker.sid")
				assert.True(t, hasBlockerSID, "blocking.blocker.sid attribute must be present for a blocked session")
				assert.Equal(t, "100", blockerSID.Str())

				_, hasBlockerState := lr.Attributes().Get("oracledb.blocking.blocker.state")
				assert.True(t, hasBlockerState, "blocking.blocker.state attribute must be present for a blocked session")

				waitDuration, hasWaitDuration := lr.Attributes().Get("oracledb.blocking.wait_duration")
				assert.True(t, hasWaitDuration, "blocking.wait_duration attribute must be present for a blocked session")
				assert.Equal(t, int64(15), waitDuration.Int())
			}

			// Uncomment line below to re-generate expected golden files.
			// golden.WriteLogs(t, test.goldenFile, logs)
			if test.goldenFile != "" {
				expectedLogs, readErr := golden.ReadLogs(test.goldenFile)
				require.NoError(t, readErr)
				errs := plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreTimestamp())
				assert.NoError(t, errs)
			}
		})
	}
}

func TestScraperWithQueryComments(t *testing.T) {
	sql := "/* application=test-123 */ SELECT * FROM test_table"

	tests := []struct {
		name        string
		allowedKeys []string
		want        string
	}{
		{
			name:        "configured allowed keys are extracted",
			allowedKeys: []string{"application"},
			want:        "application=test-123",
		},
		{
			name:        "empty allowlist extracts nothing",
			allowedKeys: []string{},
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.QuerySample.AllowedCommentKeys = tt.allowedKeys

			got := sqlcomments.ExtractAndFilterComments(sql, cfg.QuerySample.AllowedCommentKeys)
			assert.Equal(t, tt.want, got)
		})
	}
	t.Run("query samples without allowed comments", func(t *testing.T) {
		// Create a mock scraper with empty allowed comment keys
		cfg := createDefaultConfig().(*Config)
		cfg.QuerySample.AllowedCommentKeys = []string{}

		sqlWithComment := "/* nr_service_guid=test-123 */ SELECT * FROM test_table"
		result := sqlcomments.ExtractAndFilterComments(sqlWithComment, cfg.QuerySample.AllowedCommentKeys)

		if result != "" {
			t.Errorf("Expected empty string but got %q", result)
		}
	})

	t.Run("query samples with non-matching comments", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.QuerySample.AllowedCommentKeys = []string{"nr_service_guid"}

		// SQL has comments but none match the allowlist
		sqlWithComment := "/* other_key=value */ SELECT * FROM test_table"
		result := sqlcomments.ExtractAndFilterComments(sqlWithComment, cfg.QuerySample.AllowedCommentKeys)

		if result != "" {
			t.Errorf("Expected empty string but got %q", result)
		}
	})
}

func TestSessionWaitEventsQuery(t *testing.T) {
	tests := []struct {
		name       string
		dbclientFn func(db *sql.DB, s string, logger *zap.Logger) dbClient
		errWanted  string
	}{
		{
			name: "valid",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				return &fakeDbClient{
					Responses: [][]metricRow{
						sessionEventQueryResponses[s],
					},
				}
			},
		},
		{
			name: "bad wait.count data",
			dbclientFn: func(_ *sql.DB, _ string, _ *zap.Logger) dbClient {
				return &fakeDbClient{Responses: [][]metricRow{
					sessionEventQueryResponses["invalidSessionEventQuery"],
				}}
			},
			errWanted: `failed to parse int64 for oracledb.wait.count, value was invalid: strconv.ParseInt: parsing "invalid": invalid syntax`,
		},
		{
			name: "bad wait.duration data",
			dbclientFn: func(_ *sql.DB, _ string, _ *zap.Logger) dbClient {
				return &fakeDbClient{Responses: [][]metricRow{
					sessionEventQueryResponses["invalidTimeSessionEventQuery"],
				}}
			},
			errWanted: `failed to parse float64 for oracledb.wait.duration, value was invalid: strconv.ParseFloat: parsing "invalid": invalid syntax`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logsCfg := metadata.DefaultLogsBuilderConfig()
			logsCfg.ResourceAttributes.OracledbInstanceName.Enabled = true
			logsCfg.ResourceAttributes.HostName.Enabled = true
			logsCfg.Events.DbServerTopQuery.Enabled = false
			logsCfg.Events.DbServerQuerySample.Enabled = false
			logsCfg.Events.DbServerSessionWaitSample.Enabled = true
			scrpr := oracleScraper{
				logger: zap.NewNop(),
				dbProviderFunc: func() (*sql.DB, error) {
					return nil, nil
				},
				clientProviderFunc:  test.dbclientFn,
				id:                  component.ID{},
				lb:                  metadata.NewLogsBuilder(logsCfg, receivertest.NewNopSettings(metadata.Type)),
				logsBuilderConfig:   logsCfg,
				obfuscator:          newObfuscator(),
				instanceName:        "oraclehost:1521/ORCL",
				serviceInstanceID:   getInstanceID("oraclehost:1521/ORCL", zap.NewNop()),
				sessionWaitEventCfg: SessionWaitEvent{MaxRowsPerQuery: 200},
			}
			err := scrpr.start(t.Context(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(t.Context()))
			}()
			require.NoError(t, err)
			logs, err := scrpr.scrapeLogs(t.Context())
			expectedSessionEventsFile := filepath.Join("testdata", "expectedSessionEventsFile.yaml")

			if test.errWanted != "" {
				require.EqualError(t, err, test.errWanted)
			} else {
				// Uncomment line below to re-generate expected logs.
				// golden.WriteLogs(t, expectedSessionEventsFile, logs)
				require.NoError(t, err)
				expectedLogs, _ := golden.ReadLogs(expectedSessionEventsFile)
				errs := plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreTimestamp())
				assert.Equal(t, "db.server.session.wait_sample", logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
				assert.NoError(t, errs)
			}
		})
	}
}

func TestScraper_ScrapeSysMetrics(t *testing.T) {
	const floatDelta = 0.001

	tests := []struct {
		name      string
		clientFn  func(db *sql.DB, s string, logger *zap.Logger) dbClient
		errWanted string
	}{
		{
			name: "valid sysmetric data",
			clientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				return &fakeDbClient{
					Responses: [][]metricRow{queryResponses[s]},
				}
			},
		},
		{
			name: "bad sysmetric value",
			clientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == sysmetricSQL {
					return &fakeDbClient{Responses: [][]metricRow{
						{{"METRIC_NAME": "Buffer Cache Hit Ratio", "VALUE": "not_a_number"}},
					}}
				}
				return &fakeDbClient{Responses: [][]metricRow{queryResponses[s]}}
			},
			errWanted: `sysmetric "Buffer Cache Hit Ratio": failed to parse float64`,
		},
		{
			name: "sysmetric query error",
			clientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == sysmetricSQL {
					return &fakeDbClient{Err: errors.New("db connection lost")}
				}
				return &fakeDbClient{Responses: [][]metricRow{queryResponses[s]}}
			},
			errWanted: "error executing SELECT metric_name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := metadata.NewDefaultMetricsBuilderConfig()
			cfg.Metrics.OracledbBufferCacheUtilization.Enabled = true
			cfg.Metrics.OracledbHostCPUUtilization.Enabled = true
			cfg.Metrics.OracledbDatabaseCPUUtilization.Enabled = true
			cfg.Metrics.OracledbLibraryCacheUtilization.Enabled = true
			cfg.Metrics.OracledbSharedPoolUtilization.Enabled = true
			cfg.Metrics.OracledbDatabaseWaitUtilization.Enabled = true
			cfg.Metrics.OracledbParseUtilization.Enabled = true
			cfg.Metrics.OracledbSQLServiceResponseDuration.Enabled = true
			cfg.Metrics.OracledbSortRatio.Enabled = true
			cfg.Metrics.OracledbRedoAllocationUtilization.Enabled = true
			cfg.Metrics.OracledbParseRate.Enabled = true
			cfg.Metrics.OracledbExecutionUtilization.Enabled = true

			scrpr := oracleScraper{
				logger: zap.NewNop(),
				mb:     metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
				dbProviderFunc: func() (*sql.DB, error) {
					return nil, nil
				},
				clientProviderFunc:   test.clientFn,
				id:                   component.ID{},
				metricsBuilderConfig: cfg,
			}
			err := scrpr.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, scrpr.shutdown(t.Context()))
			}()

			m, err := scrpr.scrape(t.Context())

			if test.errWanted != "" {
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.Contains(t, err.Error(), test.errWanted)
				return
			}

			require.NoError(t, err)

			metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			metricMap := make(map[string]float64)
			for i := 0; i < metrics.Len(); i++ {
				metric := metrics.At(i)
				if metric.Type() == pmetric.MetricTypeGauge && metric.Gauge().DataPoints().Len() > 0 {
					metricMap[metric.Name()] = metric.Gauge().DataPoints().At(0).DoubleValue()
				}
			}

			assert.InDelta(t, 98.75, metricMap["oracledb.buffer_cache.utilization"], floatDelta)
			assert.InDelta(t, 12.34, metricMap["oracledb.host.cpu.utilization"], floatDelta)
			assert.InDelta(t, 55.66, metricMap["oracledb.database.cpu.utilization"], floatDelta)
			assert.InDelta(t, 99.10, metricMap["oracledb.library_cache.utilization"], floatDelta)
			assert.InDelta(t, 30.20, metricMap["oracledb.shared_pool.utilization"], floatDelta)
			assert.InDelta(t, 44.55, metricMap["oracledb.database.wait.utilization"], floatDelta)
			assert.InDelta(t, 88.90, metricMap["oracledb.parse.utilization"], floatDelta)
			assert.InDelta(t, 0.000042, metricMap["oracledb.sql_service.response.duration"], floatDelta)
			assert.InDelta(t, 99.50, metricMap["oracledb.sort.ratio"], floatDelta)
			assert.InDelta(t, 97.80, metricMap["oracledb.redo_allocation.utilization"], floatDelta)
			assert.InDelta(t, 0.25, metricMap["oracledb.parse.rate"], floatDelta)
			assert.InDelta(t, 75.30, metricMap["oracledb.execution.utilization"], floatDelta)
		})
	}
}

func TestGetInstanceId(t *testing.T) {
	localhostName, _ := os.Hostname()

	instanceString := "example.com:1521/XE"
	instanceID := getInstanceID(instanceString, zap.NewNop())
	assert.Equal(t, "example.com:1521/XE", instanceID)

	localHostStringUppercase := "Localhost:1521/XE"
	localInstanceID := getInstanceID(localHostStringUppercase, zap.NewNop())
	assert.NotNil(t, localInstanceID)
	assert.Equal(t, localhostName+":1521/XE", localInstanceID)

	localHostString := "127.0.0.1:1521/XE"
	localInstanceID = getInstanceID(localHostString, zap.NewNop())
	assert.NotNil(t, localInstanceID)
	assert.Equal(t, localhostName+":1521/XE", localInstanceID)

	localHostStringIPV6 := "[::1]:1521/XE"
	localInstanceID = getInstanceID(localHostStringIPV6, zap.NewNop())
	assert.NotNil(t, localInstanceID)
	assert.Equal(t, localhostName+":1521/XE", localInstanceID)

	hostWithoutService := "127.0.0.1:1521"
	localInstanceID = getInstanceID(hostWithoutService, zap.NewNop())
	assert.NotNil(t, localInstanceID)
	assert.Equal(t, localhostName+":1521", localInstanceID)

	hostNameErrorSample := ""
	localInstanceID = getInstanceID(hostNameErrorSample, zap.NewNop())
	assert.NotNil(t, localInstanceID)
	assert.Equal(t, "unknown:1521", localInstanceID)
}

func TestTopNLogsDiscardedWhenExecutionCountUnchanged(t *testing.T) {
	var metricRowData []metricRow
	var logRowData []metricRow

	// Cache value with same EXECUTIONS as mock data (300413) but different other metrics.
	// This simulates: execution count unchanged, but other metrics have positive deltas.
	cacheValueSameExecCount := map[string]int64{
		"APPLICATION_WAIT_TIME":   0,
		"BUFFER_GETS":             3808197,
		"CLUSTER_WAIT_TIME":       1000,
		"CONCURRENCY_WAIT_TIME":   20,
		"CPU_TIME":                29821063,
		"DIRECT_READS":            3,
		"DIRECT_WRITES":           6,
		"DISK_READS":              12,
		"ELAPSED_TIME":            38172810,
		"EXECUTIONS":              300413, // same as mock data — delta will be 0
		"PHYSICAL_READ_BYTES":     300,
		"PHYSICAL_READ_REQUESTS":  100,
		"PHYSICAL_WRITE_BYTES":    12,
		"PHYSICAL_WRITE_REQUESTS": 120,
		"ROWS_PROCESSED":          200413,
		"USER_IO_WAIT_TIME":       200,
		"PROCEDURE_EXECUTIONS":    300413,
	}

	logsCfg := metadata.DefaultLogsBuilderConfig()
	logsCfg.ResourceAttributes.HostName.Enabled = true
	logsCfg.Events.DbServerTopQuery.Enabled = true
	metricsCfg := metadata.NewDefaultMetricsBuilderConfig()
	lruCache, _ := lru.New[string, map[string]int64](500)
	lruCache.Add("fxk8aq3nds8aw:0", cacheValueSameExecCount)

	scrpr := oracleScraper{
		logger: zap.NewNop(),
		mb:     metadata.NewMetricsBuilder(metricsCfg, receivertest.NewNopSettings(metadata.Type)),
		lb:     metadata.NewLogsBuilder(logsCfg, receivertest.NewNopSettings(metadata.Type)),
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, nil
		},
		clientProviderFunc: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
			if strings.Contains(s, SQLPlanTable) {
				metricRowFile := readFile("oracleQueryPlanData.txt")
				_ = json.Unmarshal(metricRowFile, &logRowData)
				return &fakeDbClient{Responses: [][]metricRow{logRowData}}
			}
			metricRowFile := readFile("oracleQueryMetricsData.txt")
			_ = json.Unmarshal(metricRowFile, &metricRowData)
			return &fakeDbClient{Responses: [][]metricRow{metricRowData}}
		},
		id:                   component.ID{},
		metricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		logsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
		metricCache:          lruCache,
		topQueryCollectCfg:   TopQueryCollection{MaxQuerySampleCount: 5000, TopQueryCount: 200},
		instanceName:         "oraclehost:1521/ORCL",
		hostName:             "oraclehost:1521",
		obfuscator:           newObfuscator(),
		serviceInstanceID:    getInstanceID("oraclehost:1521/ORCL", zap.NewNop()),
	}

	scrpr.logsBuilderConfig.Events.DbServerTopQuery.Enabled = true

	err := scrpr.start(t.Context(), componenttest.NewNopHost())
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()
	require.NoError(t, err)

	logs, err := scrpr.scrapeLogs(t.Context())
	require.NoError(t, err)
	// No top query records should be emitted when execution count delta is zero
	assert.Equal(t, 0, logs.ResourceLogs().Len(), "No top query logs should be emitted when execution count has not increased")
}

func TestTopNLogsProcedureNameEmpty(t *testing.T) {
	metricsData := []metricRow{
		{
			"APPLICATION_WAIT_TIME": "0", "BUFFER_GETS": "4000000", "CHILD_ADDRESS": "ADDR1",
			"CHILD_NUMBER": "0", "CLUSTER_WAIT_TIME": "0", "CONCURRENCY_WAIT_TIME": "0",
			"CPU_TIME": "40000000", "DIRECT_READS": "0", "DIRECT_WRITES": "0", "DISK_READS": "0",
			"ELAPSED_TIME": "50000000", "EXECUTIONS": "500", "PHYSICAL_READ_BYTES": "0",
			"PHYSICAL_READ_REQUESTS": "0", "PHYSICAL_WRITE_BYTES": "0", "PHYSICAL_WRITE_REQUESTS": "0",
			"ROWS_PROCESSED": "500", "SQL_FULLTEXT": "SELECT 1 FROM DUAL",
			"SQL_ID": "abc123", "USER_IO_WAIT_TIME": "0",
			"PROGRAM_ID": "", "PROCEDURE_NAME": "", "PROCEDURE_TYPE": "", "PROCEDURE_EXECUTIONS": "0",
			"COMMAND_TYPE": "3",
		},
		{
			"APPLICATION_WAIT_TIME": "0", "BUFFER_GETS": "5000000", "CHILD_ADDRESS": "ADDR2",
			"CHILD_NUMBER": "0", "CLUSTER_WAIT_TIME": "0", "CONCURRENCY_WAIT_TIME": "0",
			"CPU_TIME": "50000000", "DIRECT_READS": "0", "DIRECT_WRITES": "0", "DISK_READS": "0",
			"ELAPSED_TIME": "60000000", "EXECUTIONS": "600", "PHYSICAL_READ_BYTES": "0",
			"PHYSICAL_READ_REQUESTS": "0", "PHYSICAL_WRITE_BYTES": "0", "PHYSICAL_WRITE_REQUESTS": "0",
			"ROWS_PROCESSED": "600", "SQL_FULLTEXT": "SELECT * FROM ADMIN.EMP",
			"SQL_ID": "def456", "USER_IO_WAIT_TIME": "0",
			"PROGRAM_ID": "98765", "PROCEDURE_NAME": "ADMIN.MY_PROC", "PROCEDURE_TYPE": "PROCEDURE", "PROCEDURE_EXECUTIONS": "600",
			"COMMAND_TYPE": "3",
		},
	}

	planData := []metricRow{}

	logsCfg := metadata.DefaultLogsBuilderConfig()
	logsCfg.ResourceAttributes.HostName.Enabled = true
	logsCfg.Events.DbServerTopQuery.Enabled = true
	metricsCfg := metadata.NewDefaultMetricsBuilderConfig()

	lruCache, _ := lru.New[string, map[string]int64](500)
	lruCache.Add("abc123:0", map[string]int64{
		"APPLICATION_WAIT_TIME": 0, "BUFFER_GETS": 3000000, "CLUSTER_WAIT_TIME": 0,
		"CONCURRENCY_WAIT_TIME": 0, "CPU_TIME": 30000000, "DIRECT_READS": 0, "DIRECT_WRITES": 0,
		"DISK_READS": 0, "ELAPSED_TIME": 40000000, "EXECUTIONS": 400, "PHYSICAL_READ_BYTES": 0,
		"PHYSICAL_READ_REQUESTS": 0, "PHYSICAL_WRITE_BYTES": 0, "PHYSICAL_WRITE_REQUESTS": 0,
		"ROWS_PROCESSED": 400, "USER_IO_WAIT_TIME": 0, "PROCEDURE_EXECUTIONS": 0,
	})
	lruCache.Add("def456:0", map[string]int64{
		"APPLICATION_WAIT_TIME": 0, "BUFFER_GETS": 4000000, "CLUSTER_WAIT_TIME": 0,
		"CONCURRENCY_WAIT_TIME": 0, "CPU_TIME": 40000000, "DIRECT_READS": 0, "DIRECT_WRITES": 0,
		"DISK_READS": 0, "ELAPSED_TIME": 50000000, "EXECUTIONS": 500, "PHYSICAL_READ_BYTES": 0,
		"PHYSICAL_READ_REQUESTS": 0, "PHYSICAL_WRITE_BYTES": 0, "PHYSICAL_WRITE_REQUESTS": 0,
		"ROWS_PROCESSED": 500, "USER_IO_WAIT_TIME": 0, "PROCEDURE_EXECUTIONS": 500,
	})

	scrpr := oracleScraper{
		logger: zap.NewNop(),
		mb:     metadata.NewMetricsBuilder(metricsCfg, receivertest.NewNopSettings(metadata.Type)),
		lb:     metadata.NewLogsBuilder(logsCfg, receivertest.NewNopSettings(metadata.Type)),
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, nil
		},
		clientProviderFunc: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
			if strings.Contains(s, SQLPlanTable) {
				return &fakeDbClient{Responses: [][]metricRow{planData}}
			}
			return &fakeDbClient{Responses: [][]metricRow{metricsData}}
		},
		id:                   component.ID{},
		metricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		logsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
		metricCache:          lruCache,
		topQueryCollectCfg:   TopQueryCollection{MaxQuerySampleCount: 5000, TopQueryCount: 200},
		instanceName:         "oraclehost:1521/ORCL",
		hostName:             "oraclehost:1521",
		obfuscator:           newObfuscator(),
		serviceInstanceID:    getInstanceID("oraclehost:1521/ORCL", zap.NewNop()),
	}

	scrpr.logsBuilderConfig.Events.DbServerTopQuery.Enabled = true

	err := scrpr.start(t.Context(), componenttest.NewNopHost())
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()
	require.NoError(t, err)

	logs, err := scrpr.scrapeLogs(t.Context())
	require.NoError(t, err)
	require.Positive(t, logs.ResourceLogs().Len())

	scopeLogs := logs.ResourceLogs().At(0).ScopeLogs().At(0)
	require.Equal(t, 2, scopeLogs.LogRecords().Len(), "Expected 2 top query records")

	for i := range scopeLogs.LogRecords().Len() {
		lr := scopeLogs.LogRecords().At(i)
		procName, _ := lr.Attributes().Get("oracledb.procedure_name")
		assert.NotEqual(t, ".", procName.Str(),
			"Procedure name must not be '.' — should be empty or a valid name (record %d)", i)
	}
}

func TestScrapesTopNLogsOnlyWhenIntervalHasElapsed(t *testing.T) {
	var metricRowData []metricRow
	var logRowData []metricRow
	tests := []struct {
		name       string
		dbclientFn func(db *sql.DB, s string, logger *zap.Logger) dbClient
		errWanted  string
	}{
		{
			name: "valid collection",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if strings.Contains(s, SQLPlanTable) {
					metricRowFile := readFile("oracleQueryPlanData.txt")
					unmarshalErr := json.Unmarshal(metricRowFile, &logRowData)
					if unmarshalErr == nil {
						return &fakeDbClient{
							Responses: [][]metricRow{
								logRowData,
							},
						}
					}
				} else {
					metricRowFile := readFile("oracleQueryMetricsData.txt")
					unmarshalErr := json.Unmarshal(metricRowFile, &metricRowData)
					if unmarshalErr == nil {
						return &fakeDbClient{
							Responses: [][]metricRow{
								metricRowData,
							},
						}
					}
				}
				return nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logsCfg := metadata.DefaultLogsBuilderConfig()
			logsCfg.Events.DbServerTopQuery.Enabled = true
			metricsCfg := metadata.NewDefaultMetricsBuilderConfig()
			lruCache, _ := lru.New[string, map[string]int64](500)
			lruCache.Add("fxk8aq3nds8aw:0", cacheValue)

			scrpr := oracleScraper{
				logger: zap.NewNop(),
				mb:     metadata.NewMetricsBuilder(metricsCfg, receivertest.NewNopSettings(metadata.Type)),
				lb:     metadata.NewLogsBuilder(logsCfg, receivertest.NewNopSettings(metadata.Type)),
				dbProviderFunc: func() (*sql.DB, error) {
					return nil, nil
				},
				clientProviderFunc:   test.dbclientFn,
				metricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				logsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
				metricCache:          lruCache,
				topQueryCollectCfg:   TopQueryCollection{MaxQuerySampleCount: 5000, TopQueryCount: 200},
				obfuscator:           newObfuscator(),
			}

			scrpr.logsBuilderConfig.Events.DbServerTopQuery.Enabled = true
			scrpr.topQueryCollectCfg.CollectionInterval = 1 * time.Minute

			err := scrpr.start(t.Context(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(t.Context()))
			}()
			require.NoError(t, err)

			assert.True(t, scrpr.lastExecutionTimestamp.IsZero(), "No value should be set for lastExecutionTimestamp before a successful collection")
			logsCol1, _ := scrpr.scrapeLogs(t.Context())
			assert.Equal(t, 1, logsCol1.ResourceLogs().At(0).ScopeLogs().Len(), "Collection should run when lastExecutionTimestamp is not available")
			assert.False(t, scrpr.lastExecutionTimestamp.IsZero(), "A value should be set for lastExecutionTimestamp after a successful collection")

			scrpr.lastExecutionTimestamp = scrpr.lastExecutionTimestamp.Add(-10 * time.Second)
			logsCol2, err := scrpr.scrapeLogs(t.Context())
			assert.Equal(t, 0, logsCol2.ResourceLogs().Len(), "top_query should not be collected until %s elapsed.", scrpr.topQueryCollectCfg.CollectionInterval.String())
			require.NoError(t, err)
		})
	}
}

// TestObfuscateCacheHitsHandlesTruncatedSQL verifies that the obfuscator
// successfully handles SQL with truncated string literals that may occur
// when Oracle's CLOB display limit is reached.
func TestObfuscateCacheHitsHandlesTruncatedSQL(t *testing.T) {
	// Build two metric rows with different SQL queries to verify obfuscation.
	metricsData := []metricRow{
		{
			"APPLICATION_WAIT_TIME": "0", "BUFFER_GETS": "4000000", "CHILD_ADDRESS": "ADDR1",
			"CHILD_NUMBER": "0", "CLUSTER_WAIT_TIME": "0", "CONCURRENCY_WAIT_TIME": "0",
			"CPU_TIME": "40000000", "DIRECT_READS": "0", "DIRECT_WRITES": "0", "DISK_READS": "0",
			"ELAPSED_TIME": "50000000", "EXECUTIONS": "500", "PHYSICAL_READ_BYTES": "0",
			"PHYSICAL_READ_REQUESTS": "0", "PHYSICAL_WRITE_BYTES": "0", "PHYSICAL_WRITE_REQUESTS": "0",
			"ROWS_PROCESSED": "500", "SQL_FULLTEXT": "SELECT 1 FROM DUAL",
			"SQL_ID": "valid001", "USER_IO_WAIT_TIME": "0",
			"PROGRAM_ID": "", "PROCEDURE_NAME": "", "PROCEDURE_TYPE": "", "PROCEDURE_EXECUTIONS": "0",
			"COMMAND_TYPE": "3",
		},
		{
			// SQL with truncated string literal.
			"APPLICATION_WAIT_TIME": "0", "BUFFER_GETS": "5000000", "CHILD_ADDRESS": "ADDR2",
			"CHILD_NUMBER": "0", "CLUSTER_WAIT_TIME": "0", "CONCURRENCY_WAIT_TIME": "0",
			"CPU_TIME": "50000000", "DIRECT_READS": "0", "DIRECT_WRITES": "0", "DISK_READS": "0",
			"ELAPSED_TIME": "60000000", "EXECUTIONS": "600", "PHYSICAL_READ_BYTES": "0",
			"PHYSICAL_READ_REQUESTS": "0", "PHYSICAL_WRITE_BYTES": "0", "PHYSICAL_WRITE_REQUESTS": "0",
			"ROWS_PROCESSED": "600", "SQL_FULLTEXT": "SELECT 'unterminated",
			"SQL_ID": "trunc01", "USER_IO_WAIT_TIME": "0",
			"PROGRAM_ID": "", "PROCEDURE_NAME": "", "PROCEDURE_TYPE": "", "PROCEDURE_EXECUTIONS": "0",
			"COMMAND_TYPE": "3",
		},
	}

	logsCfg := metadata.DefaultLogsBuilderConfig()
	logsCfg.ResourceAttributes.HostName.Enabled = true
	logsCfg.Events.DbServerTopQuery.Enabled = true
	metricsCfg := metadata.NewDefaultMetricsBuilderConfig()

	lruCache, _ := lru.New[string, map[string]int64](500)
	lruCache.Add("valid001:0", map[string]int64{
		"APPLICATION_WAIT_TIME": 0, "BUFFER_GETS": 3000000, "CLUSTER_WAIT_TIME": 0,
		"CONCURRENCY_WAIT_TIME": 0, "CPU_TIME": 30000000, "DIRECT_READS": 0, "DIRECT_WRITES": 0,
		"DISK_READS": 0, "ELAPSED_TIME": 40000000, "EXECUTIONS": 400, "PHYSICAL_READ_BYTES": 0,
		"PHYSICAL_READ_REQUESTS": 0, "PHYSICAL_WRITE_BYTES": 0, "PHYSICAL_WRITE_REQUESTS": 0,
		"ROWS_PROCESSED": 400, "USER_IO_WAIT_TIME": 0, "PROCEDURE_EXECUTIONS": 0,
	})
	lruCache.Add("trunc01:0", map[string]int64{
		"APPLICATION_WAIT_TIME": 0, "BUFFER_GETS": 4000000, "CLUSTER_WAIT_TIME": 0,
		"CONCURRENCY_WAIT_TIME": 0, "CPU_TIME": 40000000, "DIRECT_READS": 0, "DIRECT_WRITES": 0,
		"DISK_READS": 0, "ELAPSED_TIME": 50000000, "EXECUTIONS": 500, "PHYSICAL_READ_BYTES": 0,
		"PHYSICAL_READ_REQUESTS": 0, "PHYSICAL_WRITE_BYTES": 0, "PHYSICAL_WRITE_REQUESTS": 0,
		"ROWS_PROCESSED": 500, "USER_IO_WAIT_TIME": 0, "PROCEDURE_EXECUTIONS": 0,
	})

	core, observedLogs := observer.New(zapcore.WarnLevel)
	observedLogger := zap.New(core)

	scrpr := oracleScraper{
		logger: observedLogger,
		mb:     metadata.NewMetricsBuilder(metricsCfg, receivertest.NewNopSettings(metadata.Type)),
		lb:     metadata.NewLogsBuilder(logsCfg, receivertest.NewNopSettings(metadata.Type)),
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, nil
		},
		clientProviderFunc: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
			if strings.Contains(s, SQLPlanTable) {
				return &fakeDbClient{Responses: [][]metricRow{{}}}
			}
			return &fakeDbClient{Responses: [][]metricRow{metricsData}}
		},
		id:                   component.ID{},
		metricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		logsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
		metricCache:          lruCache,
		topQueryCollectCfg:   TopQueryCollection{MaxQuerySampleCount: 5000, TopQueryCount: 200},
		instanceName:         "oraclehost:1521/ORCL",
		hostName:             "oraclehost:1521",
		obfuscator:           newObfuscator(),
		serviceInstanceID:    getInstanceID("oraclehost:1521/ORCL", zap.NewNop()),
	}

	scrpr.logsBuilderConfig.Events.DbServerTopQuery.Enabled = true

	err := scrpr.start(t.Context(), componenttest.NewNopHost())
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()
	require.NoError(t, err)

	logs, err := scrpr.scrapeLogs(t.Context())
	require.NoError(t, err)

	// Both log records should be emitted.
	require.Equal(t, 1, logs.ResourceLogs().Len())
	records := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 2, records.Len(), "Expected both entries to be emitted")

	// Verify no obfuscation errors were logged.
	warnLogs := observedLogs.FilterMessage("oracleScraper failed to obfuscate SQL query, skipping entry")
	assert.Equal(t, 0, warnLogs.Len(), "Expected no obfuscation failures")
}

func TestCalculateLookbackSeconds(t *testing.T) {
	collectionInterval := 20 * time.Second
	vsqlRefreshLagSec := 10 * time.Second
	expectedMinimumLookbackTime := int((collectionInterval + vsqlRefreshLagSec).Seconds())
	currentCollectionTime := time.Now()

	scrpr := oracleScraper{
		lastExecutionTimestamp: currentCollectionTime.Add(-collectionInterval),
	}
	lookbackTime := scrpr.calculateLookbackSeconds()

	assert.LessOrEqual(t, expectedMinimumLookbackTime, lookbackTime, "`lookbackTime` should be minimum %d", expectedMinimumLookbackTime)
}

func TestScraper_ScrapeSGAInfo(t *testing.T) {
	tests := []struct {
		name       string
		dbclientFn func(db *sql.DB, s string, logger *zap.Logger) dbClient
		errWanted  string
	}{
		{
			name: "valid",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				return &fakeDbClient{Responses: [][]metricRow{queryResponses[s]}}
			},
		},
		{
			name: "bad bytes value",
			dbclientFn: func(_ *sql.DB, s string, _ *zap.Logger) dbClient {
				if s == sgaInfoSQL {
					return &fakeDbClient{Responses: [][]metricRow{
						{{"NAME": "Buffer Cache Size", "BYTES": "not_a_number"}},
					}}
				}
				return &fakeDbClient{Responses: [][]metricRow{queryResponses[s]}}
			},
			errWanted: `failed to parse int64 for SGA row "Buffer Cache Size", value was "not_a_number"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := metadata.NewDefaultMetricsBuilderConfig()
			cfg.Metrics.OracledbSgaUsage.Enabled = true
			cfg.Metrics.OracledbSgaLimit.Enabled = true

			scrpr := oracleScraper{
				logger: zap.NewNop(),
				mb:     metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
				dbProviderFunc: func() (*sql.DB, error) {
					return nil, nil
				},
				clientProviderFunc:   test.dbclientFn,
				id:                   component.ID{},
				metricsBuilderConfig: cfg,
			}
			err := scrpr.start(t.Context(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(t.Context()))
			}()
			require.NoError(t, err)
			m, err := scrpr.scrape(t.Context())
			if test.errWanted != "" {
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.Contains(t, err.Error(), test.errWanted)
				return
			}
			require.NoError(t, err)
			metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			wantUsage := map[string]int64{
				"buffer_cache":        1375731712,
				"data_transfer_cache": 0,
				"fixed_sga":           9292416,
				"java_pool":           0,
				"large_pool":          33554432,
				"redo_buffers":        14598144,
				"shared_io_pool":      134217728,
				"shared_pool":         536870912,
				"streams_pool":        0,
			}
			gotUsage := map[string]int64{}
			var sgaLimit int64
			for i := 0; i < metrics.Len(); i++ {
				metric := metrics.At(i)
				switch metric.Name() {
				case "oracledb.sga.usage":
					for j := 0; j < metric.Gauge().DataPoints().Len(); j++ {
						dp := metric.Gauge().DataPoints().At(j)
						name, _ := dp.Attributes().Get("oracledb.sga.component.name")
						gotUsage[name.Str()] = dp.IntValue()
					}
				case "oracledb.sga.limit":
					sgaLimit = metric.Gauge().DataPoints().At(0).IntValue()
				}
			}
			assert.Equal(t, wantUsage, gotUsage)
			assert.Equal(t, int64(2147483648), sgaLimit)
		})
	}
}

func readFile(fname string) []byte {
	file, err := os.ReadFile(filepath.Join("testdata", fname))
	if err != nil {
		log.Fatal(err)
	}
	return file
}
