// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func exampleIntMetric() (pmetric.Metric, pcommon.Map) {
	return buildExampleIntMetric(true)
}

func buildExampleIntMetric(fillData bool) (pmetric.Metric, pcommon.Map) {
	metric := pmetric.NewMetric()
	metric.SetName("test.metric.data")
	metric.SetUnit("bytes")
	metric.SetEmptySum()

	if fillData {
		dp := metric.Sum().DataPoints().AppendEmpty()
		dp.SetTimestamp(1605534165 * 1e9)
		dp.SetIntValue(14500)
	}

	attributes := pcommon.NewMap()
	attributes.PutStr("test", "test_value")
	attributes.PutStr("test2", "second_value")

	return metric, attributes
}

func exampleIntGaugeMetric() (pmetric.Metric, pcommon.Map) {
	return buildExampleIntGaugeMetric(true)
}

func buildExampleIntGaugeMetric(fillData bool) (pmetric.Metric, pcommon.Map) {
	attributes := pcommon.NewMap()
	metric := pmetric.NewMetric()

	metric.SetEmptyGauge()
	metric.SetName("gauge_metric_name")

	attributes.PutStr("foo", "bar")

	if fillData {
		dp := metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("remote_name", "156920")
		dp.Attributes().PutStr("url", "http://example_url")
		dp.SetIntValue(124)
		dp.SetTimestamp(1608124661.166 * 1e9)

		dp = metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("remote_name", "156955")
		dp.Attributes().PutStr("url", "http://another_url")
		dp.SetIntValue(245)
		dp.SetTimestamp(1608124662.166 * 1e9)
	}

	return metric, attributes
}

func exampleDoubleGaugeMetric() (pmetric.Metric, pcommon.Map) {
	return buildExampleDoubleGaugeMetric(true)
}

func buildExampleDoubleGaugeMetric(fillData bool) (pmetric.Metric, pcommon.Map) {
	attributes := pcommon.NewMap()
	metric := pmetric.NewMetric()

	metric.SetEmptyGauge()
	metric.SetName("gauge_metric_name_double_test")

	attributes.PutStr("foo", "bar")

	if fillData {
		dp := metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("local_name", "156720")
		dp.Attributes().PutStr("endpoint", "http://example_url")
		dp.SetDoubleValue(33.4)
		dp.SetTimestamp(1608124661.169 * 1e9)

		dp = metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("local_name", "156155")
		dp.Attributes().PutStr("endpoint", "http://another_url")
		dp.SetDoubleValue(56.8)
		dp.SetTimestamp(1608124662.186 * 1e9)
	}

	return metric, attributes
}

func exampleIntSumMetric() (pmetric.Metric, pcommon.Map) {
	return buildExampleIntSumMetric(true)
}

func buildExampleIntSumMetric(fillData bool) (pmetric.Metric, pcommon.Map) {
	attributes := pcommon.NewMap()
	metric := pmetric.NewMetric()

	metric.SetEmptySum()
	metric.SetName("sum_metric_int_test")

	attributes.PutStr("foo", "bar")

	if fillData {
		dp := metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("name", "156720")
		dp.Attributes().PutStr("address", "http://example_url")
		dp.SetIntValue(45)
		dp.SetTimestamp(1608124444.169 * 1e9)

		dp = metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("name", "156155")
		dp.Attributes().PutStr("address", "http://another_url")
		dp.SetIntValue(1238)
		dp.SetTimestamp(1608124699.186 * 1e9)
	}

	return metric, attributes
}

func exampleDoubleSumMetric() (pmetric.Metric, pcommon.Map) {
	return buildExampleDoubleSumMetric(true)
}

func buildExampleDoubleSumMetric(fillData bool) (pmetric.Metric, pcommon.Map) {
	attributes := pcommon.NewMap()
	metric := pmetric.NewMetric()

	metric.SetEmptySum()
	metric.SetName("sum_metric_double_test")

	attributes.PutStr("foo", "bar")

	if fillData {
		dp := metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("pod_name", "lorem")
		dp.Attributes().PutStr("namespace", "default")
		dp.SetDoubleValue(45.6)
		dp.SetTimestamp(1618124444.169 * 1e9)

		dp = metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("pod_name", "opsum")
		dp.Attributes().PutStr("namespace", "kube-config")
		dp.SetDoubleValue(1238.1)
		dp.SetTimestamp(1608424699.186 * 1e9)
	}

	return metric, attributes
}

func exampleSummaryMetric() (pmetric.Metric, pcommon.Map) {
	return buildExampleSummaryMetric(true)
}

