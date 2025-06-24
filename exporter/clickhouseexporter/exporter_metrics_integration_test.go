// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

func testMetricsExporter(t *testing.T, endpoint string) {
	exporter := newTestMetricsExporter(t, endpoint)
	verifyExporterMetrics(t, exporter)
}

func newTestMetricsExporter(t *testing.T, dsn string, fns ...func(*Config)) *metricsExporter {
	exporter := newMetricsExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))

	require.NoError(t, exporter.start(context.Background(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.Background()) })
	return exporter
}

func verifyExporterMetrics(t *testing.T, exporter *metricsExporter) {
	metric := pmetric.NewMetrics()
	rm := metric.ResourceMetrics().AppendEmpty()
	simpleMetrics(5000).ResourceMetrics().At(0).CopyTo(rm)

	// 3 pushes
	mustPushMetricsData(t, exporter, metric)
	mustPushMetricsData(t, exporter, metric)
	mustPushMetricsData(t, exporter, metric)

	verifyGaugeMetric(t, exporter)
	verifySumMetric(t, exporter)
	verifyHistogramMetric(t, exporter)
	verifyExphistogramMetric(t, exporter)
	verifySummaryMetric(t, exporter)
}

func mustPushMetricsData(t *testing.T, exporter *metricsExporter, md pmetric.Metrics) {
	err := exporter.pushMetricsData(context.Background(), md)
	require.NoError(t, err)
}

// simpleMetrics there will be added two ResourceMetrics and each of them have count data point
func simpleMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "demo 1")
	rm.Resource().Attributes().PutStr("Resource Attributes 1", "value1")
	rm.Resource().SetDroppedAttributesCount(10)
	rm.SetSchemaUrl("Resource SchemaUrl 1")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 1")
	sm.Scope().Attributes().PutStr("Scope Attributes 1", "value1")
	sm.Scope().SetDroppedAttributesCount(10)
	sm.Scope().SetName("Scope name 1")
	sm.Scope().SetVersion("Scope version 1")
	timestamp := time.Unix(1703498029, 0)
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.SetFlags(pmetric.DefaultDataPointFlags)
		dp.Attributes().PutStr("gauge_label_1", "1")
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetDoubleValue(11.234)
		dp.SetFlags(pmetric.DefaultDataPointFlags)
		dp.Attributes().PutStr("sum_label_1", "1")
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key2", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars.SetDoubleValue(55.22)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key2", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}

	rm = metrics.ResourceMetrics().AppendEmpty()
	// Removed service.name from second metric to test both with/without ServiceName cases
	rm.Resource().Attributes().PutStr("Resource Attributes 2", "value2")
	rm.Resource().SetDroppedAttributesCount(20)
	rm.SetSchemaUrl("Resource SchemaUrl 2")
	sm = rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 2")
	sm.Scope().Attributes().PutStr("Scope Attributes 2", "value2")
	sm.Scope().SetDroppedAttributesCount(20)
	sm.Scope().SetName("Scope name 2")
	sm.Scope().SetVersion("Scope version 2")
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("gauge_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("sum_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value2")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}

	// add a different scope metrics
	sm = rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 3")
	sm.Scope().Attributes().PutStr("Scope Attributes 3", "value3")
	sm.Scope().SetDroppedAttributesCount(20)
	sm.Scope().SetName("Scope name 3")
	sm.Scope().SetVersion("Scope version 3")
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("gauge_label_3", "3")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("sum_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value2")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}
	return metrics
}

