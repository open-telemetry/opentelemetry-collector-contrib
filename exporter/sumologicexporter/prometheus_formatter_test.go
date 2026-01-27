// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestSanitizeKey(t *testing.T) {
	f := newPrometheusFormatter()

	key := "&^*123-abc-ABC!./?_:\n\r"
	expected := "___123-abc-ABC_./__:__"
	assert.EqualValues(t, expected, f.sanitizeKeyBytes([]byte(key)))
}

func TestSanitizeValue(t *testing.T) {
	f := newPrometheusFormatter()

	// `\`, `"` and `\n` should be escaped, everything else should be left as-is
	// see: https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#line-format
	value := `&^*123-abc-ABC!?./"\` + "\n\r"
	expected := `&^*123-abc-ABC!?./\"\\\n` + "\r"
	assert.Equal(t, expected, f.sanitizeValue(value))
}

func TestTags2StringNoLabels(t *testing.T) {
	f := newPrometheusFormatter()

	_, attributes := exampleIntMetric()
	attributes.Clear()
	assert.Equal(t, prometheusTags(""), f.tags2String(attributes, pcommon.NewMap()))
}

func TestTags2String(t *testing.T) {
	f := newPrometheusFormatter()

	_, attributes := exampleIntMetric()
	attributes.PutInt("int", 200)

	labels := pcommon.NewMap()
	labels.PutInt("l_int", 200)
	labels.PutStr("l_str", "two")

	assert.Equal(
		t,
		prometheusTags(`{test="test_value",test2="second_value",int="200",l_int="200",l_str="two"}`),
		f.tags2String(attributes, labels),
	)
}

func TestTags2StringNoAttributes(t *testing.T) {
	f := newPrometheusFormatter()

	_, attributes := exampleIntMetric()
	attributes.Clear()
	assert.Equal(t, prometheusTags(""), f.tags2String(pcommon.NewMap(), pcommon.NewMap()))
}

func TestPrometheusMetricDataTypeIntGauge(t *testing.T) {
	f := newPrometheusFormatter()
	metric, attributes := exampleIntGaugeMetric()

	result := f.metric2String(metric, attributes)
	expected := `gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`
	assert.Equal(t, expected, result)
}

func TestPrometheusMetricDataTypeDoubleGauge(t *testing.T) {
	f := newPrometheusFormatter()
	metric, attributes := exampleDoubleGaugeMetric()

	result := f.metric2String(metric, attributes)
	expected := `gauge_metric_name_double_test{foo="bar",local_name="156720",endpoint="http://example_url"} 33.4 1608124661169
gauge_metric_name_double_test{foo="bar",local_name="156155",endpoint="http://another_url"} 56.8 1608124662186`
	assert.Equal(t, expected, result)
}

func TestPrometheusMetricDataTypeIntSum(t *testing.T) {
	f := newPrometheusFormatter()
	metric, attributes := exampleIntSumMetric()

	result := f.metric2String(metric, attributes)
	expected := `sum_metric_int_test{foo="bar",name="156720",address="http://example_url"} 45 1608124444169
sum_metric_int_test{foo="bar",name="156155",address="http://another_url"} 1238 1608124699186`
	assert.Equal(t, expected, result)
}

func TestPrometheusMetricDataTypeDoubleSum(t *testing.T) {
	f := newPrometheusFormatter()
	metric, attributes := exampleDoubleSumMetric()

	result := f.metric2String(metric, attributes)
	expected := `sum_metric_double_test{foo="bar",pod_name="lorem",namespace="default"} 45.6 1618124444169
sum_metric_double_test{foo="bar",pod_name="opsum",namespace="kube-config"} 1238.1 1608424699186`
	assert.Equal(t, expected, result)
}

