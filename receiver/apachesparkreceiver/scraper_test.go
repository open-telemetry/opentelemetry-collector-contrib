// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apachesparkreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"
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
		expectedErr       error
	}{
		{
			desc: "Nil client",
			setupMockClient: func(t *testing.T) client {
				return nil
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errClientNotInit,
		},
		{
			desc: "Exits on failure to get app ids",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetApplications").Return(nil, errors.New("could not retrieve app ids"))
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errFailedAppIdCollection,
		},
		{
			desc: "Successful Full Empty Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetClusterStats").Return(&models.ClusterProperties{}, nil)
				mockClient.On("GetApplications").Return(&models.Applications{}, nil)
				mockClient.On("GetStageStats", mock.Anything).Return(&models.Stages{}, nil)
				mockClient.On("GetExecutorStats", mock.Anything).Return(&models.Executors{}, nil)
				mockClient.On("GetJobStats", mock.Anything).Return(&models.Jobs{}, nil)
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_empty_golden.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
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
				mockClient.On("GetClusterStats").Return(clusterStats, nil)

				data = loadAPIResponseData(t, appsStatsResponseFile)
				var apps *models.Applications
				err = json.Unmarshal(data, &apps)
				require.NoError(t, err)
				mockClient.On("GetApplications").Return(apps, nil)

				mockClient.On("GetStageStats", mock.Anything).Return(nil, errors.New("stage api error"))
				mockClient.On("GetExecutorStats", mock.Anything).Return(nil, errors.New("executor api error"))
				mockClient.On("GetJobStats", mock.Anything).Return(nil, errors.New("jobs api error"))

				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_partial_golden.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
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
				mockClient.On("GetClusterStats").Return(clusterStats, nil)

				data = loadAPIResponseData(t, appsStatsResponseFile)
				var apps *models.Applications
				err = json.Unmarshal(data, &apps)
				require.NoError(t, err)
				mockClient.On("GetApplications").Return(apps, nil)

				data = loadAPIResponseData(t, stagesStatsResponseFile)
				var stages *models.Stages
				err = json.Unmarshal(data, &stages)
				require.NoError(t, err)
				mockClient.On("GetStageStats", mock.Anything).Return(stages, nil)

				data = loadAPIResponseData(t, executorsStatsResponseFile)
				var executors *models.Executors
				err = json.Unmarshal(data, &executors)
				require.NoError(t, err)
				mockClient.On("GetExecutorStats", mock.Anything).Return(executors, nil)

				data = loadAPIResponseData(t, jobsStatsResponseFile)
				var jobs *models.Jobs
				err = json.Unmarshal(data, &jobs)
				require.NoError(t, err)
				mockClient.On("GetJobStats", mock.Anything).Return(jobs, nil)
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_golden.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			scraper := newSparkScraper(zap.NewNop(), createDefaultConfig().(*Config), receivertest.NewNopCreateSettings())
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
