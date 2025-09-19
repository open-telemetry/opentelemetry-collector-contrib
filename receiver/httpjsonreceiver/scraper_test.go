package httpjsonreceiver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestScraper(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
            "temperature": 23.5,
            "humidity": 65,
            "status": "ok",
            "sensors": {
                "count": 5,
                "active": true
            },
            "readings": [10, 20, 30]
        }`)
	}))
	defer server.Close()

	cfg := &Config{
		ResourceAttributes: map[string]string{
			"service": "test",
		},
		Endpoints: []EndpointConfig{
			{
				URL:    server.URL,
				Method: "GET",
				Headers: map[string]string{
					"User-Agent": "test-agent",
				},
				Metrics: []MetricConfig{
					{
						Name:        "temperature",
						JSONPath:    "temperature",
						Type:        "gauge",
						Description: "Temperature reading",
						Unit:        "C",
						ValueType:   "double",
						Attributes: map[string]string{
							"sensor": "temp1",
						},
					},
					{
						Name:      "humidity",
						JSONPath:  "humidity",
						Type:      "gauge",
						ValueType: "int",
					},
					{
						Name:     "sensor_count",
						JSONPath: "sensors.count",
						Type:     "gauge",
					},
					{
						Name:     "active_status",
						JSONPath: "sensors.active",
						Type:     "gauge",
					},
					{
						Name:     "readings_count",
						JSONPath: "readings.#",
						Type:     "counter",
					},
				},
			},
		},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := zaptest.NewLogger(t)
	scraper := NewScraper(cfg, client, logger)

	ctx := context.Background()
	metrics, err := scraper.Scrape(ctx)

	require.NoError(t, err)
	require.NotNil(t, metrics)

	// Verify metrics structure - Fixed method names
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())

	rm := metrics.ResourceMetrics().At(0)
	resource := rm.Resource()

	// Check resource attributes
	receiverAttr, exists := resource.Attributes().Get("receiver")
	assert.True(t, exists)
	assert.Equal(t, "httpjson", receiverAttr.Str())

	serviceAttr, exists := resource.Attributes().Get("service")
	assert.True(t, exists)
	assert.Equal(t, "test", serviceAttr.Str())

	// Verify scope metrics
	assert.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)

	scope := sm.Scope()
	assert.Equal(t, "httpjsonreceiver", scope.Name())
	assert.Equal(t, "1.0.0", scope.Version())

	// Verify metrics count
	assert.Equal(t, 5, sm.Metrics().Len())

	// Verify specific metrics
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		switch metric.Name() {
		case "temperature":
			assert.Equal(t, "Temperature reading", metric.Description())
			assert.Equal(t, "C", metric.Unit())

			gauge := metric.Gauge()
			assert.Equal(t, 1, gauge.DataPoints().Len())

			dp := gauge.DataPoints().At(0)
			assert.Equal(t, 23.5, dp.DoubleValue())

			// Check attributes
			sensorAttr, exists := dp.Attributes().Get("sensor")
			assert.True(t, exists)
			assert.Equal(t, "temp1", sensorAttr.Str())

		case "humidity":
			gauge := metric.Gauge()
			dp := gauge.DataPoints().At(0)
			assert.Equal(t, int64(65), dp.IntValue())

		case "sensor_count":
			gauge := metric.Gauge()
			dp := gauge.DataPoints().At(0)
			assert.Equal(t, 5.0, dp.DoubleValue())

		case "active_status":
			gauge := metric.Gauge()
			dp := gauge.DataPoints().At(0)
			assert.Equal(t, 1.0, dp.DoubleValue()) // true -> 1.0

		case "readings_count":
			sum := metric.Sum()
			assert.True(t, sum.IsMonotonic())
			dp := sum.DataPoints().At(0)
			assert.Equal(t, 3.0, dp.DoubleValue()) // Array length
		}
	}
}

func TestScraperHTTPError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &Config{
		Endpoints: []EndpointConfig{
			{
				URL: server.URL,
				Metrics: []MetricConfig{
					{
						Name:     "test_metric",
						JSONPath: "value",
					},
				},
			},
		},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := zaptest.NewLogger(t)
	scraper := NewScraper(cfg, client, logger)

	ctx := context.Background()
	metrics, err := scraper.Scrape(ctx)

	// Should return metrics even if one endpoint fails
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// But no metrics should be collected
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	assert.Equal(t, 0, sm.Metrics().Len())
}

func TestScraperInvalidJSON(t *testing.T) {
	// Create test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "invalid json")
	}))
	defer server.Close()

	cfg := &Config{
		Endpoints: []EndpointConfig{
			{
				URL: server.URL,
				Metrics: []MetricConfig{
					{
						Name:     "test_metric",
						JSONPath: "value",
					},
				},
			},
		},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := zaptest.NewLogger(t)
	scraper := NewScraper(cfg, client, logger)

	ctx := context.Background()
	metrics, err := scraper.Scrape(ctx)

	// Should return metrics even if parsing fails
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// But no metrics should be collected
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	assert.Equal(t, 0, sm.Metrics().Len())
}

func TestScraperJSONPathNotFound(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"temperature": 23.5}`)
	}))
	defer server.Close()

	cfg := &Config{
		Endpoints: []EndpointConfig{
			{
				URL: server.URL,
				Metrics: []MetricConfig{
					{
						Name:     "humidity", // This path doesn't exist
						JSONPath: "humidity",
						Type:     "gauge",
					},
					{
						Name:     "temperature", // This path exists
						JSONPath: "temperature",
						Type:     "gauge",
					},
				},
			},
		},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := zaptest.NewLogger(t)
	scraper := NewScraper(cfg, client, logger)

	ctx := context.Background()
	metrics, err := scraper.Scrape(ctx)

	require.NoError(t, err)
	require.NotNil(t, metrics)

	// Should collect the valid metric only
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	assert.Equal(t, 1, sm.Metrics().Len())

	metric := sm.Metrics().At(0)
	assert.Equal(t, "temperature", metric.Name())
}

