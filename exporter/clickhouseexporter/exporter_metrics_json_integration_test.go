// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

func testMetricsJSONExporter(t *testing.T, endpoint string) {
	overrideJSONStringSetting := func(config *Config) {
		config.ConnectionParams["output_format_native_write_json_as_string"] = "1"
	}
	overrideMetricsTableNames := func(config *Config) {
		config.JSON = true
		config.MetricsTables.Gauge.Name = "otel_metrics_json_gauge"
		config.MetricsTables.Sum.Name = "otel_metrics_json_sum"
		config.MetricsTables.Histogram.Name = "otel_metrics_json_histogram"
		config.MetricsTables.ExponentialHistogram.Name = "otel_metrics_json_exponential_histogram"
		config.MetricsTables.Summary.Name = "otel_metrics_json_summary"
	}

	exporter := newTestMetricsJSONExporter(t, endpoint, overrideJSONStringSetting, overrideMetricsTableNames)
	verifyExporterMetricsJSON(t, exporter)
}

func newTestMetricsJSONExporter(t *testing.T, dsn string, fns ...func(*Config)) *metricsExporter {
	exporter := newMetricsExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))

	require.NoError(t, exporter.start(t.Context(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(t.Context()) })
	return exporter
}

func verifyExporterMetricsJSON(t *testing.T, exporter *metricsExporter) {
	metric := pmetric.NewMetrics()
	rm := metric.ResourceMetrics().AppendEmpty()
	simpleMetrics(5000).ResourceMetrics().At(0).CopyTo(rm)

	pushConcurrentlyNoError(t, func() error {
		return exporter.pushMetricsData(t.Context(), metric)
	})

	expectedResourceAttrs := map[string]any{
		"Resource Attributes 1": "value1",
		"service": map[string]any{
			"name": "demo 1",
		},
	}
	expectedScopeAttrs := map[string]any{
		"Scope Attributes 1": "value1",
	}
	expectedExemplarAttrs := map[string]any{
		"key":  "value",
		"key2": "value2",
	}

	verifyGaugeMetricJSON(t, exporter, expectedResourceAttrs, expectedScopeAttrs, expectedExemplarAttrs)
	verifySumMetricJSON(t, exporter, expectedResourceAttrs, expectedScopeAttrs, expectedExemplarAttrs)
	verifyHistogramMetricJSON(t, exporter, expectedResourceAttrs, expectedScopeAttrs, expectedExemplarAttrs)
	verifyExpHistogramMetricJSON(t, exporter, expectedResourceAttrs, expectedScopeAttrs, expectedExemplarAttrs)
	verifySummaryMetricJSON(t, exporter, expectedResourceAttrs, expectedScopeAttrs)
}

type metricJSONRowWithExemplars struct {
	ResourceAttributes          string   `ch:"ResourceAttributes"`
	ResourceAttributesKeys      []string `ch:"ResourceAttributesKeys"`
	ScopeAttributes             string   `ch:"ScopeAttributes"`
	ScopeAttributesKeys         []string `ch:"ScopeAttributesKeys"`
	Attributes                  string   `ch:"Attributes"`
	AttributesKeys              []string `ch:"AttributesKeys"`
	ExemplarsFilteredAttributes []string `ch:"Exemplars.FilteredAttributes"`
}

type metricJSONRow struct {
	ResourceAttributes     string   `ch:"ResourceAttributes"`
	ResourceAttributesKeys []string `ch:"ResourceAttributesKeys"`
	ScopeAttributes        string   `ch:"ScopeAttributes"`
	ScopeAttributesKeys    []string `ch:"ScopeAttributesKeys"`
	Attributes             string   `ch:"Attributes"`
	AttributesKeys         []string `ch:"AttributesKeys"`
}

