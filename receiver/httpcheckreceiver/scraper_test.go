// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver/internal/metadata"
)

func newMockServer(t *testing.T, responseCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(responseCode)
		// This could be expanded if the checks for the server include
		// parsing the response content
		_, err := rw.Write([]byte(``))
		assert.NoError(t, err)
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
							ClientConfig: confighttp.ClientConfig{
								Endpoint: "http://example.com",
								TLS: configtls.ClientConfig{
									Config: configtls.Config{
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
							ClientConfig: confighttp.ClientConfig{
								TLS:      configtls.ClientConfig{},
								Endpoint: "http://example.com",
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

func TestScraperScrape(t *testing.T) {
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
			if tc.endpoint != "" {
				cfg.Targets = []*targetConfig{
					{
						ClientConfig: confighttp.ClientConfig{
							Endpoint: tc.endpoint,
						},
					},
				}
			} else {
				ms := newMockServer(t, tc.expectedResponse)
				defer ms.Close()
				cfg.Targets = []*targetConfig{
					{
						ClientConfig: confighttp.ClientConfig{
							Endpoint: ms.URL,
						},
					},
				}
			}
			scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
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

func TestExtractTLSInfo(t *testing.T) {
	testCases := []struct {
		desc             string
		state            *tls.ConnectionState
		expectIssuer     string
		expectCommonName string
		expectTimeLeft   int64
		expectSANCount   int
	}{
		{
			desc:             "nil connection state",
			state:            nil,
			expectIssuer:     "",
			expectCommonName: "",
			expectTimeLeft:   0,
			expectSANCount:   0,
		},
		{
			desc: "empty peer certificates",
			state: &tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{},
			},
			expectIssuer:     "",
			expectCommonName: "",
			expectTimeLeft:   0,
			expectSANCount:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			issuer, commonName, sans, timeLeft := extractTLSInfo(tc.state)
			assert.Equal(t, tc.expectIssuer, issuer)
			assert.Equal(t, tc.expectCommonName, commonName)
			assert.Equal(t, tc.expectTimeLeft, timeLeft)
			assert.Len(t, sans, tc.expectSANCount)
		})
	}
}

func TestHTTPSWithTLS(t *testing.T) {
	// Create an HTTPS test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := createDefaultConfig().(*Config)
	// Explicitly enable the TLS metric (to test the opt-in behavior)
	cfg.Metrics.HttpcheckTLSCertRemaining.Enabled = true
	cfg.Targets = []*targetConfig{
		{
			ClientConfig: confighttp.ClientConfig{
				Endpoint: server.URL,
				TLS: configtls.ClientConfig{
					InsecureSkipVerify: true, // Skip verification for test server
				},
			},
		},
	}

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))

	metrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	// Check that we have metrics
	require.Positive(t, metrics.ResourceMetrics().Len())
	rm := metrics.ResourceMetrics().At(0)
	ilm := rm.ScopeMetrics().At(0)

	// Find the TLS certificate metric
	found := false
	for i := 0; i < ilm.Metrics().Len(); i++ {
		metric := ilm.Metrics().At(i)
		if metric.Name() == "httpcheck.tls.cert_remaining" {
			found = true
			assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
			dp := metric.Gauge().DataPoints().At(0)

			// Check attributes
			attrs := dp.Attributes()
			urlVal, ok := attrs.Get("http.url")
			assert.True(t, ok)
			assert.Equal(t, server.URL, urlVal.Str())

			// The test server cert should have some time left
			assert.Positive(t, dp.IntValue())
			break
		}
	}
	assert.True(t, found, "TLS certificate metric not found")
}

func TestHTTPSWithTLSDisabled(t *testing.T) {
	// Create an HTTPS test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := createDefaultConfig().(*Config)
	// Explicitly disable the TLS metric
	cfg.Metrics.HttpcheckTLSCertRemaining.Enabled = false
	cfg.Targets = []*targetConfig{
		{
			ClientConfig: confighttp.ClientConfig{
				Endpoint: server.URL,
				TLS: configtls.ClientConfig{
					InsecureSkipVerify: true, // Skip verification for test server
				},
			},
		},
	}

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))

	metrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	// Check that we have metrics but no TLS metric
	require.Positive(t, metrics.ResourceMetrics().Len())
	rm := metrics.ResourceMetrics().At(0)
	ilm := rm.ScopeMetrics().At(0)

	// Ensure TLS certificate metric is NOT present
	for i := 0; i < ilm.Metrics().Len(); i++ {
		metric := ilm.Metrics().At(i)
		assert.NotEqual(t, "httpcheck.tls.cert_remaining", metric.Name())
	}
}

func TestNilClient(t *testing.T) {
	scraper := newScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(metadata.Type))
	actualMetrics, err := scraper.scrape(context.Background())
	require.EqualError(t, err, errClientNotInit.Error())
	require.NoError(t, pmetrictest.CompareMetrics(pmetric.NewMetrics(), actualMetrics))
}

