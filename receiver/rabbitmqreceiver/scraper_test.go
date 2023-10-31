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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/models"
)

func TestScraperStart(t *testing.T) {
	testcases := []struct {
		desc        string
		scraper     *rabbitmqScraper
		expectError bool
	}{
		{
			desc: "Bad Config",
			scraper: &rabbitmqScraper{
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
			scraper: &rabbitmqScraper{
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

func TestScaperScrape(t *testing.T) {
	testCases := []struct {
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
			desc: "API Call Failure",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetQueues", mock.Anything).Return(nil, errors.New("some api error"))
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {

				return pmetric.NewMetrics()
			},
			expectedErr: errors.New("some api error"),
		},
		{
			desc: "Successful Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				// use helper function from client tests
				data := loadAPIResponseData(t, queuesAPIResponseFile)
				var queues []*models.Queue
				err := json.Unmarshal(data, &queues)
				require.NoError(t, err)

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
			scraper := newScraper(zap.NewNop(), createDefaultConfig().(*Config), receivertest.NewNopCreateSettings())
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