func verifyGaugeMetric(t *testing.T, exporter *metricsExporter) {
	type gauge struct {
		ResourceAttributes          map[string]string   `ch:"ResourceAttributes"`
		ResourceSchemaURL           string              `ch:"ResourceSchemaUrl"`
		ScopeName                   string              `ch:"ScopeName"`
		ScopeVersion                string              `ch:"ScopeVersion"`
		ScopeAttributes             map[string]string   `ch:"ScopeAttributes"`
		ScopeDroppedAttrCount       uint32              `ch:"ScopeDroppedAttrCount"`
		ScopeSchemaURL              string              `ch:"ScopeSchemaUrl"`
		ServiceName                 string              `ch:"ServiceName"`
		MetricName                  string              `ch:"MetricName"`
		MetricDescription           string              `ch:"MetricDescription"`
		MetricUnit                  string              `ch:"MetricUnit"`
		Attributes                  map[string]string   `ch:"Attributes"`
		StartTimeUnix               time.Time           `ch:"StartTimeUnix"`
		TimeUnix                    time.Time           `ch:"TimeUnix"`
		Value                       float64             `ch:"Value"`
		Flags                       uint32              `ch:"Flags"`
		ExemplarsFilteredAttributes []map[string]string `ch:"Exemplars.FilteredAttributes"`
		ExemplarsTimeUnix           []time.Time         `ch:"Exemplars.TimeUnix"`
		ExemplarsValue              []float64           `ch:"Exemplars.Value"`
		ExemplarsSpanID             []string            `ch:"Exemplars.SpanId"`
		ExemplarsTraceID            []string            `ch:"Exemplars.TraceId"`
	}

	expectedGauge := gauge{
		ResourceAttributes: map[string]string{
			"service.name":          "demo 1",
			"Resource Attributes 1": "value1",
		},
		ResourceSchemaURL:     "Resource SchemaUrl 1",
		ScopeName:             "Scope name 1",
		ScopeVersion:          "Scope version 1",
		ScopeDroppedAttrCount: 10,
		ScopeSchemaURL:        "Scope SchemaUrl 1",
		ScopeAttributes: map[string]string{
			"Scope Attributes 1": "value1",
		},
		ServiceName:       "demo 1",
		MetricName:        "gauge metrics",
		MetricDescription: "This is a gauge metrics",
		MetricUnit:        "count",
		Attributes: map[string]string{
			"gauge_label_1": "1",
		},
		StartTimeUnix: telemetryTimestamp,
		TimeUnix:      telemetryTimestamp,
		Value:         0,
		Flags:         0,
		ExemplarsFilteredAttributes: []map[string]string{
			{
				"key":  "value",
				"key2": "value2",
			},
		},
		ExemplarsTimeUnix: []time.Time{telemetryTimestamp},
		ExemplarsTraceID:  []string{"01020300000000000000000000000000"},
		ExemplarsSpanID:   []string{"0102030000000000"},
		ExemplarsValue:    []float64{54},
	}

	row := exporter.db.QueryRow(context.Background(), "SELECT * FROM otel_int_test.otel_metrics_gauge")
	require.NoError(t, row.Err())

	var actualGauge gauge
	err := row.ScanStruct(&actualGauge)
	require.NoError(t, err)

	require.Equal(t, expectedGauge, actualGauge)
}

func verifySumMetric(t *testing.T, exporter *metricsExporter) {
	type sum struct {
		ResourceAttributes          map[string]string   `ch:"ResourceAttributes"`
		ResourceSchemaURL           string              `ch:"ResourceSchemaUrl"`
		ScopeName                   string              `ch:"ScopeName"`
		ScopeVersion                string              `ch:"ScopeVersion"`
		ScopeAttributes             map[string]string   `ch:"ScopeAttributes"`
		ScopeDroppedAttrCount       uint32              `ch:"ScopeDroppedAttrCount"`
		ScopeSchemaURL              string              `ch:"ScopeSchemaUrl"`
		ServiceName                 string              `ch:"ServiceName"`
		MetricName                  string              `ch:"MetricName"`
		MetricDescription           string              `ch:"MetricDescription"`
		MetricUnit                  string              `ch:"MetricUnit"`
		Attributes                  map[string]string   `ch:"Attributes"`
		StartTimeUnix               time.Time           `ch:"StartTimeUnix"`
		TimeUnix                    time.Time           `ch:"TimeUnix"`
		Value                       float64             `ch:"Value"`
		Flags                       uint32              `ch:"Flags"`
		ExemplarsFilteredAttributes []map[string]string `ch:"Exemplars.FilteredAttributes"`
		ExemplarsTimeUnix           []time.Time         `ch:"Exemplars.TimeUnix"`
		ExemplarsValue              []float64           `ch:"Exemplars.Value"`
		ExemplarsSpanID             []string            `ch:"Exemplars.SpanId"`
		ExemplarsTraceID            []string            `ch:"Exemplars.TraceId"`
		AggregationTemporality      int32               `ch:"AggregationTemporality"`
		IsMonotonic                 bool                `ch:"IsMonotonic"`
	}

	expectedSum := sum{
		ResourceAttributes: map[string]string{
			"service.name":          "demo 1",
			"Resource Attributes 1": "value1",
		},
		ResourceSchemaURL:     "Resource SchemaUrl 1",
		ScopeName:             "Scope name 1",
		ScopeVersion:          "Scope version 1",
		ScopeDroppedAttrCount: 10,
		ScopeSchemaURL:        "Scope SchemaUrl 1",
		ScopeAttributes: map[string]string{
			"Scope Attributes 1": "value1",
		},
		ServiceName:       "demo 1",
		MetricName:        "sum metrics",
		MetricDescription: "This is a sum metrics",
		MetricUnit:        "count",
		Attributes: map[string]string{
			"sum_label_1": "1",
		},
		StartTimeUnix: telemetryTimestamp,
		TimeUnix:      telemetryTimestamp,
		Value:         11.234,
		Flags:         0,
		ExemplarsFilteredAttributes: []map[string]string{
			{
				"key":  "value",
				"key2": "value2",
			},
		},
		ExemplarsTimeUnix: []time.Time{telemetryTimestamp},
		ExemplarsTraceID:  []string{"01020300000000000000000000000000"},
		ExemplarsSpanID:   []string{"0102030000000000"},
		ExemplarsValue:    []float64{54},
	}

	row := exporter.db.QueryRow(context.Background(), "SELECT * FROM otel_int_test.otel_metrics_sum")
	require.NoError(t, row.Err())

	var actualSum sum
	err := row.ScanStruct(&actualSum)
	require.NoError(t, err)

	require.Equal(t, expectedSum, actualSum)
}

