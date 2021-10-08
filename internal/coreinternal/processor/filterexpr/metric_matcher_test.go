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

package filterexpr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestMetricCompileExprError(t *testing.T) {
	_, err := NewMetricMatcher("")
	require.Error(t, err)
}

func TestMetricRunExprError(t *testing.T) {
	matcher, err := NewMetricMatcher("foo")
	require.NoError(t, err)
	matched, _ := matcher.match(metricEnv{})
	require.False(t, matched)
}

func TestMetricUnknownDataType(t *testing.T) {
	matcher, err := NewMetricMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pdata.NewMetric()
	m.SetName("my.metric")
	m.SetDataType(-1)
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.False(t, matched)
}

func TestMetricEmptyGauge(t *testing.T) {
	testMetricEmptyValue(t, pdata.MetricDataTypeGauge)
}

func TestMetricEmptySum(t *testing.T) {
	testMetricEmptyValue(t, pdata.MetricDataTypeSum)
}

func TestMetricEmptyHistogram(t *testing.T) {
	testMetricEmptyValue(t, pdata.MetricDataTypeHistogram)
}

func testMetricEmptyValue(t *testing.T, dataType pdata.MetricDataType) {
	matcher, err := NewMetricMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pdata.NewMetric()
	m.SetName("my.metric")
	m.SetDataType(dataType)
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.False(t, matched)
}

func TestMetricGaugeEmptyDataPoint(t *testing.T) {
	matcher, err := NewMetricMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pdata.NewMetric()
	m.SetName("my.metric")
	m.SetDataType(pdata.MetricDataTypeGauge)
	m.Gauge().DataPoints().AppendEmpty()
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestMetricSumEmptyDataPoint(t *testing.T) {
	matcher, err := NewMetricMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pdata.NewMetric()
	m.SetName("my.metric")
	m.SetDataType(pdata.MetricDataTypeSum)
	m.Sum().DataPoints().AppendEmpty()
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestMetricHistogramEmptyDataPoint(t *testing.T) {
	matcher, err := NewMetricMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pdata.NewMetric()
	m.SetName("my.metric")
	m.SetDataType(pdata.MetricDataTypeHistogram)
	m.Histogram().DataPoints().AppendEmpty()
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestMetricMatchIntGaugeDataPointByMetricAndSecondPointLabelValue(t *testing.T) {
	matcher, err := NewMetricMatcher(
		`MetricName == 'my.metric' && Label("baz") == "glarch"`,
	)
	require.NoError(t, err)
	m := pdata.NewMetric()
	m.SetName("my.metric")
	m.SetDataType(pdata.MetricDataTypeGauge)
	dps := m.Gauge().DataPoints()

	dps.AppendEmpty().Attributes().InsertString("foo", "bar")
	dps.AppendEmpty().Attributes().InsertString("baz", "glarch")

	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestMetricMatchGaugeByMetricName(t *testing.T) {
	expression := `MetricName == 'my.metric'`
	assert.True(t, testMetricMatchGauge(t, "my.metric", expression, nil))
}

func TestMetricNonMatchGaugeByMetricName(t *testing.T) {
	expression := `MetricName == 'my.metric'`
	assert.False(t, testMetricMatchGauge(t, "foo.metric", expression, nil))
}

func TestMetricNonMatchGaugeDataPointByMetricAndHasLabel(t *testing.T) {
	expression := `MetricName == 'my.metric' && HasLabel("foo")`
	assert.False(t, testMetricMatchGauge(t, "foo.metric", expression, nil))
}

func TestMetricMatchGaugeDataPointByMetricAndHasLabel(t *testing.T) {
	expression := `MetricName == 'my.metric' && HasLabel("foo")`
	assert.True(t, testMetricMatchGauge(t, "my.metric", expression, map[string]pdata.AttributeValue{"foo": pdata.NewAttributeValueString("")}))
}

func TestMetricMatchGaugeDataPointByMetricAndLabelValue(t *testing.T) {
	expression := `MetricName == 'my.metric' && Label("foo") == "bar"`
	assert.False(t, testMetricMatchGauge(t, "my.metric", expression, map[string]pdata.AttributeValue{"foo": pdata.NewAttributeValueString("")}))
}

func TestMetricNonMatchGaugeDataPointByMetricAndLabelValue(t *testing.T) {
	expression := `MetricName == 'my.metric' && Label("foo") == "bar"`
	assert.False(t, testMetricMatchGauge(t, "my.metric", expression, map[string]pdata.AttributeValue{"foo": pdata.NewAttributeValueString("")}))
}

func testMetricMatchGauge(t *testing.T, metricName, expression string, lbls map[string]pdata.AttributeValue) bool {
	matcher, err := NewMetricMatcher(expression)
	require.NoError(t, err)
	m := pdata.NewMetric()
	m.SetName(metricName)
	m.SetDataType(pdata.MetricDataTypeGauge)
	dps := m.Gauge().DataPoints()
	pt := dps.AppendEmpty()
	if lbls != nil {
		pt.Attributes().InitFromMap(lbls)
	}
	match, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	return match
}

func TestMetricMatchSumByMetricName(t *testing.T) {
	assert.True(t, matchSum(t, "my.metric"))
}

func TestMetricNonMatchSumByMetricName(t *testing.T) {
	assert.False(t, matchSum(t, "foo.metric"))
}

func matchSum(t *testing.T, metricName string) bool {
	matcher, err := NewMetricMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pdata.NewMetric()
	m.SetName(metricName)
	m.SetDataType(pdata.MetricDataTypeSum)
	dps := m.Sum().DataPoints()
	dps.AppendEmpty()
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	return matched
}

func TestMetricMatchHistogramByMetricName(t *testing.T) {
	assert.True(t, matchHistogram(t, "my.metric"))
}

func TestMetricNonMatchHistogramByMetricName(t *testing.T) {
	assert.False(t, matchHistogram(t, "foo.metric"))
}

func matchHistogram(t *testing.T, metricName string) bool {
	matcher, err := NewMetricMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pdata.NewMetric()
	m.SetName(metricName)
	m.SetDataType(pdata.MetricDataTypeHistogram)
	dps := m.Histogram().DataPoints()
	dps.AppendEmpty()
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	return matched
}
