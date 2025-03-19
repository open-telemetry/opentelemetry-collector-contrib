// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/models"
)

func TestScraperStart(t *testing.T) {
	clientConfigNonexistentCA := confighttp.NewDefaultClientConfig()
	clientConfigNonexistentCA.Endpoint = defaultEndpoint
	clientConfigNonexistentCA.TLSSetting = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "/non/existent",
		},
	}

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint

	testcases := []struct {
		desc        string
		scraper     *rabbitmqScraper
		expectError bool
	}{
		{
			desc: "Bad Config",
			scraper: &rabbitmqScraper{
				cfg: &Config{
					ClientConfig: clientConfigNonexistentCA,
				},
				settings: componenttest.NewNopTelemetrySettings(),
			},
			expectError: true,
		},
		{
			desc: "Valid Config",
			scraper: &rabbitmqScraper{
				cfg: &Config{
					ClientConfig: clientConfig,
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
	testCases := []struct {
		desc              string
		setupMockClient   func(t *testing.T) client
		expectedMetricGen func(t *testing.T) pmetric.Metrics
		expectedErr       error
	}{
		{
			desc: "Nil client",
			setupMockClient: func(*testing.T) client {
				return nil
			},
			expectedMetricGen: func(*testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errClientNotInit,
		},
		{
			desc: "API Call Failure",
			setupMockClient: func(*testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetQueues", mock.Anything).Return(nil, errors.New("some api error"))
				mockClient.On("GetNodes", mock.Anything).Return(nil, errors.New("some api error"))
				return &mockClient
			},
			expectedMetricGen: func(*testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: scrapererror.NewPartialScrapeError(
				errors.New("failed to collect queue metrics: some api error; failed to collect node metrics: some api error"),
				0, // No metrics were collected
			),
		},
		{
			desc: "Successful Queue Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				// use helper function from client tests
				data := loadAPIResponseData(t, queuesAPIResponseFile)
				var queues []*models.Queue
				err := json.Unmarshal(data, &queues)
				require.NoError(t, err)

				mockClient.On("GetQueues", mock.Anything).Return(queues, nil)
				mockClient.On("GetNodes", mock.Anything).Return(nil, nil)
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
		{
			desc: "Successful Node Metrics Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}

				// Mock data for nodes
				nodeData := loadAPIResponseData(t, nodesAPIResponseFile)
				var nodes []*models.Node
				err := json.Unmarshal(nodeData, &nodes)
				require.NoError(t, err)

				// Mock data for queues
				queueData := loadAPIResponseData(t, queuesAPIResponseFile)
				var queues []*models.Queue
				err = json.Unmarshal(queueData, &queues)
				require.NoError(t, err)

				// Mock client methods
				mockClient.On("GetNodes", mock.Anything).Return(nodes, nil)
				mockClient.On("GetQueues", mock.Anything).Return(queues, nil)

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

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			scraper := newScraper(zap.NewNop(), createDefaultConfig().(*Config), receivertest.NewNopSettings(metadata.Type))
			scraper.client = tc.setupMockClient(t)

			actualMetrics, err := scraper.scrape(context.Background())
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}

			expectedMetrics := tc.expectedMetricGen(t)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
				pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(),
			))
		})
	}
}
