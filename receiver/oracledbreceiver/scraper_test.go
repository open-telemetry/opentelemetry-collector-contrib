// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

func TestScraper_ErrorOnStart(t *testing.T) {
	scrpr := oracleScraper{
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, errors.New("oops")
		},
	}
	err := scrpr.start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

var queryResponses = map[string][]metricRow{
	statsSQL:        {{"NAME": enqueueDeadlocks, "VALUE": "18"}, {"NAME": exchangeDeadlocks, "VALUE": "88898"}, {"NAME": executeCount, "VALUE": "178878"}, {"NAME": parseCountTotal, "VALUE": "1999"}, {"NAME": parseCountHard, "VALUE": "1"}, {"NAME": userCommits, "VALUE": "187778888"}, {"NAME": userRollbacks, "VALUE": "1898979879789"}, {"NAME": physicalReads, "VALUE": "1887777"}, {"NAME": physicalReadsDirect, "VALUE": "31337"}, {"NAME": sessionLogicalReads, "VALUE": "189"}, {"NAME": cpuTime, "VALUE": "1887"}, {"NAME": pgaMemory, "VALUE": "1999887"}, {"NAME": dbBlockGets, "VALUE": "42"}, {"NAME": consistentGets, "VALUE": "78944"}},
	sessionCountSQL: {{"VALUE": "1"}},
	systemResourceLimitsSQL: {
		{"RESOURCE_NAME": "processes", "CURRENT_UTILIZATION": "3", "MAX_UTILIZATION": "10", "INITIAL_ALLOCATION": "100", "LIMIT_VALUE": "100"},
		{"RESOURCE_NAME": "locks", "CURRENT_UTILIZATION": "3", "MAX_UTILIZATION": "10", "INITIAL_ALLOCATION": "-1", "LIMIT_VALUE": "-1"},
	},
	tablespaceUsageSQL: {{"TABLESPACE_NAME": "SYS", "USED_SPACE": "111288", "TABLESPACE_SIZE": "3518587", "BLOCK_SIZE": "8192"}},
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
}

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
			cfg := metadata.DefaultMetricsBuilderConfig()
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
				metricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			}
			err := scrpr.start(context.Background(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(context.Background()))
			}()
			require.NoError(t, err)
			m, err := scrpr.scrape(context.Background())
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
				if strings.Contains(s, "V$SQL_PLAN") {
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
			metricsCfg := metadata.DefaultMetricsBuilderConfig()
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
				metricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				logsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
				metricCache:          lruCache,
				topQueryCollectCfg:   TopQueryCollection{MaxQuerySampleCount: 5000, TopQueryCount: 200},
				instanceName:         "oracle-instance-sample-1",
				hostName:             "oracle-host-sample-1",
				obfuscator:           newObfuscator(),
			}

			scrpr.logsBuilderConfig.Events.DbServerTopQuery.Enabled = true

			err := scrpr.start(context.Background(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(context.Background()))
			}()
			require.NoError(t, err)
			expectedQueryPlanFile := filepath.Join("testdata", "expectedQueryTextAndPlanQuery.yaml")

			logs, err := scrpr.scrapeLogs(context.Background())

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
			}
		})
	}
}

var samplesQueryResponses = map[string][]metricRow{
	samplesQuery: {{
		"MACHINE": "TEST-MACHINE", "USERNAME": "ADMIN", "SCHEMANAME": "ADMIN", "SQL_ID": "48bc50b6fuz4y", "WAIT_CLASS": "ONE", "OBJECT_NAME": "BLAH",
		"SQL_CHILD_NUMBER": "0", "SID": "675", "SERIAL#": "51295", "SQL_FULLTEXT": "test_query", "OSUSER": "test-user", "PROCESS": "1115", "OBJECT_TYPE": "OBJECT_TYPE-A",
		"PORT": "54440", "PROGRAM": "Oracle SQL Developer for VS Code", "MODULE": "Oracle SQL Developer for VS Code", "STATUS": "ACTIVE", "STATE": "WAITED KNOWN TIME", "PLAN_HASH_VALUE": "4199919568", "DURATION_SEC": "1",
	}},
	"invalidQuery": {{
		"MACHINE": "TEST-MACHINE", "USERNAME": "ADMIN", "SCHEMANAME": "ADMIN", "SQL_ID": "48bc50b6fuz4y",
		"SQL_CHILD_NUMBER": "0", "S.SID": "675", "SERIAL#": "51295", "SQL_FULLTEXT": "test_query", "OSUSER": "test-user", "PROCESS": "1115",
		"PORT": "54440", "PROGRAM": "Oracle SQL Developer for VS Code", "MODULE": "Oracle SQL Developer for VS Code", "STATUS": "ACTIVE", "STATE": "WAITED KNOWN TIME", "PLAN_HASH_VALUE": "4199919568", "DURATION_SEC": "",
	}},
}

func TestSamplesQuery(t *testing.T) {
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
						samplesQueryResponses[s],
					},
				}
			},
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
			}
			scrpr.logsBuilderConfig.Events.DbServerTopQuery.Enabled = false
			scrpr.logsBuilderConfig.Events.DbServerQuerySample.Enabled = true
			err := scrpr.start(context.Background(), componenttest.NewNopHost())
			defer func() {
				assert.NoError(t, scrpr.shutdown(context.Background()))
			}()
			require.NoError(t, err)
			logs, err := scrpr.scrapeLogs(context.Background())
			expectedSamplesFile := filepath.Join("testdata", "expectedSamplesFile.yaml")

			if test.errWanted != "" {
				require.EqualError(t, err, test.errWanted)
			} else {
				// Uncomment line below to re-generate expected logs.
				// golden.WriteLogs(t, expectedSamplesFile, logs)
				require.NoError(t, err)
				expectedLogs, _ := golden.ReadLogs(expectedSamplesFile)
				errs := plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreTimestamp())
				assert.Equal(t, "db.server.query_sample", logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
				assert.NoError(t, errs)
			}
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
