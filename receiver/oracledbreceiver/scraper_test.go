// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

func TestScraper_ErrorOnStart(t *testing.T) {
	scrpr := scraper{
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, errors.New("oops")
		},
	}
	err := scrpr.start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

var queryResponses = map[string][]metricRow{
	statsSQL:        {{"NAME": enqueueDeadlocks, "VALUE": "18"}, {"NAME": exchangeDeadlocks, "VALUE": "88898"}, {"NAME": executeCount, "VALUE": "178878"}, {"NAME": parseCountTotal, "VALUE": "1999"}, {"NAME": parseCountHard, "VALUE": "1"}, {"NAME": userCommits, "VALUE": "187778888"}, {"NAME": userRollbacks, "VALUE": "1898979879789"}, {"NAME": physicalReads, "VALUE": "1887777"}, {"NAME": sessionLogicalReads, "VALUE": "189"}, {"NAME": cpuTime, "VALUE": "1887"}, {"NAME": pgaMemory, "VALUE": "1999887"}},
	sessionCountSQL: {{"VALUE": "1"}},
	systemResourceLimitsSQL: {{"RESOURCE_NAME": "processes", "CURRENT_UTILIZATION": "3", "MAX_UTILIZATION": "10", "INITIAL_ALLOCATION": "100", "LIMIT_VALUE": "100"},
		{"RESOURCE_NAME": "locks", "CURRENT_UTILIZATION": "3", "MAX_UTILIZATION": "10", "INITIAL_ALLOCATION": "-1", "LIMIT_VALUE": "-1"}},
	tablespaceUsageSQL:    {{"TABLESPACE_NAME": "SYS", "BYTES": "1024"}},
	tablespaceMaxSpaceSQL: {{"TABLESPACE_NAME": "SYS", "VALUE": "1024"}},
}

func TestScraper_Scrape(t *testing.T) {
	metricsBuilder := metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings(), receivertest.NewNopCreateSettings())

	scrpr := scraper{
		logger:         zap.NewNop(),
		metricsBuilder: metricsBuilder,
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, nil
		},
		clientProviderFunc: func(db *sql.DB, s string, logger *zap.Logger) dbClient {
			return &fakeDbClient{
				Responses: [][]metricRow{
					queryResponses[s],
				},
			}
		},
		id:              component.ID{},
		metricsSettings: metadata.DefaultMetricsSettings(),
	}
	err := scrpr.start(context.Background(), componenttest.NewNopHost())
	defer func() {
		assert.NoError(t, scrpr.shutdown(context.Background()))
	}()
	require.NoError(t, err)
	m, err := scrpr.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 16, m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
}

func TestPartial_InvalidScrape(t *testing.T) {
	metricsBuilder := metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings(), receivertest.NewNopCreateSettings())

	scrpr := scraper{
		logger:         zap.NewNop(),
		metricsBuilder: metricsBuilder,
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, nil
		},
		clientProviderFunc: func(db *sql.DB, s string, logger *zap.Logger) dbClient {
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
		id:              component.ID{},
		metricsSettings: metadata.DefaultMetricsSettings(),
	}
	err := scrpr.start(context.Background(), componenttest.NewNopHost())
	defer func() {
		assert.NoError(t, scrpr.shutdown(context.Background()))
	}()
	require.NoError(t, err)
	_, err = scrpr.scrape(context.Background())
	require.Error(t, err)
	require.True(t, scrapererror.IsPartialScrapeError(err))
	require.EqualError(t, err, `failed to parse int64 for OracledbTablespaceSizeUsage, value was : strconv.ParseInt: parsing "": invalid syntax`)
}