func verifyHistogramMetric(t *testing.T, exporter *metricsExporter) {
	type histogram struct {
		ResourceAttributes          map[string]string   `ch:"ResourceAttributes"`
		ResourceSchemaURL           string              `ch:"ResourceSchemaUrl"`
		ScopeName                   string              `ch:"ScopeName"`
		ScopeVersion                string              `ch:"ScopeVersion"`
		ScopeAttributes             map[string]string   `ch:"ScopeAttributes"`
		ScopeDroppedAttrCount       uint32              `ch:"ScopeDroppedAttrCount"`
		ScopeSchemaURL              string              `ch:"ScopeSchemaUrl"`
		ServiceName                 string              `ch:"ServiceName"`
		MetricName                  string              `ch:"MetricName"`
		MetricDescription           string              `ch:"MetricDescription"`
		MetricUnit                  string              `ch:"MetricUnit"`
		Attributes                  map[string]string   `ch:"Attributes"`
		StartTimeUnix               time.Time           `ch:"StartTimeUnix"`
		TimeUnix                    time.Time           `ch:"TimeUnix"`
		Count                       uint64              `ch:"Count"`
		Sum                         float64             `ch:"Sum"`
		BucketCounts                []uint64            `ch:"BucketCounts"`
		ExplicitBounds              []float64           `ch:"ExplicitBounds"`
		ExemplarsFilteredAttributes []map[string]string `ch:"Exemplars.FilteredAttributes"`
		ExemplarsTimeUnix           []time.Time         `ch:"Exemplars.TimeUnix"`
		ExemplarsValue              []float64           `ch:"Exemplars.Value"`
		ExemplarsSpanID             []string            `ch:"Exemplars.SpanId"`
		ExemplarsTraceID            []string            `ch:"Exemplars.TraceId"`
		AggregationTemporality      int32               `ch:"AggregationTemporality"`
		Flags                       uint32              `ch:"Flags"`
		Min                         float64             `ch:"Min"`
		Max                         float64             `ch:"Max"`
	}

	expectedHistogram := histogram{
		ResourceAttributes: map[string]string{
			"service.name":          "demo 1",
			"Resource Attributes 1": "value1",
		},
		ResourceSchemaURL:     "Resource SchemaUrl 1",
		ScopeName:             "Scope name 1",
		ScopeVersion:          "Scope version 1",
		ScopeDroppedAttrCount: 10,
		ScopeSchemaURL:        "Scope SchemaUrl 1",
		ScopeAttributes: map[string]string{
			"Scope Attributes 1": "value1",
		},
		ServiceName:       "demo 1",
		MetricName:        "histogram metrics",
		MetricDescription: "This is a histogram metrics",
		MetricUnit:        "ms",
		Attributes: map[string]string{
			"key":  "value",
			"key2": "value",
		},
		StartTimeUnix:  telemetryTimestamp,
		TimeUnix:       telemetryTimestamp,
		Count:          1,
		Sum:            1,
		BucketCounts:   []uint64{0, 0, 0, 1, 0},
		ExplicitBounds: []float64{0, 0, 0, 0, 0},
		Flags:          0,
		Min:            0,
		Max:            1,
		ExemplarsFilteredAttributes: []map[string]string{
			{
				"key":  "value",
				"key2": "value2",
			},
		},
		ExemplarsTimeUnix: []time.Time{telemetryTimestamp},
		ExemplarsTraceID:  []string{"01020300000000000000000000000000"},
		ExemplarsSpanID:   []string{"0102030000000000"},
		ExemplarsValue:    []float64{55.22},
	}

	row := exporter.db.QueryRow(context.Background(), "SELECT * FROM otel_int_test.otel_metrics_histogram")
	require.NoError(t, row.Err())

	var actualHistogram histogram
	err := row.ScanStruct(&actualHistogram)
	require.NoError(t, err)

	require.Equal(t, expectedHistogram, actualHistogram)
}

