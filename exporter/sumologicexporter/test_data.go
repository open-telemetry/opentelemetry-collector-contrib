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

package sumologicexporter

import (
	"go.opentelemetry.io/collector/model/pdata"
)

func exampleIntMetric() metricPair {
	metric := pdata.NewMetric()
	metric.SetName("test.metric.data")
	metric.SetUnit("bytes")
	metric.SetDataType(pdata.MetricDataTypeIntSum)
	dp := metric.IntSum().DataPoints().AppendEmpty()
	dp.SetTimestamp(1605534165 * 1e9)
	dp.SetValue(14500)

	attributes := pdata.NewAttributeMap()
	attributes.InsertString("test", "test_value")
	attributes.InsertString("test2", "second_value")

	return metricPair{
		metric:     metric,
		attributes: attributes,
	}
}

func exampleIntGaugeMetric() metricPair {
	metric := metricPair{
		attributes: pdata.NewAttributeMap(),
		metric:     pdata.NewMetric(),
	}

	metric.metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.metric.SetName("gauge_metric_name")

	metric.attributes.InsertString("foo", "bar")

	dp := metric.metric.IntGauge().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("remote_name", "156920")
	dp.LabelsMap().Insert("url", "http://example_url")
	dp.SetValue(124)
	dp.SetTimestamp(1608124661.166 * 1e9)

	dp = metric.metric.IntGauge().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("remote_name", "156955")
	dp.LabelsMap().Insert("url", "http://another_url")
	dp.SetValue(245)
	dp.SetTimestamp(1608124662.166 * 1e9)

	return metric
}

func exampleDoubleGaugeMetric() metricPair {
	metric := metricPair{
		attributes: pdata.NewAttributeMap(),
		metric:     pdata.NewMetric(),
	}

	metric.metric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	metric.metric.SetName("gauge_metric_name_double_test")

	metric.attributes.InsertString("foo", "bar")

	dp := metric.metric.DoubleGauge().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("local_name", "156720")
	dp.LabelsMap().Insert("endpoint", "http://example_url")
	dp.SetValue(33.4)
	dp.SetTimestamp(1608124661.169 * 1e9)

	dp = metric.metric.DoubleGauge().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("local_name", "156155")
	dp.LabelsMap().Insert("endpoint", "http://another_url")
	dp.SetValue(56.8)
	dp.SetTimestamp(1608124662.186 * 1e9)

	return metric
}

func exampleIntSumMetric() metricPair {
	metric := metricPair{
		attributes: pdata.NewAttributeMap(),
		metric:     pdata.NewMetric(),
	}

	metric.metric.SetDataType(pdata.MetricDataTypeIntSum)
	metric.metric.SetName("sum_metric_int_test")

	metric.attributes.InsertString("foo", "bar")

	dp := metric.metric.IntSum().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("name", "156720")
	dp.LabelsMap().Insert("address", "http://example_url")
	dp.SetValue(45)
	dp.SetTimestamp(1608124444.169 * 1e9)

	dp = metric.metric.IntSum().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("name", "156155")
	dp.LabelsMap().Insert("address", "http://another_url")
	dp.SetValue(1238)
	dp.SetTimestamp(1608124699.186 * 1e9)

	return metric
}

func exampleDoubleSumMetric() metricPair {
	metric := metricPair{
		attributes: pdata.NewAttributeMap(),
		metric:     pdata.NewMetric(),
	}

	metric.metric.SetDataType(pdata.MetricDataTypeDoubleSum)
	metric.metric.SetName("sum_metric_double_test")

	metric.attributes.InsertString("foo", "bar")

	dp := metric.metric.DoubleSum().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("pod_name", "lorem")
	dp.LabelsMap().Insert("namespace", "default")
	dp.SetValue(45.6)
	dp.SetTimestamp(1618124444.169 * 1e9)

	dp = metric.metric.DoubleSum().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("pod_name", "opsum")
	dp.LabelsMap().Insert("namespace", "kube-config")
	dp.SetValue(1238.1)
	dp.SetTimestamp(1608424699.186 * 1e9)

	return metric
}

