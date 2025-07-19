// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
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

func TestRequestBodySupport(t *testing.T) {
	// Create a mock server that captures request details
	var receivedBody []byte
	var receivedMethod string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedContentType = r.Header.Get("Content-Type")

		body, err := io.ReadAll(r.Body)
		if err == nil {
			receivedBody = body
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, writeErr := w.Write([]byte(`{"result": "created"}`))
		assert.NoError(t, writeErr)
	}))
	defer server.Close()

	testCases := []struct {
		name                string
		method              string
		body                string
		expectedMethod      string
		expectedBody        string
		expectedContentType string
	}{
		{
			name:                "POST with JSON body",
			method:              "POST",
			body:                `{"name": "test", "email": "test@example.com"}`,
			expectedMethod:      "POST",
			expectedBody:        `{"name": "test", "email": "test@example.com"}`,
			expectedContentType: "application/json",
		},
		{
			name:                "PUT with JSON array",
			method:              "PUT",
			body:                `[{"id": 1, "name": "test"}]`,
			expectedMethod:      "PUT",
			expectedBody:        `[{"id": 1, "name": "test"}]`,
			expectedContentType: "application/json",
		},
		{
			name:                "POST with form data",
			method:              "POST",
			body:                `name=test&email=test@example.com`,
			expectedMethod:      "POST",
			expectedBody:        `name=test&email=test@example.com`,
			expectedContentType: "application/x-www-form-urlencoded",
		},
		{
			name:                "POST with plain text",
			method:              "POST",
			body:                `plain text content`,
			expectedMethod:      "POST",
			expectedBody:        `plain text content`,
			expectedContentType: "text/plain",
		},
		{
			name:                "PATCH with JSON",
			method:              "PATCH",
			body:                `{"status": "updated"}`,
			expectedMethod:      "PATCH",
			expectedBody:        `{"status": "updated"}`,
			expectedContentType: "application/json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset captured values
			receivedBody = nil
			receivedMethod = ""
			receivedContentType = ""

			cfg := createDefaultConfig().(*Config)
			cfg.Targets = []*targetConfig{
				{
					ClientConfig: confighttp.ClientConfig{
						Endpoint: server.URL,
					},
					Method: tc.method,
					Body:   tc.body,
				},
			}

			scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
			require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))

			_, err := scraper.scrape(context.Background())
			require.NoError(t, err)

			// Verify the request was made correctly
			assert.Equal(t, tc.expectedMethod, receivedMethod)
			assert.Equal(t, tc.expectedBody, string(receivedBody))
			assert.Equal(t, tc.expectedContentType, receivedContentType)
		})
	}
}

func TestRequestBodyWithCustomHeaders(t *testing.T) {
	// Create a mock server that captures request details
	var receivedContentType string
	var receivedCustomHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		receivedCustomHeader = r.Header.Get("X-Custom-Header")

		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"result": "ok"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	cfg := createDefaultConfig().(*Config)
	cfg.Targets = []*targetConfig{
		{
			ClientConfig: confighttp.ClientConfig{
				Endpoint: server.URL,
				Headers: map[string]configopaque.String{
					"Content-Type":    configopaque.String("application/custom+json"),
					"X-Custom-Header": configopaque.String("custom-value"),
				},
			},
			Method: "POST",
			Body:   `{"data": "test"}`,
		},
	}

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))

	_, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	// Verify custom Content-Type overrides auto-detection
	assert.Equal(t, "application/custom+json", receivedContentType)
	assert.Equal(t, "custom-value", receivedCustomHeader)
}

func TestAutoContentTypeConfiguration(t *testing.T) {
	// Create a mock server that captures request details
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")

		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"result": "ok"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	testCases := []struct {
		name                string
		body                string
		autoContentType     bool
		expectedContentType string
	}{
		{
			name:                "JSON body with auto_content_type enabled",
			body:                `{"key": "value"}`,
			autoContentType:     true,
			expectedContentType: "application/json",
		},
		{
			name:                "JSON body with auto_content_type disabled",
			body:                `{"key": "value"}`,
			autoContentType:     false,
			expectedContentType: "", // No Content-Type should be set
		},
		{
			name:                "Form data with auto_content_type enabled",
			body:                `name=test&email=test@example.com`,
			autoContentType:     true,
			expectedContentType: "application/x-www-form-urlencoded",
		},
		{
			name:                "Form data with auto_content_type disabled",
			body:                `name=test&email=test@example.com`,
			autoContentType:     false,
			expectedContentType: "", // No Content-Type should be set
		},
		{
			name:                "Plain text with auto_content_type enabled",
			body:                `plain text content`,
			autoContentType:     true,
			expectedContentType: "text/plain",
		},
		{
			name:                "Plain text with auto_content_type disabled",
			body:                `plain text content`,
			autoContentType:     false,
			expectedContentType: "", // No Content-Type should be set
		},
		{
			name:                "Empty body with auto_content_type enabled",
			body:                "",
			autoContentType:     true,
			expectedContentType: "", // No Content-Type should be set for empty body
		},
		{
			name:                "Empty body with auto_content_type disabled",
			body:                "",
			autoContentType:     false,
			expectedContentType: "", // No Content-Type should be set for empty body
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset captured values
			receivedContentType = ""

			cfg := createDefaultConfig().(*Config)
			cfg.Targets = []*targetConfig{
				{
					ClientConfig: confighttp.ClientConfig{
						Endpoint: server.URL,
					},
					Method:          "POST",
					Body:            tc.body,
					AutoContentType: tc.autoContentType,
				},
			}

			scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
			require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))

			_, err := scraper.scrape(context.Background())
			require.NoError(t, err)

			// Verify the Content-Type header behavior
			assert.Equal(t, tc.expectedContentType, receivedContentType)
		})
	}
}