func TestPrometheusMetricDataTypeSummary(t *testing.T) {
	f := newPrometheusFormatter()
	metric, attributes := exampleSummaryMetric()

	result := f.metric2String(metric, attributes)
	expected := `summary_metric_double_test{foo="bar",quantile="0.6",pod_name="dolor",namespace="sumologic"} 0.7 1618124444169
summary_metric_double_test{foo="bar",quantile="2.6",pod_name="dolor",namespace="sumologic"} 4 1618124444169
summary_metric_double_test_sum{foo="bar",pod_name="dolor",namespace="sumologic"} 45.6 1618124444169
summary_metric_double_test_count{foo="bar",pod_name="dolor",namespace="sumologic"} 3 1618124444169
summary_metric_double_test_sum{foo="bar",pod_name="sit",namespace="main"} 1238.1 1608424699186
summary_metric_double_test_count{foo="bar",pod_name="sit",namespace="main"} 7 1608424699186`
	assert.Equal(t, expected, result)
}

func TestPrometheusMetricDataTypeHistogram(t *testing.T) {
	f := newPrometheusFormatter()
	metric, attributes := exampleHistogramMetric()

	result := f.metric2String(metric, attributes)
	expected := `histogram_metric_double_test_bucket{bar="foo",le="0.1",container="dolor",branch="sumologic"} 0 1618124444169
histogram_metric_double_test_bucket{bar="foo",le="0.2",container="dolor",branch="sumologic"} 12 1618124444169
histogram_metric_double_test_bucket{bar="foo",le="0.5",container="dolor",branch="sumologic"} 19 1618124444169
histogram_metric_double_test_bucket{bar="foo",le="0.8",container="dolor",branch="sumologic"} 24 1618124444169
histogram_metric_double_test_bucket{bar="foo",le="1",container="dolor",branch="sumologic"} 32 1618124444169
histogram_metric_double_test_bucket{bar="foo",le="+Inf",container="dolor",branch="sumologic"} 45 1618124444169
histogram_metric_double_test_sum{bar="foo",container="dolor",branch="sumologic"} 45.6 1618124444169
histogram_metric_double_test_count{bar="foo",container="dolor",branch="sumologic"} 7 1618124444169
histogram_metric_double_test_bucket{bar="foo",le="0.1",container="sit",branch="main"} 0 1608424699186
histogram_metric_double_test_bucket{bar="foo",le="0.2",container="sit",branch="main"} 10 1608424699186
histogram_metric_double_test_bucket{bar="foo",le="0.5",container="sit",branch="main"} 11 1608424699186
histogram_metric_double_test_bucket{bar="foo",le="0.8",container="sit",branch="main"} 12 1608424699186
histogram_metric_double_test_bucket{bar="foo",le="1",container="sit",branch="main"} 16 1608424699186
histogram_metric_double_test_bucket{bar="foo",le="+Inf",container="sit",branch="main"} 22 1608424699186
histogram_metric_double_test_sum{bar="foo",container="sit",branch="main"} 54.1 1608424699186
histogram_metric_double_test_count{bar="foo",container="sit",branch="main"} 98 1608424699186`
	assert.Equal(t, expected, result)
}

func TestEmptyPrometheusMetrics(t *testing.T) {
	type testCase struct {
		name       string
		metricFunc func(fillData bool) (pmetric.Metric, pcommon.Map)
		expected   string
	}

	tests := []testCase{
		{
			name:       "empty int gauge",
			metricFunc: buildExampleIntGaugeMetric,
			expected:   "",
		},
		{
			name:       "empty double gauge",
			metricFunc: buildExampleDoubleGaugeMetric,
			expected:   "",
		},
		{
			name:       "empty int sum",
			metricFunc: buildExampleIntSumMetric,
			expected:   "",
		},
		{
			name:       "empty double sum",
			metricFunc: buildExampleDoubleSumMetric,
			expected:   "",
		},
		{
			name:       "empty summary",
			metricFunc: buildExampleSummaryMetric,
			expected:   "",
		},
		{
			name:       "histogram with one datapoint, no sum or buckets",
			metricFunc: buildExampleHistogramMetric,
			expected:   `histogram_metric_double_test_count{bar="foo"} 0 0`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newPrometheusFormatter()

			result := f.metric2String(tt.metricFunc(false))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Benchmark_PrometheusFormatter_Metric2String(b *testing.B) {
	f := newPrometheusFormatter()

	metric, attributes := buildExampleHistogramMetric(true)

	for b.Loop() {
		_ = f.metric2String(metric, attributes)
	}
}