func exampleSummaryMetric() metricPair {
	metric := metricPair{
		attributes: pdata.NewAttributeMap(),
		metric:     pdata.NewMetric(),
	}

	metric.metric.SetDataType(pdata.MetricDataTypeSummary)
	metric.metric.SetName("summary_metric_double_test")

	metric.attributes.InsertString("foo", "bar")

	dp := metric.metric.Summary().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("pod_name", "dolor")
	dp.LabelsMap().Insert("namespace", "sumologic")
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
	dp.LabelsMap().Insert("pod_name", "sit")
	dp.LabelsMap().Insert("namespace", "main")
	dp.SetSum(1238.1)
	dp.SetCount(7)
	dp.SetTimestamp(1608424699.186 * 1e9)

	return metric
}

func exampleIntHistogramMetric() metricPair {
	metric := metricPair{
		attributes: pdata.NewAttributeMap(),
		metric:     pdata.NewMetric(),
	}

	metric.metric.SetDataType(pdata.MetricDataTypeIntHistogram)
	metric.metric.SetName("histogram_metric_int_test")

	metric.attributes.InsertString("foo", "bar")

	dp := metric.metric.IntHistogram().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("pod_name", "dolor")
	dp.LabelsMap().Insert("namespace", "sumologic")
	dp.SetBucketCounts([]uint64{0, 12, 7, 5, 8, 13})
	dp.SetExplicitBounds([]float64{0.1, 0.2, 0.5, 0.8, 1})
	dp.SetTimestamp(1618124444.169 * 1e9)
	dp.SetSum(45)
	dp.SetCount(3)

	dp = metric.metric.IntHistogram().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("pod_name", "sit")
	dp.LabelsMap().Insert("namespace", "main")
	dp.SetBucketCounts([]uint64{0, 10, 1, 1, 4, 6})
	dp.SetExplicitBounds([]float64{0.1, 0.2, 0.5, 0.8, 1})
	dp.SetTimestamp(1608424699.186 * 1e9)
	dp.SetSum(54)
	dp.SetCount(5)

	return metric
}

func exampleHistogramMetric() metricPair {
	metric := metricPair{
		attributes: pdata.NewAttributeMap(),
		metric:     pdata.NewMetric(),
	}

	metric.metric.SetDataType(pdata.MetricDataTypeHistogram)
	metric.metric.SetName("histogram_metric_double_test")

	metric.attributes.InsertString("bar", "foo")

	dp := metric.metric.Histogram().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("container", "dolor")
	dp.LabelsMap().Insert("branch", "sumologic")
	dp.SetBucketCounts([]uint64{0, 12, 7, 5, 8, 13})
	dp.SetExplicitBounds([]float64{0.1, 0.2, 0.5, 0.8, 1})
	dp.SetTimestamp(1618124444.169 * 1e9)
	dp.SetSum(45.6)
	dp.SetCount(7)

	dp = metric.metric.Histogram().DataPoints().AppendEmpty()
	dp.LabelsMap().Insert("container", "sit")
	dp.LabelsMap().Insert("branch", "main")
	dp.SetBucketCounts([]uint64{0, 10, 1, 1, 4, 6})
	dp.SetExplicitBounds([]float64{0.1, 0.2, 0.5, 0.8, 1})
	dp.SetTimestamp(1608424699.186 * 1e9)
	dp.SetSum(54.1)
	dp.SetCount(98)

	return metric
}

func metricPairToMetrics(mp []metricPair) pdata.Metrics {
	metrics := pdata.NewMetrics()
	metrics.ResourceMetrics().Resize(len(mp))
	for num, record := range mp {
		record.attributes.CopyTo(metrics.ResourceMetrics().At(num).Resource().Attributes())
		// TODO: Change metricPair to have an init metric func.
		record.metric.CopyTo(metrics.ResourceMetrics().At(num).InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty())
	}

	return metrics
}

func fieldsFromMap(s map[string]string) fields {
	attrMap := pdata.NewAttributeMap()
	for k, v := range s {
		attrMap.InsertString(k, v)
	}
	return newFields(attrMap)
}