func buildExampleSummaryMetric(fillData bool) (pmetric.Metric, pcommon.Map) {
	attributes := pcommon.NewMap()
	metric := pmetric.NewMetric()

	metric.SetEmptySummary()
	metric.SetName("summary_metric_double_test")

	attributes.PutStr("foo", "bar")

	if fillData {
		dp := metric.Summary().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("pod_name", "dolor")
		dp.Attributes().PutStr("namespace", "sumologic")
		dp.SetSum(45.6)
		dp.SetCount(3)
		dp.SetTimestamp(1618124444.169 * 1e9)

		quantile := dp.QuantileValues().AppendEmpty()
		quantile.SetQuantile(0.6)
		quantile.SetValue(0.7)

		quantile = dp.QuantileValues().AppendEmpty()
		quantile.SetQuantile(2.6)
		quantile.SetValue(4)

		dp = metric.Summary().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("pod_name", "sit")
		dp.Attributes().PutStr("namespace", "main")
		dp.SetSum(1238.1)
		dp.SetCount(7)
		dp.SetTimestamp(1608424699.186 * 1e9)
	}

	return metric, attributes
}

func exampleHistogramMetric() (pmetric.Metric, pcommon.Map) {
	return buildExampleHistogramMetric(true)
}

func buildExampleHistogramMetric(fillData bool) (pmetric.Metric, pcommon.Map) {
	attributes := pcommon.NewMap()
	metric := pmetric.NewMetric()

	metric.SetEmptyHistogram()
	metric.SetName("histogram_metric_double_test")

	attributes.PutStr("bar", "foo")

	if fillData {
		dp := metric.Histogram().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("container", "dolor")
		dp.Attributes().PutStr("branch", "sumologic")
		si := pcommon.NewUInt64Slice()
		si.FromRaw([]uint64{0, 12, 7, 5, 8, 13})
		si.CopyTo(dp.BucketCounts())

		sf := pcommon.NewFloat64Slice()
		sf.FromRaw([]float64{0.1, 0.2, 0.5, 0.8, 1})
		sf.CopyTo(dp.ExplicitBounds())

		dp.SetTimestamp(1618124444.169 * 1e9)
		dp.SetSum(45.6)
		dp.SetCount(7)

		dp = metric.Histogram().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("container", "sit")
		dp.Attributes().PutStr("branch", "main")

		si = pcommon.NewUInt64Slice()
		si.FromRaw([]uint64{0, 10, 1, 1, 4, 6})
		si.CopyTo(dp.BucketCounts())

		sf = pcommon.NewFloat64Slice()
		sf.FromRaw([]float64{0.1, 0.2, 0.5, 0.8, 1})
		sf.CopyTo(dp.ExplicitBounds())

		dp.SetTimestamp(1608424699.186 * 1e9)
		dp.SetSum(54.1)
		dp.SetCount(98)
	} else {
		dp := metric.Histogram().DataPoints().AppendEmpty()
		dp.SetCount(0)
	}

	return metric, attributes
}

func metricPairToMetrics(mp ...metricPair) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(len(mp))
	for _, record := range mp {
		rms := metrics.ResourceMetrics().AppendEmpty()
		record.attributes.CopyTo(rms.Resource().Attributes())
		// TODO: Change metricPair to have an init metric func.
		record.metric.CopyTo(rms.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	}

	metrics.MarkReadOnly()
	return metrics
}

func metricAndAttrsToPdataMetrics(attributes pcommon.Map, ms ...pmetric.Metric) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(len(ms))

	rms := metrics.ResourceMetrics().AppendEmpty()
	attributes.CopyTo(rms.Resource().Attributes())

	metricsSlice := rms.ScopeMetrics().AppendEmpty().Metrics()

	for _, record := range ms {
		record.CopyTo(metricsSlice.AppendEmpty())
	}

	return metrics
}

func metricAndAttributesToPdataMetrics(metric pmetric.Metric, attributes pcommon.Map) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(attributes.Len())
	rms := metrics.ResourceMetrics().AppendEmpty()
	attributes.CopyTo(rms.Resource().Attributes())
	metric.CopyTo(rms.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())

	return metrics
}

func fieldsFromMap(s map[string]string) fields {
	attrMap := pcommon.NewMap()
	for k, v := range s {
		attrMap.PutStr(k, v)
	}
	return newFields(attrMap)
}

func exampleTrace() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("hostname", "testHost")
	rs.Resource().Attributes().PutStr("_sourceHost", "source_host")
	rs.Resource().Attributes().PutStr("_sourceName", "source_name")
	rs.Resource().Attributes().PutStr("_sourceCategory", "source_category")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{0x5B, 0x8E, 0xFF, 0xF7, 0x98, 0x3, 0x81, 0x3, 0xD2, 0x69, 0xB6, 0x33, 0x81, 0x3F, 0xC6, 0xC}))
	span.SetSpanID(pcommon.SpanID([8]byte{0xEE, 0xE1, 0x9B, 0x7E, 0xC3, 0xC1, 0xB1, 0x73}))
	span.SetName("testSpan")
	span.SetStartTimestamp(1544712660000000000)
	span.SetEndTimestamp(1544712661000000000)
	span.Attributes().PutInt("attr1", 55)
	td.MarkReadOnly()
	return td
}
