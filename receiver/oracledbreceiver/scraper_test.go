// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

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