func verifyExphistogramMetric(t *testing.T, exporter *metricsExporter) {
	type expHistogram struct {
		ResourceAttributes          map[string]string   `ch:"ResourceAttributes"`
		ResourceSchemaURL           string              `ch:"ResourceSchemaUrl"`
		ScopeName                   string              `ch:"ScopeName"`
		ScopeVersion                string              `ch:"ScopeVersion"`
		ScopeAttributes             map[string]string   `ch:"ScopeAttributes"`
		ScopeDroppedAttrCount       uint32              `ch:"ScopeDroppedAttrCount"`
		ScopeSchemaURL              string              `ch:"ScopeSchemaUrl"`
		ServiceName                 string              `ch:"ServiceName"`
		MetricName                  string              `ch:"MetricName"`
		MetricDescription           string              `ch:"MetricDescription"`
		MetricUnit                  string              `ch:"MetricUnit"`
		Attributes                  map[string]string   `ch:"Attributes"`
		StartTimeUnix               time.Time           `ch:"StartTimeUnix"`
		TimeUnix                    time.Time           `ch:"TimeUnix"`
		Count                       uint64              `ch:"Count"`
		Sum                         float64             `ch:"Sum"`
		Scale                       int32               `ch:"Scale"`
		ZeroCount                   uint64              `ch:"ZeroCount"`
		PositiveOffset              int32               `ch:"PositiveOffset"`
		PositiveBucketCounts        []uint64            `ch:"PositiveBucketCounts"`
		NegativeOffset              int32               `ch:"NegativeOffset"`
		NegativeBucketCounts        []uint64            `ch:"NegativeBucketCounts"`
		ExemplarsFilteredAttributes []map[string]string `ch:"Exemplars.FilteredAttributes"`
		ExemplarsTimeUnix           []time.Time         `ch:"Exemplars.TimeUnix"`
		ExemplarsValue              []float64           `ch:"Exemplars.Value"`
		ExemplarsSpanID             []string            `ch:"Exemplars.SpanId"`
		ExemplarsTraceID            []string            `ch:"Exemplars.TraceId"`
		AggregationTemporality      int32               `ch:"AggregationTemporality"`
		Flags                       uint32              `ch:"Flags"`
		Min                         float64             `ch:"Min"`
		Max                         float64             `ch:"Max"`
	}

	expectedExpHistogram := expHistogram{
		ResourceAttributes: map[string]string{
			"service.name":          "demo 1",
			"Resource Attributes 1": "value1",
		},
		ResourceSchemaURL:     "Resource SchemaUrl 1",
		ScopeName:             "Scope name 1",
		ScopeVersion:          "Scope version 1",
		ScopeDroppedAttrCount: 10,
		ScopeSchemaURL:        "Scope SchemaUrl 1",
		ScopeAttributes: map[string]string{
			"Scope Attributes 1": "value1",
		},
		ServiceName:       "demo 1",
		MetricName:        "exp histogram metrics",
		MetricDescription: "This is a exp histogram metrics",
		MetricUnit:        "ms",
		Attributes: map[string]string{
			"key":  "value",
			"key2": "value",
		},
		StartTimeUnix:        telemetryTimestamp,
		TimeUnix:             telemetryTimestamp,
		Count:                1,
		Sum:                  1,
		Scale:                0,
		ZeroCount:            0,
		PositiveOffset:       1,
		PositiveBucketCounts: []uint64{0, 0, 0, 1, 0},
		NegativeOffset:       1,
		NegativeBucketCounts: []uint64{0, 0, 0, 1, 0},
		Flags:                0,
		Min:                  0,
		Max:                  1,
		ExemplarsFilteredAttributes: []map[string]string{
			{
				"key":  "value",
				"key2": "value2",
			},
		},
		ExemplarsTimeUnix: []time.Time{telemetryTimestamp},
		ExemplarsTraceID:  []string{"01020300000000000000000000000000"},
		ExemplarsSpanID:   []string{"0102030000000000"},
		ExemplarsValue:    []float64{54},
	}

	row := exporter.db.QueryRow(context.Background(), "SELECT * FROM otel_int_test.otel_metrics_exponential_histogram")
	require.NoError(t, row.Err())

	var actualExpHistogram expHistogram
	err := row.ScanStruct(&actualExpHistogram)
	require.NoError(t, err)

	require.Equal(t, expectedExpHistogram, actualExpHistogram)
}