func TestScraperPOSTRequest(t *testing.T) {
	// Create test server that expects POST
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-value", r.Header.Get("X-Custom-Header"))

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"result": 1}`) // Changed from "success" to 1
	}))
	defer server.Close()

	cfg := &Config{
		Endpoints: []EndpointConfig{
			{
				URL:    server.URL,
				Method: "POST",
				Headers: map[string]string{
					"X-Custom-Header": "test-value",
				},
				Body: `{"query": "test"}`,
				Metrics: []MetricConfig{
					{
						Name:     "success",
						JSONPath: "result",
						Type:     "gauge",
					},
				},
			},
		},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := zaptest.NewLogger(t)
	scraper := NewScraper(cfg, client, logger)

	ctx := context.Background()
	metrics, err := scraper.Scrape(ctx)

	require.NoError(t, err)
	require.NotNil(t, metrics)

	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	assert.Equal(t, 1, sm.Metrics().Len())
}

func TestScraperTimeout(t *testing.T) {
	// Create test server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintln(w, `{"value": 1}`)
	}))
	defer server.Close()

	cfg := &Config{
		Endpoints: []EndpointConfig{
			{
				URL:     server.URL,
				Timeout: 50 * time.Millisecond, // Shorter than server delay
				Metrics: []MetricConfig{
					{
						Name:     "test_metric",
						JSONPath: "value",
					},
				},
			},
		},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := zaptest.NewLogger(t)
	scraper := NewScraper(cfg, client, logger)

	ctx := context.Background()
	metrics, err := scraper.Scrape(ctx)

	// Should return metrics even if request times out
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// But no metrics should be collected due to timeout
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	assert.Equal(t, 0, sm.Metrics().Len())
}
