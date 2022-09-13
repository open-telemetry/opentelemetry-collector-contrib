// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		dp.SetIntVal(14500)
	}

	attributes := pcommon.NewMap()
	attributes.UpsertString("test", "test_value")
	attributes.UpsertString("test2", "second_value")

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

	metric.attributes.UpsertString("foo", "bar")

	if fillData {
		dp := metric.metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("remote_name", "156920")
		dp.Attributes().UpsertString("url", "http://example_url")
		dp.SetIntVal(124)
		dp.SetTimestamp(1608124661.166 * 1e9)

		dp = metric.metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("remote_name", "156955")
		dp.Attributes().UpsertString("url", "http://another_url")
		dp.SetIntVal(245)
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

	metric.attributes.UpsertString("foo", "bar")

	if fillData {
		dp := metric.metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("local_name", "156720")
		dp.Attributes().UpsertString("endpoint", "http://example_url")
		dp.SetDoubleVal(33.4)
		dp.SetTimestamp(1608124661.169 * 1e9)

		dp = metric.metric.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("local_name", "156155")
		dp.Attributes().UpsertString("endpoint", "http://another_url")
		dp.SetDoubleVal(56.8)
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

	metric.attributes.UpsertString("foo", "bar")

	if fillData {
		dp := metric.metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("name", "156720")
		dp.Attributes().UpsertString("address", "http://example_url")
		dp.SetIntVal(45)
		dp.SetTimestamp(1608124444.169 * 1e9)

		dp = metric.metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("name", "156155")
		dp.Attributes().UpsertString("address", "http://another_url")
		dp.SetIntVal(1238)
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

	metric.attributes.UpsertString("foo", "bar")

	if fillData {
		dp := metric.metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("pod_name", "lorem")
		dp.Attributes().UpsertString("namespace", "default")
		dp.SetDoubleVal(45.6)
		dp.SetTimestamp(1618124444.169 * 1e9)

		dp = metric.metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("pod_name", "opsum")
		dp.Attributes().UpsertString("namespace", "kube-config")
		dp.SetDoubleVal(1238.1)
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

	metric.attributes.UpsertString("foo", "bar")

	if fillData {
		dp := metric.metric.Summary().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("pod_name", "dolor")
		dp.Attributes().UpsertString("namespace", "sumologic")
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
		dp.Attributes().UpsertString("pod_name", "sit")
		dp.Attributes().UpsertString("namespace", "main")
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

	metric.attributes.UpsertString("bar", "foo")

	if fillData {
		dp := metric.metric.Histogram().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("container", "dolor")
		dp.Attributes().UpsertString("branch", "sumologic")
		dp.BucketCounts().FromRaw([]uint64{0, 12, 7, 5, 8, 13})
		dp.ExplicitBounds().FromRaw([]float64{0.1, 0.2, 0.5, 0.8, 1})
		dp.SetTimestamp(1618124444.169 * 1e9)
		dp.SetSum(45.6)
		dp.SetCount(7)

		dp = metric.metric.Histogram().DataPoints().AppendEmpty()
		dp.Attributes().UpsertString("container", "sit")
		dp.Attributes().UpsertString("branch", "main")
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
		attrMap.UpsertString(k, v)
	}
	return newFields(attrMap)
}