func verifySummaryMetric(t *testing.T, exporter *metricsExporter) {
	type summary struct {
		ResourceAttributes    map[string]string `ch:"ResourceAttributes"`
		ResourceSchemaURL     string            `ch:"ResourceSchemaUrl"`
		ScopeName             string            `ch:"ScopeName"`
		ScopeVersion          string            `ch:"ScopeVersion"`
		ScopeAttributes       map[string]string `ch:"ScopeAttributes"`
		ScopeDroppedAttrCount uint32            `ch:"ScopeDroppedAttrCount"`
		ScopeSchemaURL        string            `ch:"ScopeSchemaUrl"`
		ServiceName           string            `ch:"ServiceName"`
		MetricName            string            `ch:"MetricName"`
		MetricDescription     string            `ch:"MetricDescription"`
		MetricUnit            string            `ch:"MetricUnit"`
		Attributes            map[string]string `ch:"Attributes"`
		StartTimeUnix         time.Time         `ch:"StartTimeUnix"`
		TimeUnix              time.Time         `ch:"TimeUnix"`
		Count                 uint64            `ch:"Count"`
		Sum                   float64           `ch:"Sum"`
		Quantile              []float64         `ch:"ValueAtQuantiles.Quantile"`
		QuantilesValue        []float64         `ch:"ValueAtQuantiles.Value"`
		Flags                 uint32            `ch:"Flags"`
	}

	expectedSummary := summary{
		ResourceAttributes: map[string]string{
			"service.name":          "demo 1",
			"Resource Attributes 1": "value1",
		},
		ResourceSchemaURL:     "Resource SchemaUrl 1",
		ScopeName:             "Scope name 1",
		ScopeVersion:          "Scope version 1",
		ScopeDroppedAttrCount: 10,
		ScopeSchemaURL:        "Scope SchemaUrl 1",
		ScopeAttributes: map[string]string{
			"Scope Attributes 1": "value1",
		},
		ServiceName:       "demo 1",
		MetricName:        "summary metrics",
		MetricDescription: "This is a summary metrics",
		MetricUnit:        "ms",
		Attributes: map[string]string{
			"key":  "value",
			"key2": "value",
		},
		StartTimeUnix:  telemetryTimestamp,
		TimeUnix:       telemetryTimestamp,
		Count:          1,
		Sum:            1,
		Quantile:       []float64{1},
		QuantilesValue: []float64{1},
		Flags:          0,
	}

	row := exporter.db.QueryRow(context.Background(), "SELECT * FROM otel_int_test.otel_metrics_summary")
	require.NoError(t, row.Err())

	var actualSummary summary
	err := row.ScanStruct(&actualSummary)
	require.NoError(t, err)

	require.Equal(t, expectedSummary, actualSummary)
}