func verifyGaugeMetricJSON(t *testing.T, exporter *metricsExporter, expectedRes, expectedScope, expectedExemplar map[string]any) {
	tableName := fmt.Sprintf("%q.%q", exporter.cfg.database(), exporter.cfg.MetricsTables.Gauge.Name)
	row := exporter.db.QueryRow(t.Context(), fmt.Sprintf(
		"SELECT ResourceAttributes, ResourceAttributesKeys, ScopeAttributes, ScopeAttributesKeys, Attributes, AttributesKeys, Exemplars.FilteredAttributes FROM %s WHERE MetricName = 'gauge metrics' AND ServiceName = 'demo 1' LIMIT 1",
		tableName,
	))
	require.NoError(t, row.Err())

	var actual metricJSONRowWithExemplars
	require.NoError(t, row.ScanStruct(&actual))

	assertJSONEqual(t, expectedRes, actual.ResourceAttributes)
	require.Equal(t, []string{"Resource Attributes 1", "service.name"}, actual.ResourceAttributesKeys)
	assertJSONEqual(t, expectedScope, actual.ScopeAttributes)
	require.Equal(t, []string{"Scope Attributes 1"}, actual.ScopeAttributesKeys)
	assertJSONEqual(t, map[string]any{"gauge_label_1": "1"}, actual.Attributes)
	require.Equal(t, []string{"gauge_label_1"}, actual.AttributesKeys)
	require.Len(t, actual.ExemplarsFilteredAttributes, 1)
	assertJSONEqual(t, expectedExemplar, actual.ExemplarsFilteredAttributes[0])
}

func verifySumMetricJSON(t *testing.T, exporter *metricsExporter, expectedRes, expectedScope, expectedExemplar map[string]any) {
	tableName := fmt.Sprintf("%q.%q", exporter.cfg.database(), exporter.cfg.MetricsTables.Sum.Name)
	row := exporter.db.QueryRow(t.Context(), fmt.Sprintf(
		"SELECT ResourceAttributes, ResourceAttributesKeys, ScopeAttributes, ScopeAttributesKeys, Attributes, AttributesKeys, Exemplars.FilteredAttributes FROM %s WHERE MetricName = 'sum metrics' AND ServiceName = 'demo 1' LIMIT 1",
		tableName,
	))
	require.NoError(t, row.Err())

	var actual metricJSONRowWithExemplars
	require.NoError(t, row.ScanStruct(&actual))

	assertJSONEqual(t, expectedRes, actual.ResourceAttributes)
	require.Equal(t, []string{"Resource Attributes 1", "service.name"}, actual.ResourceAttributesKeys)
	assertJSONEqual(t, expectedScope, actual.ScopeAttributes)
	require.Equal(t, []string{"Scope Attributes 1"}, actual.ScopeAttributesKeys)
	assertJSONEqual(t, map[string]any{"sum_label_1": "1"}, actual.Attributes)
	require.Equal(t, []string{"sum_label_1"}, actual.AttributesKeys)
	require.Len(t, actual.ExemplarsFilteredAttributes, 1)
	assertJSONEqual(t, expectedExemplar, actual.ExemplarsFilteredAttributes[0])
}

func verifyHistogramMetricJSON(t *testing.T, exporter *metricsExporter, expectedRes, expectedScope, expectedExemplar map[string]any) {
	tableName := fmt.Sprintf("%q.%q", exporter.cfg.database(), exporter.cfg.MetricsTables.Histogram.Name)
	row := exporter.db.QueryRow(t.Context(), fmt.Sprintf(
		"SELECT ResourceAttributes, ResourceAttributesKeys, ScopeAttributes, ScopeAttributesKeys, Attributes, AttributesKeys, Exemplars.FilteredAttributes FROM %s WHERE MetricName = 'histogram metrics' AND ServiceName = 'demo 1' LIMIT 1",
		tableName,
	))
	require.NoError(t, row.Err())

	var actual metricJSONRowWithExemplars
	require.NoError(t, row.ScanStruct(&actual))

	assertJSONEqual(t, expectedRes, actual.ResourceAttributes)
	require.Equal(t, []string{"Resource Attributes 1", "service.name"}, actual.ResourceAttributesKeys)
	assertJSONEqual(t, expectedScope, actual.ScopeAttributes)
	require.Equal(t, []string{"Scope Attributes 1"}, actual.ScopeAttributesKeys)
	assertJSONEqual(t, map[string]any{"key": "value", "key2": "value"}, actual.Attributes)
	require.Equal(t, []string{"key", "key2"}, actual.AttributesKeys)
	require.Len(t, actual.ExemplarsFilteredAttributes, 1)
	assertJSONEqual(t, expectedExemplar, actual.ExemplarsFilteredAttributes[0])
}

