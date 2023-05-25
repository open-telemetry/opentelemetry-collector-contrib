// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/models"
)

var (
	mockJobmanagerMetrics  = "mock_jobmanager_metrics.json"
	mockTaskmanagerMetrics = "mock_taskmanager_metrics.json"
	mockJobsMetrics        = "mock_jobs_metrics.json"
	mockSubtaskMetrics     = "mock_subtask_metrics.json"

	mockResponses = "mockresponses"
)

func TestScraperStart(t *testing.T) {
	testcases := []struct {
		desc        string
		scraper     *flinkmetricsScraper
		expectError bool
	}{
		{
			desc: "Bad Config",
			scraper: &flinkmetricsScraper{
				cfg: &Config{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: defaultEndpoint,
						TLSSetting: configtls.TLSClientSetting{
							TLSSetting: configtls.TLSSetting{
								CAFile: "/non/existent",
							},
						},
					},
				},
				settings: componenttest.NewNopTelemetrySettings(),
			},
			expectError: true,
		},
		{
			desc: "Valid Config",
			scraper: &flinkmetricsScraper{
				cfg: &Config{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						TLSSetting: configtls.TLSClientSetting{},
						Endpoint:   defaultEndpoint,
					},
				},
				settings: componenttest.NewNopTelemetrySettings(),
			},
			expectError: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.scraper.start(context.Background(), componenttest.NewNopHost())
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestScraperScrape(t *testing.T) {
	// use helper function from client tests
	jobmanagerMetricValuesData := loadAPIResponseData(t, mockResponses, mockJobmanagerMetrics)
	taskmanagerMetricValuesData := loadAPIResponseData(t, mockResponses, mockTaskmanagerMetrics)
	jobsMetricValuesData := loadAPIResponseData(t, mockResponses, mockJobsMetrics)
	subtaskMetricValuesData := loadAPIResponseData(t, mockResponses, mockSubtaskMetrics)

	// unmarshal api responses into a metrics response
	var jobmanagerMetricsResponse *models.MetricsResponse
	var taskmanagerMetricsResponse *models.MetricsResponse
	var jobsMetricsResponse *models.MetricsResponse
	var subtaskMetricsResponse *models.MetricsResponse
	err := json.Unmarshal(jobmanagerMetricValuesData, &jobmanagerMetricsResponse)
	require.NoError(t, err)
	err = json.Unmarshal(taskmanagerMetricValuesData, &taskmanagerMetricsResponse)
	require.NoError(t, err)
	err = json.Unmarshal(jobsMetricValuesData, &jobsMetricsResponse)
	require.NoError(t, err)
	err = json.Unmarshal(subtaskMetricValuesData, &subtaskMetricsResponse)
	require.NoError(t, err)

	// populate scope metrics with attributes using the scope metrics response
	jobmanagerMetrics := models.JobmanagerMetrics{
		Host:    "mock-host",
		Metrics: *jobmanagerMetricsResponse,
	}

	var taskmanagerMetricsInstances []*models.TaskmanagerMetrics
	taskmanagerMetricsInstances = append(taskmanagerMetricsInstances, &models.TaskmanagerMetrics{
		Host:          "mock-host",
		TaskmanagerID: "mock-taskmanager-id",
		Metrics:       *taskmanagerMetricsResponse,
	})
	taskmanagerMetricsInstances = append(taskmanagerMetricsInstances, &models.TaskmanagerMetrics{
		Host:          "mock-host2",
		TaskmanagerID: "mock-taskmanager-id2",
		Metrics:       *taskmanagerMetricsResponse,
	})

	var jobsMetricsInstances []*models.JobMetrics
	jobsMetricsInstances = append(jobsMetricsInstances, &models.JobMetrics{
		Host:    "mock-host",
		JobName: "mock-job-name",
		Metrics: *jobsMetricsResponse,
	})
	jobsMetricsInstances = append(jobsMetricsInstances, &models.JobMetrics{
		Host:    "mock-host2",
		JobName: "mock-job-name2",
		Metrics: *jobsMetricsResponse,
	})

	var subtaskMetricsInstances []*models.SubtaskMetrics
	subtaskMetricsInstances = append(subtaskMetricsInstances, &models.SubtaskMetrics{
		Host:          "mock-host",
		TaskmanagerID: "mock-taskmanager-id",
		JobName:       "mock-job-name",
		TaskName:      "mock-task-name",
		SubtaskIndex:  "mock-subtask-index",
		Metrics:       *subtaskMetricsResponse,
	})

	testCases := []struct {
		desc               string
		setupMockClient    func(t *testing.T) client
		expectedMetricFile string
		expectedErr        error
	}{
		{
			desc: "Nil client",
			setupMockClient: func(t *testing.T) client {
				return nil
			},
			expectedMetricFile: filepath.Join("testdata", "expected_metrics", "no_metrics.yaml"),
			expectedErr:        errClientNotInit,
		},
		{
			desc: "API Call Failure on Jobmanagers",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetJobmanagerMetrics", mock.Anything).Return(&models.JobmanagerMetrics{}, errors.New("some api error"))
				mockClient.On("GetTaskmanagersMetrics", mock.Anything).Return(nil, nil)
				mockClient.On("GetJobsMetrics", mock.Anything).Return(nil, nil)
				mockClient.On("GetSubtasksMetrics", mock.Anything).Return(nil, nil)
				return &mockClient
			},
			expectedMetricFile: filepath.Join("testdata", "expected_metrics", "no_metrics.yaml"),
			expectedErr:        errors.New(jobmanagerFailedFetch + " some api error"),
		},
		{
			desc: "API Call Failure on Taskmanagers",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetJobmanagerMetrics", mock.Anything).Return(&jobmanagerMetrics, nil)
				mockClient.On("GetTaskmanagersMetrics", mock.Anything).Return(nil, errors.New("some api error"))
				mockClient.On("GetJobsMetrics", mock.Anything).Return(nil, nil)
				mockClient.On("GetSubtasksMetrics", mock.Anything).Return(nil, nil)
				return &mockClient
			},
			expectedMetricFile: filepath.Join("testdata", "expected_metrics", "partial_metrics_no_taskmanagers.yaml"),
			expectedErr:        errors.New(taskmanagerFailedFetch + " some api error"),
		},
		{
			desc: "API Call Failure on Jobs",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetJobmanagerMetrics", mock.Anything).Return(&jobmanagerMetrics, nil)
				mockClient.On("GetTaskmanagersMetrics", mock.Anything).Return(taskmanagerMetricsInstances, nil)
				mockClient.On("GetJobsMetrics", mock.Anything).Return(nil, errors.New("some api error"))
				mockClient.On("GetSubtasksMetrics", mock.Anything).Return(nil, nil)
				return &mockClient
			},
			expectedMetricFile: filepath.Join("testdata", "expected_metrics", "partial_metrics_no_jobs.yaml"),
			expectedErr:        errors.New(jobsFailedFetch + " some api error"),
		},
		{
			desc: "API Call Failure on Subtasks",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetJobmanagerMetrics", mock.Anything).Return(&jobmanagerMetrics, nil)
				mockClient.On("GetTaskmanagersMetrics", mock.Anything).Return(taskmanagerMetricsInstances, nil)
				mockClient.On("GetJobsMetrics", mock.Anything).Return(jobsMetricsInstances, nil)
				mockClient.On("GetSubtasksMetrics", mock.Anything).Return(nil, errors.New("some api error"))
				return &mockClient
			},
			expectedMetricFile: filepath.Join("testdata", "expected_metrics", "partial_metrics_no_subtasks.yaml"),
			expectedErr:        errors.New(subtasksFailedFetch + " some api error"),
		},
		{
			desc: "Successful Collection no jobs running",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				var jobsEmptyInstances []*models.JobMetrics
				var subtaskEmptyInstances []*models.SubtaskMetrics
				require.NoError(t, err)
				mockClient.On("GetJobmanagerMetrics", mock.Anything).Return(&jobmanagerMetrics, nil)
				mockClient.On("GetTaskmanagersMetrics", mock.Anything).Return(taskmanagerMetricsInstances, nil)
				mockClient.On("GetJobsMetrics", mock.Anything).Return(jobsEmptyInstances, nil)
				mockClient.On("GetSubtasksMetrics", mock.Anything).Return(subtaskEmptyInstances, nil)
				return &mockClient
			},
			expectedMetricFile: filepath.Join("testdata", "expected_metrics", "metrics_no_jobs_golden.yaml"),
			expectedErr:        nil,
		},
		{
			desc: "Successful Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}

				// mock client calls
				mockClient.On("GetJobmanagerMetrics", mock.Anything).Return(&jobmanagerMetrics, nil)
				mockClient.On("GetTaskmanagersMetrics", mock.Anything).Return(taskmanagerMetricsInstances, nil)
				mockClient.On("GetJobsMetrics", mock.Anything).Return(jobsMetricsInstances, nil)
				mockClient.On("GetSubtasksMetrics", mock.Anything).Return(subtaskMetricsInstances, nil)

				return &mockClient
			},
			expectedMetricFile: filepath.Join("testdata", "expected_metrics", "metrics_golden.yaml"),
			expectedErr:        nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			scraper := newflinkScraper(createDefaultConfig().(*Config), receivertest.NewNopCreateSettings())
			scraper.client = tc.setupMockClient(t)
			actualMetrics, err := scraper.scrape(context.Background())

			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}

			expectedMetrics, err := golden.ReadMetrics(tc.expectedMetricFile)
			require.NoError(t, err)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
				pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
		})
	}
}