func TestStatusCodeConditionalInclusion(t *testing.T) {
	// Test that http.status_code is only included when the metric value is 1
	testCases := []struct {
		name           string
		responseCode   int
		expectedChecks func(t *testing.T, metrics pmetric.Metrics)
	}{
		{
			name:         "200 OK response",
			responseCode: 200,
			expectedChecks: func(t *testing.T, metrics pmetric.Metrics) {
				// Find status metrics
				rm := metrics.ResourceMetrics().At(0)
				ilm := rm.ScopeMetrics().At(0)

				statusMetricsFound := 0
				for i := 0; i < ilm.Metrics().Len(); i++ {
					metric := ilm.Metrics().At(i)
					if metric.Name() == "httpcheck.status" {
						dps := metric.Sum().DataPoints()
						for j := 0; j < dps.Len(); j++ {
							dp := dps.At(j)
							statusClass, _ := dp.Attributes().Get("http.status_class")

							if dp.IntValue() == 1 {
								// When value is 1, status_code should be present and be 200
								statusCode, ok := dp.Attributes().Get("http.status_code")
								assert.True(t, ok, "http.status_code should be present when value is 1")
								assert.Equal(t, int64(200), statusCode.Int())
								assert.Equal(t, "2xx", statusClass.Str())
							} else {
								// When value is 0, status_code should NOT be present
								_, ok := dp.Attributes().Get("http.status_code")
								assert.False(t, ok, "http.status_code should NOT be present when value is 0 for class %s", statusClass.Str())
							}
							statusMetricsFound++
						}
					}
				}
				assert.Equal(t, 5, statusMetricsFound, "Should have 5 status data points (one for each class)")
			},
		},
		{
			name:         "404 Not Found response",
			responseCode: 404,
			expectedChecks: func(t *testing.T, metrics pmetric.Metrics) {
				rm := metrics.ResourceMetrics().At(0)
				ilm := rm.ScopeMetrics().At(0)

				statusMetricsFound := 0
				for i := 0; i < ilm.Metrics().Len(); i++ {
					metric := ilm.Metrics().At(i)
					if metric.Name() == "httpcheck.status" {
						dps := metric.Sum().DataPoints()
						for j := 0; j < dps.Len(); j++ {
							dp := dps.At(j)
							statusClass, _ := dp.Attributes().Get("http.status_class")

							if dp.IntValue() == 1 {
								// When value is 1, status_code should be present and be 404
								statusCode, ok := dp.Attributes().Get("http.status_code")
								assert.True(t, ok, "http.status_code should be present when value is 1")
								assert.Equal(t, int64(404), statusCode.Int())
								assert.Equal(t, "4xx", statusClass.Str())
							} else {
								// When value is 0, status_code should NOT be present
								_, ok := dp.Attributes().Get("http.status_code")
								assert.False(t, ok, "http.status_code should NOT be present when value is 0 for class %s", statusClass.Str())
							}
							statusMetricsFound++
						}
					}
				}
				assert.Equal(t, 5, statusMetricsFound, "Should have 5 status data points (one for each class)")
			},
		},
		{
			name:         "500 Server Error response",
			responseCode: 500,
			expectedChecks: func(t *testing.T, metrics pmetric.Metrics) {
				rm := metrics.ResourceMetrics().At(0)
				ilm := rm.ScopeMetrics().At(0)

				statusMetricsFound := 0
				for i := 0; i < ilm.Metrics().Len(); i++ {
					metric := ilm.Metrics().At(i)
					if metric.Name() == "httpcheck.status" {
						dps := metric.Sum().DataPoints()
						for j := 0; j < dps.Len(); j++ {
							dp := dps.At(j)
							statusClass, _ := dp.Attributes().Get("http.status_class")

							if dp.IntValue() == 1 {
								// When value is 1, status_code should be present and be 500
								statusCode, ok := dp.Attributes().Get("http.status_code")
								assert.True(t, ok, "http.status_code should be present when value is 1")
								assert.Equal(t, int64(500), statusCode.Int())
								assert.Equal(t, "5xx", statusClass.Str())
							} else {
								// When value is 0, status_code should NOT be present
								_, ok := dp.Attributes().Get("http.status_code")
								assert.False(t, ok, "http.status_code should NOT be present when value is 0 for class %s", statusClass.Str())
							}
							statusMetricsFound++
						}
					}
				}
				assert.Equal(t, 5, statusMetricsFound, "Should have 5 status data points (one for each class)")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ms := newMockServer(t, tc.responseCode)
			defer ms.Close()

			cfg := createDefaultConfig().(*Config)
			cfg.Targets = []*targetConfig{
				{
					ClientConfig: confighttp.ClientConfig{
						Endpoint: ms.URL,
					},
				},
			}

			scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
			require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))

			actualMetrics, err := scraper.scrape(context.Background())
			require.NoError(t, err)

			tc.expectedChecks(t, actualMetrics)
		})
	}
}

func TestScraperMultipleTargets(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	ms1 := newMockServer(t, 200)
	defer ms1.Close()
	ms2 := newMockServer(t, 404)
	defer ms2.Close()

	cfg.Targets = append(cfg.Targets,
		&targetConfig{
			ClientConfig: confighttp.ClientConfig{
				Endpoint: ms1.URL,
			},
		},
		&targetConfig{
			ClientConfig: confighttp.ClientConfig{
				Endpoint: ms2.URL,
			},
		})

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
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
