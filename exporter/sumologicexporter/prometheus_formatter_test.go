// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestSanitizeKey(t *testing.T) {
	f := newPrometheusFormatter()

	key := "&^*123-abc-ABC!?"
	expected := "___123_abc_ABC__"
	assert.Equal(t, expected, f.sanitizeKey(key))
}

func TestSanitizeValue(t *testing.T) {
	f := newPrometheusFormatter()

	value := `&^*123-abc-ABC!?"\\n`
	expected := `&^*123-abc-ABC!?\"\\\n`
	assert.Equal(t, expected, f.sanitizeValue(value))
}

func TestTags2StringNoLabels(t *testing.T) {
	f := newPrometheusFormatter()

	mp := exampleIntMetric()
	mp.attributes.Clear()
	assert.Equal(t, prometheusTags(""), f.tags2String(mp.attributes, pcommon.NewMap()))
}

func TestTags2String(t *testing.T) {
	f := newPrometheusFormatter()

	mp := exampleIntMetric()
	assert.Equal(
		t,
		prometheusTags(`{test="test_value",test2="second_value"}`),
		f.tags2String(mp.attributes, pcommon.NewMap()),
	)
}

func TestTags2StringNoAttributes(t *testing.T) {
	f := newPrometheusFormatter()
	assert.Equal(t, prometheusTags(""), f.tags2String(pcommon.NewMap(), pcommon.NewMap()))
}

func TestPrometheusMetricTypeIntGauge(t *testing.T) {
	f := newPrometheusFormatter()
	metric := exampleIntGaugeMetric()

	result := f.metric2String(metric)
	expected := `gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`
	assert.Equal(t, expected, result)

	metric = buildExampleIntGaugeMetric(false)
	result = f.metric2String(metric)
	expected = ""
	assert.Equal(t, expected, result)
}

func TestPrometheusMetricTypeDoubleGauge(t *testing.T) {
	f := newPrometheusFormatter()
	metric := exampleDoubleGaugeMetric()

	result := f.metric2String(metric)
	expected := `gauge_metric_name_double_test{foo="bar",local_name="156720",endpoint="http://example_url"} 33.4 1608124661169
gauge_metric_name_double_test{foo="bar",local_name="156155",endpoint="http://another_url"} 56.8 1608124662186`
	assert.Equal(t, expected, result)

	metric = buildExampleDoubleGaugeMetric(false)
	result = f.metric2String(metric)
	expected = ""
	assert.Equal(t, expected, result)
}

func TestPrometheusMetricTypeIntSum(t *testing.T) {
	f := newPrometheusFormatter()
	metric := exampleIntSumMetric()

	result := f.metric2String(metric)
	expected := `sum_metric_int_test{foo="bar",name="156720",address="http://example_url"} 45 1608124444169
sum_metric_int_test{foo="bar",name="156155",address="http://another_url"} 1238 1608124699186`
	assert.Equal(t, expected, result)

	metric = buildExampleIntSumMetric(false)
	result = f.metric2String(metric)
	expected = ""
	assert.Equal(t, expected, result)
}

func TestPrometheusMetricTypeDoubleSum(t *testing.T) {
	f := newPrometheusFormatter()
	metric := exampleDoubleSumMetric()

	result := f.metric2String(metric)
	expected := `sum_metric_double_test{foo="bar",pod_name="lorem",namespace="default"} 45.6 1618124444169
sum_metric_double_test{foo="bar",pod_name="opsum",namespace="kube-config"} 1238.1 1608424699186`
	assert.Equal(t, expected, result)

	metric = buildExampleDoubleSumMetric(false)
	result = f.metric2String(metric)
	expected = ""
	assert.Equal(t, expected, result)
}

func TestPrometheusMetricTypeSummary(t *testing.T) {
	f := newPrometheusFormatter()
	metric := exampleSummaryMetric()

	result := f.metric2String(metric)
	expected := `summary_metric_double_test{foo="bar",quantile="0.6",pod_name="dolor",namespace="sumologic"} 0.7 1618124444169
summary_metric_double_test{foo="bar",quantile="2.6",pod_name="dolor",namespace="sumologic"} 4 1618124444169
summary_metric_double_test_sum{foo="bar",pod_name="dolor",namespace="sumologic"} 45.6 1618124444169
summary_metric_double_test_count{foo="bar",pod_name="dolor",namespace="sumologic"} 3 1618124444169
summary_metric_double_test_sum{foo="bar",pod_name="sit",namespace="main"} 1238.1 1608424699186
summary_metric_double_test_count{foo="bar",pod_name="sit",namespace="main"} 7 1608424699186`
	assert.Equal(t, expected, result)

	metric = buildExampleSummaryMetric(false)
	result = f.metric2String(metric)
	expected = ""
	assert.Equal(t, expected, result)
}

func TestPrometheusMetricTypeHistogram(t *testing.T) {
	f := newPrometheusFormatter()
	metric := exampleHistogramMetric()

	result := f.metric2String(metric)
	expected := `histogram_metric_double_test{bar="foo",le="0.1",container="dolor",branch="sumologic"} 0 1618124444169
histogram_metric_double_test{bar="foo",le="0.2",container="dolor",branch="sumologic"} 12 1618124444169
histogram_metric_double_test{bar="foo",le="0.5",container="dolor",branch="sumologic"} 19 1618124444169
histogram_metric_double_test{bar="foo",le="0.8",container="dolor",branch="sumologic"} 24 1618124444169
histogram_metric_double_test{bar="foo",le="1",container="dolor",branch="sumologic"} 32 1618124444169
histogram_metric_double_test{bar="foo",le="+Inf",container="dolor",branch="sumologic"} 45 1618124444169
histogram_metric_double_test_sum{bar="foo",container="dolor",branch="sumologic"} 45.6 1618124444169
histogram_metric_double_test_count{bar="foo",container="dolor",branch="sumologic"} 7 1618124444169
histogram_metric_double_test{bar="foo",le="0.1",container="sit",branch="main"} 0 1608424699186
histogram_metric_double_test{bar="foo",le="0.2",container="sit",branch="main"} 10 1608424699186
histogram_metric_double_test{bar="foo",le="0.5",container="sit",branch="main"} 11 1608424699186
histogram_metric_double_test{bar="foo",le="0.8",container="sit",branch="main"} 12 1608424699186
histogram_metric_double_test{bar="foo",le="1",container="sit",branch="main"} 16 1608424699186
histogram_metric_double_test{bar="foo",le="+Inf",container="sit",branch="main"} 22 1608424699186
histogram_metric_double_test_sum{bar="foo",container="sit",branch="main"} 54.1 1608424699186
histogram_metric_double_test_count{bar="foo",container="sit",branch="main"} 98 1608424699186`
	assert.Equal(t, expected, result)

	metric = buildExampleHistogramMetric(false)
	result = f.metric2String(metric)
	expected = ""

	assert.Equal(t, expected, result)
}
