// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestStartMetrics(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.MySQLEndpoint = "127.0.0.1:9030"
	config.Username = "admin"
	config.Password = "admin"
	config.Database = "otel2"
	config.Table.Metrics = "metrics"
	config.HistoryDays = 3

	exporter, err := newMetricsExporter(nil, config)
	ctx := context.Background()
	if err != nil {
		t.Error(err)
		return
	}
	defer exporter.shutdown(ctx)

	err = exporter.start(ctx, nil)
	if err != nil {
		t.Error(err)
		return
	}
}

func TestPushMetricData(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "http://127.0.0.1:8030"
	config.MySQLEndpoint = "127.0.0.1:9030"
	config.Username = "admin"
	config.Password = "admin"
	config.Database = "otel2"
	config.Table.Metrics = "metrics"
	config.HistoryDays = 3
	config.CreateHistoryDays = 1
	config.TimeZone = "America/New_York"

	err := config.Validate()
	if err != nil {
		t.Error(err)
		return
	}

	exporter, err := newMetricsExporter(nil, config)
	ctx := context.Background()
	if err != nil {
		t.Error(err)
		return
	}
	defer exporter.shutdown(ctx)

	err = exporter.start(ctx, nil)
	if err != nil {
		t.Error(err)
		return
	}

	typeSet := map[pmetric.MetricType]any{
		pmetric.MetricTypeGauge:                nil,
		pmetric.MetricTypeSum:                  nil,
		pmetric.MetricTypeHistogram:            nil,
		pmetric.MetricTypeExponentialHistogram: nil,
		pmetric.MetricTypeSummary:              nil,
	}

	err = exporter.pushMetricData(ctx, simpleMetrics(10, typeSet))
	if err != nil {
		t.Error(err)
		return
	}
}

