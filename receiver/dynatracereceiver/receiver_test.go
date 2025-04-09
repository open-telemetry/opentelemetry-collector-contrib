// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynatracereceiver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestReceiver_StartAndShutdown(t *testing.T) {
	cfg := &Config{
		APIEndpoint:     "http://example.com",
		APIToken:        "token",
		MetricSelectors: []string{"builtin:metric"},
		PollInterval:    10 * time.Millisecond,
		HTTPTimeout:     1 * time.Second,
	}
	dummyConsumer := &DummyConsumer{}
	receiver := &Receiver{
		Config:     cfg,
		NextMetric: dummyConsumer,
		httpClient: &http.Client{},
	}

	ctx := context.Background()
	err := receiver.Start(ctx, nil)
	assert.NoError(t, err, "Receiver should start without error")
	time.Sleep(50 * time.Millisecond)
	err = receiver.Shutdown(ctx)
	assert.NoError(t, err, "Receiver should shutdown")
}

type DummyConsumer struct{}

func (d *DummyConsumer) ConsumeMetrics(_ context.Context, _ pmetric.Metrics) error {
	return nil
}

func (d *DummyConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestPullDynatraceMetrics_Retry(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		if requestCount < 3 {
			http.Error(w, "failure", http.StatusInternalServerError)
			return
		}

		fmt.Fprintln(w, `{
			"totalCount": 1,
			"result": [{"metricId": "test.metric", "data": []}]
		}`)
	}))
	defer server.Close()
	cfg := &Config{
		APIEndpoint: server.URL,
		APIToken:    "dummy-token",
		MaxRetries:  5,
		HTTPTimeout: 2 * time.Second,
	}

	receiver := &Receiver{
		Config:     cfg,
		httpClient: server.Client(),
	}

	ctx := context.Background()
	metrics, err := receiver.pullDynatraceMetrics(ctx, cfg)
	assert.NoError(t, err)
	assert.Len(t, metrics, 1)
	assert.Equal(t, 3, requestCount, "Should retry twice before succeeding")
}

func TestFetchAllDynatraceMetrics(t *testing.T) {
	mockResponse := `{
		"totalCount": 1,
		"nextPageKey": null,
		"resolution": "1h",
		"result": [{
			"metricId": "builtin:containers.cpu.usageTime",
			"data": [{
				"timestamps": [1712203200000],
				"values": [55.0],
				"dimensions": ["container-xyz"],
				"dimensionMap": {
					"container": "testapp"
				}
			}]
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Authorization"), "Api-Token")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, mockResponse)
	}))
	defer server.Close()
	cfg := &Config{
		APIEndpoint:     server.URL,
		APIToken:        "dummy-token",
		MetricSelectors: []string{"builtin:containers.cpu.usageTime"},
		Resolution:      "1h",
		From:            "2025-04-01T00:00:00Z",
		To:              "2025-04-02T00:00:00Z",
		MaxRetries:      1,
		HTTPTimeout:     2 * time.Second,
	}

	receiver := &Receiver{
		Config:     cfg,
		httpClient: server.Client(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPTimeout)
	defer cancel()
	result, err := receiver.fetchAllDynatraceMetrics(ctx, cfg)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "builtin:containers.cpu.usageTime", result[0].MetricID)
	assert.Equal(t, 1, len(result[0].Data))
	assert.Equal(t, 55.0, result[0].Data[0].Values[0])
	assert.Equal(t, "testapp", result[0].Data[0].DimensionMap["container"])
}

func TestFetchAllDynatraceMetrics_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, "<html>Not JSON</html>")
	}))
	defer server.Close()
	cfg := &Config{
		APIEndpoint:     server.URL,
		APIToken:        "dummy-token",
		MetricSelectors: []string{"builtin:containers.cpu.usageTime"},
		Resolution:      "1h",
		From:            "2025-04-01T00:00:00Z",
		To:              "2025-04-02T00:00:00Z",
		HTTPTimeout:     2 * time.Second,
		MaxRetries:      1,
	}

	receiver := &Receiver{
		Config:     cfg,
		httpClient: server.Client(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPTimeout)
	defer cancel()
	_, err := receiver.fetchAllDynatraceMetrics(ctx, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "json unmarshal failed")
}

func TestFetchAllDynatraceMetrics_HttpError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer server.Close()
	cfg := &Config{
		APIEndpoint:     server.URL,
		APIToken:        "dummy-token",
		MetricSelectors: []string{"builtin:containers.cpu.usageTime"},
		Resolution:      "1h",
		From:            "2025-04-01T00:00:00Z",
		To:              "2025-04-02T00:00:00Z",
	}

	receiver := &Receiver{
		Config:     cfg,
		httpClient: server.Client(),
	}
	_, err := receiver.fetchAllDynatraceMetrics(context.Background(), cfg)
	assert.Error(t, err)
}

func TestFetchAllDynatraceMetrics_HTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond) // Delay to trigger timeout
		fmt.Fprintln(w, "{}")
	}))
	defer server.Close()
	cfg := &Config{
		APIEndpoint: server.URL,
		APIToken:    "dummy-token",
		HTTPTimeout: 50 * time.Millisecond, // Short timeout
		MaxRetries:  1,
	}

	receiver := &Receiver{
		Config:     cfg,
		httpClient: server.Client(),
	}

	ctx := context.Background()
	_, err := receiver.fetchAllDynatraceMetrics(ctx, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestConvertToMetricData(t *testing.T) {
	sample := []DynatraceMetricData{
		{
			MetricID: "builtin:containers.cpu.usageTime",
			Data: []MetricValues{
				{
					Timestamps: []int64{1712203200000},
					Values:     []float64{42.0},
					DimensionMap: map[string]string{
						"container": "example-container",
					},
				},
			},
		},
	}

	result := convertToMetricData(sample)

	metrics := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 1, metrics.Len(), "Expected one metric")

	metric := metrics.At(0)
	assert.Equal(t, "builtin:containers.cpu.usageTime", metric.Name())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	assert.Equal(t, 1, metric.Gauge().DataPoints().Len())
	assert.Equal(t, 42.0, metric.Gauge().DataPoints().At(0).DoubleValue())

	val, exists := metric.Gauge().DataPoints().At(0).Attributes().Get("container")
	assert.True(t, exists, "container attribute should exist")
	assert.Equal(t, "example-container", val.Str())
}
