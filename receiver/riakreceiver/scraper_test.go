// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package riakreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver"

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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/model"
)

func TestScraperStart(t *testing.T) {
	testcases := []struct {
		desc        string
		scraper     *riakScraper
		expectError bool
	}{
		{
			desc: "Bad Config",
			scraper: &riakScraper{
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
			scraper: &riakScraper{
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
		setupCfg          func() *Config
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
			setupCfg: func() *Config {
				return createDefaultConfig().(*Config)
			},
			expectedErr: errClientNotInit,
		},
		{
			desc: "API Call Failure",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetStats", mock.Anything).Return(nil, errors.New("some api error"))
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			setupCfg: func() *Config {
				return createDefaultConfig().(*Config)
			},
			expectedErr: errors.New("some api error"),
		},
		{
			desc: "Metrics Disabled",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				// use helper function from client tests
				data := loadAPIResponseData(t, statsAPIResponseFile)
				var stats *model.Stats
				err := json.Unmarshal(data, &stats)
				require.NoError(t, err)

				mockClient.On("GetStats", mock.Anything).Return(stats, nil)
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "expected_disabled.json")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			setupCfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Metrics = metadata.MetricsSettings{
					RiakMemoryLimit: metadata.MetricSettings{
						Enabled: false,
					},
					RiakNodeOperationCount: metadata.MetricSettings{
						Enabled: false,
					},
					RiakNodeOperationTimeMean: metadata.MetricSettings{
						Enabled: true,
					},
					RiakNodeReadRepairCount: metadata.MetricSettings{
						Enabled: true,
					},
					RiakVnodeIndexOperationCount: metadata.MetricSettings{
						Enabled: true,
					},
					RiakVnodeOperationCount: metadata.MetricSettings{
						Enabled: true,
					},
				}
				return cfg
			},
			expectedErr: nil,
		},
		{
			desc: "Successful Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				// use helper function from client tests
				data := loadAPIResponseData(t, statsAPIResponseFile)
				var stats *model.Stats
				err := json.Unmarshal(data, &stats)
				require.NoError(t, err)

				mockClient.On("GetStats", mock.Anything).Return(stats, nil)
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "expected.json")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			setupCfg: func() *Config {
				return createDefaultConfig().(*Config)
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			scraper := newScraper(zap.NewNop(), tc.setupCfg(), componenttest.NewNopTelemetrySettings())
			scraper.client = tc.setupMockClient(t)
			actualMetrics, err := scraper.scrape(context.Background())
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}

			expectedMetrics := tc.expectedMetricGen(t)

			err = scrapertest.CompareMetrics(expectedMetrics, actualMetrics)
			require.NoError(t, err)
		})
	}
}
