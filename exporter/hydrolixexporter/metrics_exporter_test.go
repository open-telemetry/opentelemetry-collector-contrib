// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hydrolixexporter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestMetricsExporter_ConvertGauge(t *testing.T) {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()

	metric.SetName("test.gauge")
	metric.SetDescription("Test gauge metric")
	metric.SetUnit("bytes")

	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(42.5)
	dp.Attributes().PutStr("label1", "value1")

	rm.Resource().Attributes().PutStr("service.name", "test-service")

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8080",
			Timeout:  30 * time.Second,
		},
		HDXTable:     "test_table",
		HDXTransform: "test_transform",
		HDXUsername:  "user",
		HDXPassword:  "pass",
	}

	exporter := newMetricsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	result := exporter.convertToHydrolixMetrics(metrics)

	require.Len(t, result, 1)
	assert.Equal(t, "test.gauge", result[0].Name)
	assert.Equal(t, "Test gauge metric", result[0].Description)
	assert.Equal(t, "bytes", result[0].Unit)
	assert.Equal(t, "gauge", result[0].MetricType)
	assert.Equal(t, 42.5, result[0].Value)
	assert.Equal(t, "test-service", result[0].ServiceName)
	assert.Greater(t, len(result[0].MetricAttributes), 0)
}

func TestMetricsExporter_ConvertSum(t *testing.T) {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()

	metric.SetName("test.sum")
	metric.SetDescription("Test sum metric")
	metric.SetUnit("requests")

	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Minute)))
	dp.SetIntValue(100)

	rm.Resource().Attributes().PutStr("service.name", "test-service")

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8080",
		},
	}

	exporter := newMetricsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	result := exporter.convertToHydrolixMetrics(metrics)

	require.Len(t, result, 1)
	assert.Equal(t, "test.sum", result[0].Name)
	assert.Equal(t, "sum", result[0].MetricType)
	assert.Equal(t, float64(100), result[0].Value)
	assert.Greater(t, result[0].StartTime, uint64(0))
}

func TestMetricsExporter_ConvertHistogram(t *testing.T) {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()

	metric.SetName("test.histogram")

	histogram := metric.SetEmptyHistogram()
	dp := histogram.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetCount(10)
	dp.SetSum(100.5)
	dp.SetMin(1.0)
	dp.SetMax(20.0)
	dp.ExplicitBounds().FromRaw([]float64{0, 5, 10, 25, 50})
	dp.BucketCounts().FromRaw([]uint64{2, 3, 2, 2, 1, 0})

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8080",
		},
	}

	exporter := newMetricsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	result := exporter.convertToHydrolixMetrics(metrics)

	require.Len(t, result, 1)
	assert.Equal(t, "test.histogram", result[0].Name)
	assert.Equal(t, "histogram", result[0].MetricType)
	assert.Equal(t, uint64(10), result[0].Count)
	assert.Equal(t, 100.5, result[0].Sum)
	assert.Equal(t, 1.0, result[0].Min)
	assert.Equal(t, 20.0, result[0].Max)
	assert.Len(t, result[0].BucketCounts, 6)
	assert.Len(t, result[0].ExplicitBounds, 5)
}

func TestMetricsExporter_PushMetrics(t *testing.T) {
	// Create a test server
	var receivedData []HydrolixMetric
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		err = json.Unmarshal(body, &receivedData)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test metrics
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()

	metric.SetName("test.metric")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(123.45)

	// Create exporter
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		HDXTable:     "test_table",
		HDXTransform: "test_transform",
		HDXUsername:  "testuser",
		HDXPassword:  "testpass",
	}

	exporter := newMetricsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))

	// Push metrics
	err := exporter.pushMetrics(context.Background(), metrics)
	require.NoError(t, err)

	// Verify received data
	require.Len(t, receivedData, 1)
	assert.Equal(t, "test.metric", receivedData[0].Name)
	assert.Equal(t, 123.45, receivedData[0].Value)

	// Verify headers
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
	assert.Equal(t, "test_table", receivedHeaders.Get("x-hdx-table"))
	assert.Equal(t, "test_transform", receivedHeaders.Get("x-hdx-transform"))
	assert.NotEmpty(t, receivedHeaders.Get("Authorization"))
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Basic")
}

func TestMetricsExporter_PushMetricsError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test.metric")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.0)

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		HDXTable:     "test_table",
		HDXTransform: "test_transform",
		HDXUsername:  "user",
		HDXPassword:  "pass",
	}

	exporter := newMetricsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	err := exporter.pushMetrics(context.Background(), metrics)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code")
}

func TestConvertBucketCounts(t *testing.T) {
	slice := pcommon.NewUInt64Slice()
	slice.FromRaw([]uint64{1, 2, 3, 4, 5})

	result := convertBucketCounts(slice)
	assert.Equal(t, []uint64{1, 2, 3, 4, 5}, result)
}

func TestConvertExplicitBounds(t *testing.T) {
	slice := pcommon.NewFloat64Slice()
	slice.FromRaw([]float64{0.1, 0.5, 1.0, 5.0, 10.0})

	result := convertExplicitBounds(slice)
	assert.Equal(t, []float64{0.1, 0.5, 1.0, 5.0, 10.0}, result)
}

func TestMetricsExporter_TraceContext(t *testing.T) {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()

	metric.SetName("test.metric")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(42.5)
	dp.Attributes().PutStr("trace_id", "fd700fd4e084e765d90a54a0412616b7")
	dp.Attributes().PutStr("span_id", "53e89fb8c3aaac01")

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8080",
		},
	}

	exporter := newMetricsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	result := exporter.convertToHydrolixMetrics(metrics)

	require.Len(t, result, 1)
	assert.Equal(t, "fd700fd4e084e765d90a54a0412616b7", result[0].TraceID)
	assert.Equal(t, "53e89fb8c3aaac01", result[0].SpanID)
}
