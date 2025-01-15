// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
)

func TestNewMetricsClient(t *testing.T) {
	t.Parallel()

	endpoint := "https://helix1:8080"
	apiKey := "api_key"

	cfg := confighttp.NewDefaultClientConfig()
	cfg.Endpoint = endpoint
	cfg.Timeout = 10 * time.Second

	ctx := context.Background()
	host := componenttest.NewNopHost()
	settings := componenttest.NewNopTelemetrySettings()
	config := &Config{
		ClientConfig: cfg,
		APIKey:       apiKey,
	}
	metricsClient, err := newMetricsClient(ctx, config, host, settings, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, metricsClient)

	assert.Equal(t, "https://helix1:8080/metrics-gateway-service/api/v1.0/insert", metricsClient.url)
	assert.Equal(t, apiKey, metricsClient.apiKey)
	assert.NotNil(t, metricsClient.httpClient)
}

func TestSendHelixPayload200(t *testing.T) {
	t.Parallel()

	// Mock payload
	sample := BmcHelixSample{
		Value:     42,
		Timestamp: 1634236000,
	}

	metric := BmcHelixMetric{
		Labels: map[string]string{
			"isDeviceMappingEnabled": "true",
			"entityTypeId":           "test-entity-type-id",
			"entityName":             "test-entity",
			"source":                 "OTEL",
			"unit":                   "ms",
			"hostType":               "server",
			"metricName":             "test_metric",
			"hostname":               "test-hostname",
			"instanceName":           "test-entity-Name",
			"entityId":               "OTEL:test-hostname:test-entity-type-id:test-entity",
			"parentEntityName":       "test-entity-type-id_container",
			"parentEntityTypeId":     "test-entity-type-id_container",
		},
		Samples: []BmcHelixSample{sample},
	}

	parent := BmcHelixMetric{
		Labels: map[string]string{
			"entityTypeId":           "test-entity-type-id_container",
			"entityName":             "test-entity-type-id_container",
			"isDeviceMappingEnabled": "true",
			"source":                 "OTEL",
			"hostType":               "server",
			"hostname":               "test-hostname",
			"entityId":               "OTEL:test-hostname:test-entity-type-id_container:test-entity-type-id_container",
			"metricName":             "identity",
		},
		Samples: []BmcHelixSample{},
	}

	payload := []BmcHelixMetric{parent, metric}

	apiKey := "apiKey"

	// Create a mock HTTP server
	mockServer := mockHTTPServer(t, apiKey, payload, http.StatusOK)
	defer mockServer.Close()

	cfg := confighttp.NewDefaultClientConfig()
	cfg.Endpoint = mockServer.URL
	cfg.Timeout = 10 * time.Second

	ctx := context.Background()
	host := componenttest.NewNopHost()
	settings := componenttest.NewNopTelemetrySettings()
	config := &Config{
		ClientConfig: cfg,
		APIKey:       apiKey,
	}
	client, err := newMetricsClient(ctx, config, host, settings, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Call SendHelixPayload
	err = client.SendHelixPayload(ctx, payload)
	assert.NoError(t, err)
}

func TestSendHelixPayloadEmpty(t *testing.T) {
	t.Parallel()

	cfg := confighttp.NewDefaultClientConfig()
	cfg.Endpoint = "https://helix1:8080"
	cfg.Timeout = 10 * time.Second

	ctx := context.Background()
	host := componenttest.NewNopHost()
	settings := componenttest.NewNopTelemetrySettings()
	config := &Config{
		ClientConfig: cfg,
		APIKey:       "apiKey",
	}
	client, err := newMetricsClient(ctx, config, host, settings, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Call SendHelixPayload
	err = client.SendHelixPayload(ctx, []BmcHelixMetric{})
	assert.NoError(t, err)
}

func TestSendHelixPayload400(t *testing.T) {
	t.Parallel()

	apiKey := "apiKey"
	payload := []BmcHelixMetric{
		{
			Labels:  map[string]string{},
			Samples: []BmcHelixSample{},
		},
	}

	// Create a mock HTTP server
	mockServer := mockHTTPServer(t, apiKey, payload, http.StatusBadRequest)
	defer mockServer.Close()

	cfg := confighttp.NewDefaultClientConfig()
	cfg.Endpoint = mockServer.URL
	cfg.Timeout = 10 * time.Second

	ctx := context.Background()
	host := componenttest.NewNopHost()
	settings := componenttest.NewNopTelemetrySettings()
	config := &Config{
		ClientConfig: cfg,
		APIKey:       apiKey,
	}
	client, err := newMetricsClient(ctx, config, host, settings, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Call SendHelixPayload
	err = client.SendHelixPayload(ctx, payload)
	assert.Error(t, err)
	assert.Equal(t, "received non-2xx response: 400", err.Error())
}

func TestSendHelixPayloadConnectionRefused(t *testing.T) {
	t.Parallel()

	cfg := confighttp.NewDefaultClientConfig()

	// Generate a random available port
	listener, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)
	listener.Close()

	randomPort := listener.Addr().(*net.TCPAddr).Port
	cfg.Endpoint = "https://localhost:" + strconv.Itoa(randomPort)
	cfg.Timeout = 500 * time.Millisecond

	ctx := context.Background()
	host := componenttest.NewNopHost()
	settings := componenttest.NewNopTelemetrySettings()
	config := &Config{
		ClientConfig: cfg,
		APIKey:       "apiKey",
	}
	client, err := newMetricsClient(ctx, config, host, settings, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, client)

	payload := []BmcHelixMetric{
		{
			Labels:  map[string]string{},
			Samples: []BmcHelixSample{},
		},
	}

	// Call SendHelixPayload
	err = client.SendHelixPayload(ctx, payload)
	assert.Error(t, err)
}

// mockHTTPServer creates a new mock HTTP server that verifies the request headers, body, and responds with the given status code
func mockHTTPServer(t *testing.T, apiKey string, payload []BmcHelixMetric, httpStatusCode int) *httptest.Server {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, r.Header.Get("Authorization"), "Bearer "+apiKey)

		// Verify the request body
		var receivedPayload []BmcHelixMetric
		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		assert.NoError(t, err)
		assert.NotEmpty(t, receivedPayload)
		assert.Equal(t, payload, receivedPayload)

		// Respond with a success status
		w.WriteHeader(httpStatusCode)
	}))
	return mockServer
}
