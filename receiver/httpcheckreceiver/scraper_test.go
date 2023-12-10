// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func newMockServer(t *testing.T, responseCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(responseCode)
		// This could be expanded if the checks for the server include
		// parsing the response content
		_, err := rw.Write([]byte(``))
		require.NoError(t, err)
	}))
}

func TestScraperStart(t *testing.T) {
	testcases := []struct {
		desc        string
		scraper     *httpcheckScraper
		expectError bool
	}{
		{
			desc: "Bad Config",
			scraper: &httpcheckScraper{
				cfg: &Config{
					Targets: []*targetConfig{
						{
							HTTPClientSettings: confighttp.HTTPClientSettings{
								Endpoint: "http://example.com",
								TLSSetting: configtls.TLSClientSetting{
									TLSSetting: configtls.TLSSetting{
										CAFile: "/non/existent",
									},
								},
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
			scraper: &httpcheckScraper{
				cfg: &Config{
					Targets: []*targetConfig{
						{
							HTTPClientSettings: confighttp.HTTPClientSettings{
								TLSSetting: configtls.TLSClientSetting{},
								Endpoint:   "http://example.com",
							},
						},
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
		expectedResponse  int
		expectedMetricGen func(t *testing.T) pmetric.Metrics
		expectedErr       error
		endpoint          string
		compareOptions    []pmetrictest.CompareMetricsOption
	}{
		{
			desc:             "Successful Collection",
			expectedResponse: 200,
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_golden.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: nil,
			compareOptions: []pmetrictest.CompareMetricsOption{
				pmetrictest.IgnoreMetricAttributeValue("http.url"),
				pmetrictest.IgnoreMetricValues("httpcheck.duration"),
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreTimestamp(),
			},
		},
		{
			desc:             "Endpoint returning 404",
			expectedResponse: 404,
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "endpoint_404.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: nil,
			compareOptions: []pmetrictest.CompareMetricsOption{
				pmetrictest.IgnoreMetricAttributeValue("http.url"),
				pmetrictest.IgnoreMetricValues("httpcheck.duration"),
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreTimestamp(),
			},
		},
		{
			desc:     "Invalid endpoint",
			endpoint: "http://invalid-endpoint",
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "invalid_endpoint.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: nil,
			compareOptions: []pmetrictest.CompareMetricsOption{
				pmetrictest.IgnoreMetricValues("httpcheck.duration"),
				pmetrictest.IgnoreMetricAttributeValue("error.message"),
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreTimestamp(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			if len(tc.endpoint) > 0 {
				cfg.Targets = []*targetConfig{{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: tc.endpoint,
					}},
				}
			} else {
				ms := newMockServer(t, tc.expectedResponse)
				defer ms.Close()
				cfg.Targets = []*targetConfig{{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: ms.URL,
					}},
				}
			}
			scraper := newScraper(cfg, receivertest.NewNopCreateSettings())
			require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))

			actualMetrics, err := scraper.scrape(context.Background())
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}

			expectedMetrics := tc.expectedMetricGen(t)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, tc.compareOptions...))
		})
	}
}

func TestNilClient(t *testing.T) {
	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopCreateSettings())
	actualMetrics, err := scraper.scrape(context.Background())
	require.EqualError(t, err, errClientNotInit.Error())
	require.NoError(t, pmetrictest.CompareMetrics(pmetric.NewMetrics(), actualMetrics))

}

func TestScraperMultipleTargets(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	ms1 := newMockServer(t, 200)
	defer ms1.Close()
	ms2 := newMockServer(t, 404)
	defer ms2.Close()

	cfg.Targets = append(cfg.Targets, &targetConfig{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: ms1.URL,
		},
	})
	cfg.Targets = append(cfg.Targets, &targetConfig{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: ms2.URL,
		},
	})

	scraper := newScraper(cfg, receivertest.NewNopCreateSettings())
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	goldenPath := filepath.Join("testdata", "expected_metrics", "multiple_targets.yaml")
	expectedMetrics, err := golden.ReadMetrics(goldenPath)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreMetricAttributeValue("http.url"),
		pmetrictest.IgnoreMetricValues("httpcheck.duration"),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(),
	))
}
