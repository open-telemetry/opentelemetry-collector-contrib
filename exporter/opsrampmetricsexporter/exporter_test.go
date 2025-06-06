// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampmetricsexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Define TestTime for v0.113.0 since componenttest.TestTime isn't available
var TestTime = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)

func TestCreateExporter(t *testing.T) {
	// Create a config
	cfg := createDefaultConfig().(*Config)
	// No need to set Endpoint, Token, TenantID as they're not part of the Config struct

	// Create an exporter
	exp, err := newOpsRampMetricsExporter(cfg, exportertest.NewNopSettings())
	require.NoError(t, err)
	require.NotNil(t, exp)

	// Ensure it can be shutdown
	err = exp.shutdown(context.Background())
	require.NoError(t, err)
}

func TestConvertToOpsRampMetrics_Gauge(t *testing.T) {
	// Create OTLP metrics
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("resource.key", "resource-value")
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()

	// Configure a gauge metric
	metric.SetName("test.gauge")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(42.0)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(TestTime))
	dp.Attributes().PutStr("attribute.key", "attribute-value")

	// Create exporter with test config
	cfg := createDefaultConfig().(*Config)
	cfg.ResourceToTelemetrySettings.Enabled = true
	exp, err := newOpsRampMetricsExporter(cfg, exportertest.NewNopSettings())
	require.NoError(t, err)

	// Convert to OpsRampMetric format
	opsrampMetrics, err := exp.convertToOpsRampMetrics(metrics)
	require.NoError(t, err)

	// Verify conversion
	require.Len(t, opsrampMetrics, 1)
	opsrampMetric := opsrampMetrics[0]
	assert.Equal(t, "test.gauge", opsrampMetric.MetricName)
	assert.Equal(t, float64(42.0), opsrampMetric.Value)
	assert.Equal(t, TestTime.UnixMilli(), opsrampMetric.Timestamp)
	assert.Equal(t, map[string]string{
		"attribute.key": "attribute-value",
		"resource.key":  "resource-value",
	}, opsrampMetric.Labels)
}

func TestConvertToOpsRampMetrics_Sum(t *testing.T) {
	// Create OTLP metrics
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()

	// Configure a sum metric
	metric.SetName("test.sum")
	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetIntValue(123)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(TestTime))

	// Create exporter with test config
	cfg := createDefaultConfig().(*Config)
	cfg.AddMetricSuffixes = true
	exp, err := newOpsRampMetricsExporter(cfg, exportertest.NewNopSettings())
	require.NoError(t, err)

	// Convert to OpsRampMetric format
	opsrampMetrics, err := exp.convertToOpsRampMetrics(metrics)
	require.NoError(t, err)

	// Verify conversion
	require.Len(t, opsrampMetrics, 1)
	opsrampMetric := opsrampMetrics[0]
	assert.Equal(t, "test.sum", opsrampMetric.MetricName)
	assert.Equal(t, float64(123), opsrampMetric.Value)
	assert.Equal(t, TestTime.UnixMilli(), opsrampMetric.Timestamp)
}

func TestConvertToOpsRampMetrics_Histogram(t *testing.T) {
	// Create OTLP metrics
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()

	// Configure a histogram metric
	metric.SetName("test.histogram")
	histogram := metric.SetEmptyHistogram()
	dp := histogram.DataPoints().AppendEmpty()
	dp.SetCount(100)
	dp.SetSum(500.5)
	dp.ExplicitBounds().FromRaw([]float64{0, 10, 20})
	dp.BucketCounts().FromRaw([]uint64{5, 15, 80})
	dp.SetTimestamp(pcommon.NewTimestampFromTime(TestTime))

	// Create exporter with test config
	cfg := createDefaultConfig().(*Config)
	cfg.AddMetricSuffixes = true
	exp, err := newOpsRampMetricsExporter(cfg, exportertest.NewNopSettings())
	require.NoError(t, err)

	// Convert to OpsRampMetric format
	opsrampMetrics, err := exp.convertToOpsRampMetrics(metrics)
	require.NoError(t, err)

	// Verify conversion
	require.GreaterOrEqual(t, len(opsrampMetrics), 2) // at least count and sum

	// Verify count metric
	countMetric := findMetric(opsrampMetrics, "test.histogram_count")
	require.NotNil(t, countMetric)
	assert.Equal(t, float64(100), countMetric.Value)

	// Verify sum metric
	sumMetric := findMetric(opsrampMetrics, "test.histogram_sum")
	require.NotNil(t, sumMetric)
	assert.Equal(t, 500.5, sumMetric.Value)
}

func TestConvertToOpsRampMetrics_Summary(t *testing.T) {
	// Create OTLP metrics
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()

	// Configure a summary metric
	metric.SetName("test.summary")
	summary := metric.SetEmptySummary()
	dp := summary.DataPoints().AppendEmpty()
	dp.SetCount(100)
	dp.SetSum(500.5)
	qp := dp.QuantileValues().AppendEmpty()
	qp.SetQuantile(0.5)
	qp.SetValue(42.0)
	qp = dp.QuantileValues().AppendEmpty()
	qp.SetQuantile(0.9)
	qp.SetValue(90.0)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(TestTime))

	// Create exporter with test config
	cfg := createDefaultConfig().(*Config)
	cfg.AddMetricSuffixes = true
	exp, err := newOpsRampMetricsExporter(cfg, exportertest.NewNopSettings())
	require.NoError(t, err)

	// Convert to OpsRampMetric format
	opsrampMetrics, err := exp.convertToOpsRampMetrics(metrics)
	require.NoError(t, err)

	// Verify conversion
	require.GreaterOrEqual(t, len(opsrampMetrics), 3) // at least count, sum, and one quantile

	// Verify count metric
	countMetric := findMetric(opsrampMetrics, "test.summary_count")
	require.NotNil(t, countMetric)
	assert.Equal(t, float64(100), countMetric.Value)

	// Verify sum metric
	sumMetric := findMetric(opsrampMetrics, "test.summary_sum")
	require.NotNil(t, sumMetric)
	assert.Equal(t, 500.5, sumMetric.Value)

	// Find a quantile metric that has quantile=0.5
	var q50Metric *OpsRampMetric
	for _, m := range opsrampMetrics {
		if val, ok := m.Labels["quantile"]; ok && val == "0.5" {
			q50Metric = &m
			break
		}
	}
	require.NotNil(t, q50Metric)
	assert.Equal(t, 42.0, q50Metric.Value)

	// Find a quantile metric that has quantile=0.9
	var q90Metric *OpsRampMetric
	for _, m := range opsrampMetrics {
		if val, ok := m.Labels["quantile"]; ok && val == "0.9" {
			q90Metric = &m
			break
		}
	}
	require.NotNil(t, q90Metric)
	assert.Equal(t, 90.0, q90Metric.Value)
}

// Helper function to find a metric by name
func findMetric(metrics []OpsRampMetric, name string) *OpsRampMetric {
	for _, m := range metrics {
		if m.MetricName == name {
			return &m
		}
	}
	return nil
}
