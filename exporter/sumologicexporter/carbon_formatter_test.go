// Copyright The OpenTelemetry Authors
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
)

func TestCarbon2TagString(t *testing.T) {
	metric := exampleIntMetric()
	data := carbon2TagString(metric)
	assert.Equal(t, "test=test_value test2=second_value metric=test.metric.data unit=bytes", data)

	metric = exampleIntGaugeMetric()
	data = carbon2TagString(metric)
	assert.Equal(t, "foo=bar metric=gauge_metric_name", data)

	metric = exampleDoubleSumMetric()
	data = carbon2TagString(metric)
	assert.Equal(t, "foo=bar metric=sum_metric_double_test", data)

	metric = exampleDoubleGaugeMetric()
	data = carbon2TagString(metric)
	assert.Equal(t, "foo=bar metric=gauge_metric_name_double_test", data)
}

func TestCarbonMetricTypeIntGauge(t *testing.T) {
	metric := exampleIntGaugeMetric()

	result := carbon2Metric2String(metric)
	expected := `foo=bar metric=gauge_metric_name  124 1608124661
foo=bar metric=gauge_metric_name  245 1608124662`
	assert.Equal(t, expected, result)
}

func TestCarbonMetricTypeDoubleGauge(t *testing.T) {
	metric := exampleDoubleGaugeMetric()

	result := carbon2Metric2String(metric)
	expected := `foo=bar metric=gauge_metric_name_double_test  33.4 1608124661
foo=bar metric=gauge_metric_name_double_test  56.8 1608124662`
	assert.Equal(t, expected, result)
}

func TestCarbonMetricTypeIntSum(t *testing.T) {
	metric := exampleIntSumMetric()

	result := carbon2Metric2String(metric)
	expected := `foo=bar metric=sum_metric_int_test  45 1608124444
foo=bar metric=sum_metric_int_test  1238 1608124699`
	assert.Equal(t, expected, result)
}

func TestCarbonMetricTypeDoubleSum(t *testing.T) {
	metric := exampleDoubleSumMetric()

	result := carbon2Metric2String(metric)
	expected := `foo=bar metric=sum_metric_double_test  45.6 1618124444
foo=bar metric=sum_metric_double_test  1238.1 1608424699`
	assert.Equal(t, expected, result)
}

func TestCarbonMetricTypeSummary(t *testing.T) {
	metric := exampleSummaryMetric()

	result := carbon2Metric2String(metric)
	expected := ``
	assert.Equal(t, expected, result)

	metric = buildExampleSummaryMetric(false)
	result = carbon2Metric2String(metric)
	assert.Equal(t, expected, result)
}

func TestCarbonMetricTypeHistogram(t *testing.T) {
	metric := exampleHistogramMetric()

	result := carbon2Metric2String(metric)
	expected := ``
	assert.Equal(t, expected, result)

	metric = buildExampleHistogramMetric(false)
	result = carbon2Metric2String(metric)
	assert.Equal(t, expected, result)
}
