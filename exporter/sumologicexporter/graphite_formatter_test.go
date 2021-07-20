// Copyright 2021, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sumologicexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscapeGraphiteString(t *testing.T) {
	gf, err := newGraphiteFormatter("%{k8s.cluster}.%{k8s.namespace}.%{k8s.pod}.%{_metric_}")
	require.NoError(t, err)

	value := gf.escapeGraphiteString("this.is_example&metric.value")
	expected := "this_is_example&metric_value"

	assert.Equal(t, expected, value)
}

func TestGraphiteFormat(t *testing.T) {
	gf, err := newGraphiteFormatter("%{k8s.cluster}.%{k8s.namespace}.%{k8s.pod}.%{_metric_}")
	require.NoError(t, err)

	fs := fieldsFromMap(map[string]string{
		"k8s.cluster":   "test_cluster",
		"k8s.namespace": "sumologic",
		"k8s.pod":       "example_pod",
	})

	expected := "test_cluster.sumologic.example_pod.test_metric"
	result := gf.format(fs, "test_metric")

	assert.Equal(t, expected, result)
}

func TestGraphiteMetricDataTypeIntGauge(t *testing.T) {
	gf, err := newGraphiteFormatter("%{cluster}.%{namespace}.%{pod}.%{_metric_}")
	require.NoError(t, err)

	metric := exampleIntGaugeMetric()
	metric.attributes.InsertString("cluster", "my_cluster")
	metric.attributes.InsertString("namespace", "default")
	metric.attributes.InsertString("pod", "some pod")

	result := gf.metric2String(metric)
	expected := `my_cluster.default.some_pod.gauge_metric_name 124 1608124661
my_cluster.default.some_pod.gauge_metric_name 245 1608124662`
	assert.Equal(t, expected, result)
}

func TestGraphiteMetricDataTypeDoubleGauge(t *testing.T) {
	gf, err := newGraphiteFormatter("%{cluster}.%{namespace}.%{pod}.%{_metric_}")
	require.NoError(t, err)

	metric := exampleDoubleGaugeMetric()
	metric.attributes.InsertString("cluster", "my_cluster")
	metric.attributes.InsertString("namespace", "default")
	metric.attributes.InsertString("pod", "some pod")

	result := gf.metric2String(metric)
	expected := `my_cluster.default.some_pod.gauge_metric_name_double_test 33.4 1608124661
my_cluster.default.some_pod.gauge_metric_name_double_test 56.8 1608124662`
	assert.Equal(t, expected, result)
}

func TestGraphiteNoattribute(t *testing.T) {
	gf, err := newGraphiteFormatter("%{cluster}.%{namespace}.%{pod}.%{_metric_}")
	require.NoError(t, err)

	metric := exampleDoubleGaugeMetric()
	metric.attributes.InsertString("cluster", "my_cluster")
	metric.attributes.InsertString("pod", "some pod")

	result := gf.metric2String(metric)
	expected := `my_cluster..some_pod.gauge_metric_name_double_test 33.4 1608124661
my_cluster..some_pod.gauge_metric_name_double_test 56.8 1608124662`
	assert.Equal(t, expected, result)
}

func TestGraphiteMetricDataTypeIntSum(t *testing.T) {
	gf, err := newGraphiteFormatter("%{cluster}.%{namespace}.%{pod}.%{_metric_}")
	require.NoError(t, err)

	metric := exampleIntSumMetric()
	metric.attributes.InsertString("cluster", "my_cluster")
	metric.attributes.InsertString("namespace", "default")
	metric.attributes.InsertString("pod", "some pod")

	result := gf.metric2String(metric)
	expected := `my_cluster.default.some_pod.sum_metric_int_test 45 1608124444
my_cluster.default.some_pod.sum_metric_int_test 1238 1608124699`
	assert.Equal(t, expected, result)
}

func TestGraphiteMetricDataTypeDoubleSum(t *testing.T) {
	gf, err := newGraphiteFormatter("%{cluster}.%{namespace}.%{pod}.%{_metric_}")
	require.NoError(t, err)

	metric := exampleDoubleSumMetric()
	metric.attributes.InsertString("cluster", "my_cluster")
	metric.attributes.InsertString("namespace", "default")
	metric.attributes.InsertString("pod", "some pod")

	result := gf.metric2String(metric)
	expected := `my_cluster.default.some_pod.sum_metric_double_test 45.6 1618124444
my_cluster.default.some_pod.sum_metric_double_test 1238.1 1608424699`
	assert.Equal(t, expected, result)
}

func TestGraphiteMetricDataTypeSummary(t *testing.T) {
	gf, err := newGraphiteFormatter("%{cluster}.%{namespace}.%{pod}.%{_metric_}")
	require.NoError(t, err)

	metric := exampleSummaryMetric()
	metric.attributes.InsertString("cluster", "my_cluster")
	metric.attributes.InsertString("namespace", "default")
	metric.attributes.InsertString("pod", "some pod")

	result := gf.metric2String(metric)
	expected := ``
	assert.Equal(t, expected, result)
}

func TestGraphiteMetricDataTypeHistogram(t *testing.T) {
	gf, err := newGraphiteFormatter("%{cluster}.%{namespace}.%{pod}.%{_metric_}")
	require.NoError(t, err)

	metric := exampleHistogramMetric()
	metric.attributes.InsertString("cluster", "my_cluster")
	metric.attributes.InsertString("namespace", "default")
	metric.attributes.InsertString("pod", "some pod")

	result := gf.metric2String(metric)
	expected := ``
	assert.Equal(t, expected, result)
}