func simpleMetrics(count int, typeSet map[pmetric.MetricType]any) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "demo 1")
	rm.Resource().Attributes().PutStr("Resource Attributes 1", "value1")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 1")
	sm.Scope().Attributes().PutStr("Scope Attributes 1", "value1")
	sm.Scope().SetName("Scope name 1")
	sm.Scope().SetVersion("Scope version 1")
	timestamp := time.Now()
	for i := 0; i < count; i++ {
		// gauge
		if _, ok := typeSet[pmetric.MetricTypeGauge]; ok {
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
		}

		// sum
		if _, ok := typeSet[pmetric.MetricTypeSum]; ok {
			m := sm.Metrics().AppendEmpty()
			m.SetName("sum metrics")
			m.SetUnit("count")
			m.SetDescription("This is a sum metrics")
			dp := m.SetEmptySum().DataPoints().AppendEmpty()
			dp.SetDoubleValue(11.234)
			dp.SetFlags(pmetric.DefaultDataPointFlags)
			dp.Attributes().PutStr("sum_label_1", "1")
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
			exemplars := dp.Exemplars().AppendEmpty()
			exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
			exemplars.SetIntValue(54)
			exemplars.FilteredAttributes().PutStr("key", "value")
			exemplars.FilteredAttributes().PutStr("key2", "value2")
			exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
			exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})
		}

		// histogram
		if _, ok := typeSet[pmetric.MetricTypeHistogram]; ok {
			m := sm.Metrics().AppendEmpty()
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
			exemplars := dpHisto.Exemplars().AppendEmpty()
			exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
			exemplars.SetDoubleValue(55.22)
			exemplars.FilteredAttributes().PutStr("key", "value")
			exemplars.FilteredAttributes().PutStr("key2", "value2")
			exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
			exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})
		}

		// exp histogram
		if _, ok := typeSet[pmetric.MetricTypeExponentialHistogram]; ok {
			m := sm.Metrics().AppendEmpty()
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

			exemplars := dpExpHisto.Exemplars().AppendEmpty()
			exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
			exemplars.SetIntValue(54)
			exemplars.FilteredAttributes().PutStr("key", "value")
			exemplars.FilteredAttributes().PutStr("key2", "value2")
			exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
			exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})
		}

		// summary
		if _, ok := typeSet[pmetric.MetricTypeSummary]; ok {
			m := sm.Metrics().AppendEmpty()
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
	}

	rm = metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "demo 2")
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
		if _, ok := typeSet[pmetric.MetricTypeGauge]; ok {
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
		}

		// sum
		if _, ok := typeSet[pmetric.MetricTypeSum]; ok {
			m := sm.Metrics().AppendEmpty()
			m.SetName("sum metrics")
			m.SetUnit("count")
			m.SetDescription("This is a sum metrics")
			dp := m.SetEmptySum().DataPoints().AppendEmpty()
			dp.SetIntValue(int64(i))
			dp.Attributes().PutStr("sum_label_2", "2")
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			exemplars := dp.Exemplars().AppendEmpty()
			exemplars.SetIntValue(54)
			exemplars.FilteredAttributes().PutStr("key", "value")
			exemplars.FilteredAttributes().PutStr("key2", "value2")
			exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
			exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		}

		// histogram
		if _, ok := typeSet[pmetric.MetricTypeHistogram]; ok {
			m := sm.Metrics().AppendEmpty()
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
			exemplars := dpHisto.Exemplars().AppendEmpty()
			exemplars.SetIntValue(54)
			exemplars.FilteredAttributes().PutStr("key", "value")
			exemplars.FilteredAttributes().PutStr("key2", "value2")
			exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
			exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		}

		// exp histogram
		if _, ok := typeSet[pmetric.MetricTypeExponentialHistogram]; ok {
			m := sm.Metrics().AppendEmpty()
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

			exemplars := dpExpHisto.Exemplars().AppendEmpty()
			exemplars.SetIntValue(54)
			exemplars.FilteredAttributes().PutStr("key", "value")
			exemplars.FilteredAttributes().PutStr("key2", "value2")
			exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
			exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})
		}

		// summary
		if _, ok := typeSet[pmetric.MetricTypeSummary]; ok {
			m := sm.Metrics().AppendEmpty()
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
		if _, ok := typeSet[pmetric.MetricTypeGauge]; ok {
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
		}

		// sum
		if _, ok := typeSet[pmetric.MetricTypeSum]; ok {
			m := sm.Metrics().AppendEmpty()
			m.SetName("sum metrics")
			m.SetUnit("count")
			m.SetDescription("This is a sum metrics")
			dp := m.SetEmptySum().DataPoints().AppendEmpty()
			dp.SetIntValue(int64(i))
			dp.Attributes().PutStr("sum_label_2", "2")
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			exemplars := dp.Exemplars().AppendEmpty()
			exemplars.SetIntValue(54)
			exemplars.FilteredAttributes().PutStr("key", "value")
			exemplars.FilteredAttributes().PutStr("key2", "value2")
			exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
			exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})
		}

		// histogram
		if _, ok := typeSet[pmetric.MetricTypeHistogram]; ok {
			m := sm.Metrics().AppendEmpty()
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
			exemplars := dpHisto.Exemplars().AppendEmpty()
			exemplars.SetIntValue(54)
			exemplars.FilteredAttributes().PutStr("key", "value")
			exemplars.FilteredAttributes().PutStr("key2", "value2")
			exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
			exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		}

		// exp histogram
		if _, ok := typeSet[pmetric.MetricTypeExponentialHistogram]; ok {
			m := sm.Metrics().AppendEmpty()
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

			exemplars := dpExpHisto.Exemplars().AppendEmpty()
			exemplars.SetIntValue(54)
			exemplars.FilteredAttributes().PutStr("key", "value")
			exemplars.FilteredAttributes().PutStr("key2", "value2")
			exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
			exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		}

		// summary
		if _, ok := typeSet[pmetric.MetricTypeSummary]; ok {
			m := sm.Metrics().AppendEmpty()
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
	}
	return metrics
}
