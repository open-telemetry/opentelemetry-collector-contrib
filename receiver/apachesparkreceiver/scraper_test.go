// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachesparkreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"
)

const (
	clusterStatsResponseFile   = "cluster_stats_response.json"
	appsStatsResponseFile      = "apps_stats_response.json"
	stagesStatsResponseFile    = "stages_stats_response.json"
	executorsStatsResponseFile = "executors_stats_response.json"
	jobsStatsResponseFile      = "jobs_stats_response.json"
)

func TestScraper(t *testing.T) {
	testcases := []struct {
		desc              string
		setupMockClient   func(t *testing.T) client
		expectedMetricGen func(t *testing.T) pmetric.Metrics
		config            *Config
		expectedErr       error
	}{
		{
			desc: "Exits on failure to get app ids",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("Applications").Return(nil, errors.New("could not retrieve app ids"))
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			config:      createDefaultConfig().(*Config),
			expectedErr: errFailedAppIDCollection,
		},
		{
			desc: "No Matching Allowed Apps",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("Applications").Return(&models.Applications{}, nil)
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			config: &Config{ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				CollectionInterval: defaultCollectionInterval,
			},
				ApplicationNames: []string{"local-123", "local-987"},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: defaultEndpoint,
				},
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedErr: errNoMatchingAllowedApps,
		},
		{
			desc: "Successful Full Empty Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("ClusterStats").Return(&models.ClusterProperties{}, nil)
				mockClient.On("Applications").Return(&models.Applications{}, nil)
				mockClient.On("StageStats", mock.Anything).Return(&models.Stages{}, nil)
				mockClient.On("ExecutorStats", mock.Anything).Return(&models.Executors{}, nil)
				mockClient.On("JobStats", mock.Anything).Return(&models.Jobs{}, nil)
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_empty_golden.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			config:      createDefaultConfig().(*Config),
			expectedErr: nil,
		},
		{
			desc: "Successful Partial Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				data := loadAPIResponseData(t, clusterStatsResponseFile)
				var clusterStats *models.ClusterProperties
				err := json.Unmarshal(data, &clusterStats)
				require.NoError(t, err)
				mockClient.On("ClusterStats").Return(clusterStats, nil)

				data = loadAPIResponseData(t, appsStatsResponseFile)
				var apps *models.Applications
				err = json.Unmarshal(data, &apps)
				require.NoError(t, err)
				mockClient.On("Applications").Return(apps, nil)

				mockClient.On("StageStats", mock.Anything).Return(nil, errors.New("stage api error"))
				mockClient.On("ExecutorStats", mock.Anything).Return(nil, errors.New("executor api error"))
				mockClient.On("JobStats", mock.Anything).Return(nil, errors.New("jobs api error"))

				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_partial_golden.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			config:      createDefaultConfig().(*Config),
			expectedErr: scrapererror.NewPartialScrapeError(errors.New("stage api error; executor api error; jobs api error"), 0),
		},
		{
			desc: "Successful Full Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				data := loadAPIResponseData(t, clusterStatsResponseFile)
				var clusterStats *models.ClusterProperties
				err := json.Unmarshal(data, &clusterStats)
				require.NoError(t, err)
				mockClient.On("ClusterStats").Return(clusterStats, nil)

				data = loadAPIResponseData(t, appsStatsResponseFile)
				var apps *models.Applications
				err = json.Unmarshal(data, &apps)
				require.NoError(t, err)
				mockClient.On("Applications").Return(apps, nil)

				data = loadAPIResponseData(t, stagesStatsResponseFile)
				var stages *models.Stages
				err = json.Unmarshal(data, &stages)
				require.NoError(t, err)
				mockClient.On("StageStats", mock.Anything).Return(stages, nil)

				data = loadAPIResponseData(t, executorsStatsResponseFile)
				var executors *models.Executors
				err = json.Unmarshal(data, &executors)
				require.NoError(t, err)
				mockClient.On("ExecutorStats", mock.Anything).Return(executors, nil)

				data = loadAPIResponseData(t, jobsStatsResponseFile)
				var jobs *models.Jobs
				err = json.Unmarshal(data, &jobs)
				require.NoError(t, err)
				mockClient.On("JobStats", mock.Anything).Return(jobs, nil)
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_golden.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			config:      createDefaultConfig().(*Config),
			expectedErr: nil,
		},
		{
			desc: "Successfully allowing apps",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				data := loadAPIResponseData(t, clusterStatsResponseFile)
				var clusterStats *models.ClusterProperties
				err := json.Unmarshal(data, &clusterStats)
				require.NoError(t, err)
				mockClient.On("ClusterStats").Return(clusterStats, nil)

				data = loadAPIResponseData(t, appsStatsResponseFile)
				var apps *models.Applications
				err = json.Unmarshal(data, &apps)
				require.NoError(t, err)
				mockClient.On("Applications").Return(apps, nil)

				data = loadAPIResponseData(t, stagesStatsResponseFile)
				var stages *models.Stages
				err = json.Unmarshal(data, &stages)
				require.NoError(t, err)
				mockClient.On("StageStats", mock.Anything).Return(stages, nil)

				data = loadAPIResponseData(t, executorsStatsResponseFile)
				var executors *models.Executors
				err = json.Unmarshal(data, &executors)
				require.NoError(t, err)
				mockClient.On("ExecutorStats", mock.Anything).Return(executors, nil)

				data = loadAPIResponseData(t, jobsStatsResponseFile)
				var jobs *models.Jobs
				err = json.Unmarshal(data, &jobs)
				require.NoError(t, err)
				mockClient.On("JobStats", mock.Anything).Return(jobs, nil)
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_golden.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			config: &Config{ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				CollectionInterval: defaultCollectionInterval,
			},
				ApplicationNames: []string{"streaming-example"},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: defaultEndpoint,
				},
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			scraper := newSparkScraper(zap.NewNop(), tc.config, receivertest.NewNopCreateSettings())
			scraper.client = tc.setupMockClient(t)

			actualMetrics, err := scraper.scrape(context.Background())

			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}

			expectedMetrics := tc.expectedMetricGen(t)

			err = pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
				pmetrictest.IgnoreResourceMetricsOrder(), pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreTimestamp())
			require.NoError(t, err)
		})
	}
}
