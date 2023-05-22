// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func exampleIntMetric() metricPair {
	return buildExampleIntMetric(true)
}

func buildExampleIntMetric(fillData bool) metricPair {
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

	return metricPair{
		metric:     metric,
		attributes: attributes,
	}
}

func exampleIntGaugeMetric() metricPair {
	return buildExampleIntGaugeMetric(true)
}

func buildExampleIntGaugeMetric(fillData bool) metricPair {
	metric := metricPair{
		attributes: pcommon.NewMap(),
		metric:     pmetric.NewMetric(),
	}

	metric.metric.SetName("gauge_metric_name")
	metric.metric.SetEmptyGauge()

	metric.attributes.PutStr("foo", "bar")

	if fillData {
		dp := metric.metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("remote_name", "156920")
		dp.Attributes().PutStr("url", "http://example_url")
		dp.SetIntValue(124)
		dp.SetTimestamp(1608124661.166 * 1e9)

		dp = metric.metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("remote_name", "156955")
		dp.Attributes().PutStr("url", "http://another_url")
		dp.SetIntValue(245)
		dp.SetTimestamp(1608124662.166 * 1e9)
	}

	return metric
}

func exampleDoubleGaugeMetric() metricPair {
	return buildExampleDoubleGaugeMetric(true)
}

func buildExampleDoubleGaugeMetric(fillData bool) metricPair {
	metric := metricPair{
		attributes: pcommon.NewMap(),
		metric:     pmetric.NewMetric(),
	}

	metric.metric.SetEmptyGauge()
	metric.metric.SetName("gauge_metric_name_double_test")

	metric.attributes.PutStr("foo", "bar")

	if fillData {
		dp := metric.metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("local_name", "156720")
		dp.Attributes().PutStr("endpoint", "http://example_url")
		dp.SetDoubleValue(33.4)
		dp.SetTimestamp(1608124661.169 * 1e9)

		dp = metric.metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("local_name", "156155")
		dp.Attributes().PutStr("endpoint", "http://another_url")
		dp.SetDoubleValue(56.8)
		dp.SetTimestamp(1608124662.186 * 1e9)
	}

	return metric
}

func exampleIntSumMetric() metricPair {
	return buildExampleIntSumMetric(true)
}

func buildExampleIntSumMetric(fillData bool) metricPair {
	metric := metricPair{
		attributes: pcommon.NewMap(),
		metric:     pmetric.NewMetric(),
	}

	metric.metric.SetEmptySum()
	metric.metric.SetName("sum_metric_int_test")

	metric.attributes.PutStr("foo", "bar")

	if fillData {
		dp := metric.metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("name", "156720")
		dp.Attributes().PutStr("address", "http://example_url")
		dp.SetIntValue(45)
		dp.SetTimestamp(1608124444.169 * 1e9)

		dp = metric.metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("name", "156155")
		dp.Attributes().PutStr("address", "http://another_url")
		dp.SetIntValue(1238)
		dp.SetTimestamp(1608124699.186 * 1e9)
	}

	return metric
}

func exampleDoubleSumMetric() metricPair {
	return buildExampleDoubleSumMetric(true)
}

func buildExampleDoubleSumMetric(fillData bool) metricPair {
	metric := metricPair{
		attributes: pcommon.NewMap(),
		metric:     pmetric.NewMetric(),
	}

	metric.metric.SetEmptySum()
	metric.metric.SetName("sum_metric_double_test")

	metric.attributes.PutStr("foo", "bar")

	if fillData {
		dp := metric.metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("pod_name", "lorem")
		dp.Attributes().PutStr("namespace", "default")
		dp.SetDoubleValue(45.6)
		dp.SetTimestamp(1618124444.169 * 1e9)

		dp = metric.metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("pod_name", "opsum")
		dp.Attributes().PutStr("namespace", "kube-config")
		dp.SetDoubleValue(1238.1)
		dp.SetTimestamp(1608424699.186 * 1e9)
	}

	return metric
}

func exampleSummaryMetric() metricPair {
	return buildExampleSummaryMetric(true)
}

func buildExampleSummaryMetric(fillData bool) metricPair {
	metric := metricPair{
		attributes: pcommon.NewMap(),
		metric:     pmetric.NewMetric(),
	}

	metric.metric.SetEmptySummary()
	metric.metric.SetName("summary_metric_double_test")

	metric.attributes.PutStr("foo", "bar")

	if fillData {
		dp := metric.metric.Summary().DataPoints().AppendEmpty()
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

		dp = metric.metric.Summary().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("pod_name", "sit")
		dp.Attributes().PutStr("namespace", "main")
		dp.SetSum(1238.1)
		dp.SetCount(7)
		dp.SetTimestamp(1608424699.186 * 1e9)
	}

	return metric
}

func exampleHistogramMetric() metricPair {
	return buildExampleHistogramMetric(true)
}

func buildExampleHistogramMetric(fillData bool) metricPair {
	metric := metricPair{
		attributes: pcommon.NewMap(),
		metric:     pmetric.NewMetric(),
	}

	metric.metric.SetEmptyHistogram()
	metric.metric.SetName("histogram_metric_double_test")

	metric.attributes.PutStr("bar", "foo")

	if fillData {
		dp := metric.metric.Histogram().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("container", "dolor")
		dp.Attributes().PutStr("branch", "sumologic")
		dp.BucketCounts().FromRaw([]uint64{0, 12, 7, 5, 8, 13})
		dp.ExplicitBounds().FromRaw([]float64{0.1, 0.2, 0.5, 0.8, 1})
		dp.SetTimestamp(1618124444.169 * 1e9)
		dp.SetSum(45.6)
		dp.SetCount(7)

		dp = metric.metric.Histogram().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("container", "sit")
		dp.Attributes().PutStr("branch", "main")
		dp.BucketCounts().FromRaw([]uint64{0, 10, 1, 1, 4, 6})
		dp.ExplicitBounds().FromRaw([]float64{0.1, 0.2, 0.5, 0.8, 1})
		dp.SetTimestamp(1608424699.186 * 1e9)
		dp.SetSum(54.1)
		dp.SetCount(98)
	} else {
		dp := metric.metric.Histogram().DataPoints().AppendEmpty()
		dp.SetCount(0)
	}

	return metric
}

func metricPairToMetrics(mp []metricPair) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(len(mp))
	for num, record := range mp {
		record.attributes.CopyTo(metrics.ResourceMetrics().AppendEmpty().Resource().Attributes())
		// TODO: Change metricPair to have an init metric func.
		record.metric.CopyTo(metrics.ResourceMetrics().At(num).ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	}

	return metrics
}

func fieldsFromMap(s map[string]string) fields {
	attrMap := pcommon.NewMap()
	for k, v := range s {
		attrMap.PutStr(k, v)
	}
	return newFields(attrMap)
}