func verifyExpHistogramMetricJSON(t *testing.T, exporter *metricsExporter, expectedRes, expectedScope, expectedExemplar map[string]any) {
	tableName := fmt.Sprintf("%q.%q", exporter.cfg.database(), exporter.cfg.MetricsTables.ExponentialHistogram.Name)
	row := exporter.db.QueryRow(t.Context(), fmt.Sprintf(
		"SELECT ResourceAttributes, ResourceAttributesKeys, ScopeAttributes, ScopeAttributesKeys, Attributes, AttributesKeys, Exemplars.FilteredAttributes FROM %s WHERE MetricName = 'exp histogram metrics' AND ServiceName = 'demo 1' LIMIT 1",
		tableName,
	))
	require.NoError(t, row.Err())

	var actual metricJSONRowWithExemplars
	require.NoError(t, row.ScanStruct(&actual))

	assertJSONEqual(t, expectedRes, actual.ResourceAttributes)
	require.Equal(t, []string{"Resource Attributes 1", "service.name"}, actual.ResourceAttributesKeys)
	assertJSONEqual(t, expectedScope, actual.ScopeAttributes)
	require.Equal(t, []string{"Scope Attributes 1"}, actual.ScopeAttributesKeys)
	assertJSONEqual(t, map[string]any{"key": "value", "key2": "value"}, actual.Attributes)
	require.Equal(t, []string{"key", "key2"}, actual.AttributesKeys)
	require.Len(t, actual.ExemplarsFilteredAttributes, 1)
	assertJSONEqual(t, expectedExemplar, actual.ExemplarsFilteredAttributes[0])
}

func verifySummaryMetricJSON(t *testing.T, exporter *metricsExporter, expectedRes, expectedScope map[string]any) {
	tableName := fmt.Sprintf("%q.%q", exporter.cfg.database(), exporter.cfg.MetricsTables.Summary.Name)
	row := exporter.db.QueryRow(t.Context(), fmt.Sprintf(
		"SELECT ResourceAttributes, ResourceAttributesKeys, ScopeAttributes, ScopeAttributesKeys, Attributes, AttributesKeys FROM %s WHERE MetricName = 'summary metrics' AND ServiceName = 'demo 1' LIMIT 1",
		tableName,
	))
	require.NoError(t, row.Err())

	var actual metricJSONRow
	require.NoError(t, row.ScanStruct(&actual))

	assertJSONEqual(t, expectedRes, actual.ResourceAttributes)
	require.Equal(t, []string{"Resource Attributes 1", "service.name"}, actual.ResourceAttributesKeys)
	assertJSONEqual(t, expectedScope, actual.ScopeAttributes)
	require.Equal(t, []string{"Scope Attributes 1"}, actual.ScopeAttributesKeys)
	assertJSONEqual(t, map[string]any{"key": "value", "key2": "value"}, actual.Attributes)
	require.Equal(t, []string{"key", "key2"}, actual.AttributesKeys)
}

func assertJSONEqual(t *testing.T, expected map[string]any, actualJSON string) {
	t.Helper()

	var actual map[string]any
	require.NoError(t, json.Unmarshal([]byte(actualJSON), &actual))
	require.Equal(t, expected, actual)
}
