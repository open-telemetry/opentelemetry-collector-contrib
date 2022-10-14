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
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestCompileExprError(t *testing.T) {
	_, err := NewMatcher("")
	require.Error(t, err)
}

func TestRunExprError(t *testing.T) {
	matcher, err := NewMatcher("foo")
	require.NoError(t, err)
	matched, _ := matcher.match(env{})
	require.False(t, matched)
}

func TestUnknownDataType(t *testing.T) {
	matcher, err := NewMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pmetric.NewMetric()
	m.SetName("my.metric")
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.False(t, matched)
}

func TestDataTypeFilter(t *testing.T) {
	matcher, err := NewMatcher(`MetricType == 'Sum'`)
	require.NoError(t, err)

	m := pmetric.NewMetric()

	m.SetEmptyGauge().DataPoints().AppendEmpty()
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.False(t, matched)

	m.SetEmptySum().DataPoints().AppendEmpty()
	matched, err = matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestGaugeMatch(t *testing.T) {
	testMetricNameMatch(t, pmetric.MetricTypeGauge)
}

func TestSumMatch(t *testing.T) {
	testMetricNameMatch(t, pmetric.MetricTypeSum)
}

func TestHistogramMatch(t *testing.T) {
	testMetricNameMatch(t, pmetric.MetricTypeHistogram)
}

func TestExponentialHistogramMatch(t *testing.T) {
	testMetricNameMatch(t, pmetric.MetricTypeExponentialHistogram)
}

func TestSummaryMatch(t *testing.T) {
	testMetricNameMatch(t, pmetric.MetricTypeSummary)
}

func testMetricNameMatch(t *testing.T, dataType pmetric.MetricType) {
	matcher, err := NewMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pmetric.NewMetric()
	m.SetName("my.metric")

	// Empty metric - no match.
	switch dataType {
	case pmetric.MetricTypeGauge:
		m.SetEmptyGauge()
	case pmetric.MetricTypeSum:
		m.SetEmptySum()
	case pmetric.MetricTypeHistogram:
		m.SetEmptyHistogram()
	case pmetric.MetricTypeExponentialHistogram:
		m.SetEmptyExponentialHistogram()
	case pmetric.MetricTypeSummary:
		m.SetEmptySummary()
	}
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.False(t, matched)

	// Metric with one data point - match.
	switch dataType {
	case pmetric.MetricTypeGauge:
		m.Gauge().DataPoints().AppendEmpty()
	case pmetric.MetricTypeSum:
		m.Sum().DataPoints().AppendEmpty()
	case pmetric.MetricTypeHistogram:
		m.Histogram().DataPoints().AppendEmpty()
	case pmetric.MetricTypeExponentialHistogram:
		m.ExponentialHistogram().DataPoints().AppendEmpty()
	case pmetric.MetricTypeSummary:
		m.Summary().DataPoints().AppendEmpty()
	}
	matched, err = matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.True(t, matched)

}

func TestMatchIntGaugeDataPointByMetricAndSecondPointLabelValue(t *testing.T) {
	matcher, err := NewMatcher(
		`MetricName == 'my.metric' && Label("baz") == "glarch"`,
	)
	require.NoError(t, err)
	m := pmetric.NewMetric()
	m.SetName("my.metric")
	dps := m.SetEmptyGauge().DataPoints()

	dps.AppendEmpty().Attributes().PutStr("foo", "bar")
	dps.AppendEmpty().Attributes().PutStr("baz", "glarch")

	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestMatchGaugeByMetricName(t *testing.T) {
	expression := `MetricName == 'my.metric'`
	assert.True(t, testMatchGauge(t, "my.metric", expression, nil))
}

func TestNonMatchGaugeByMetricName(t *testing.T) {
	expression := `MetricName == 'my.metric'`
	assert.False(t, testMatchGauge(t, "foo.metric", expression, nil))
}

func TestNonMatchGaugeDataPointByMetricAndHasLabel(t *testing.T) {
	expression := `MetricName == 'my.metric' && HasLabel("foo")`
	assert.False(t, testMatchGauge(t, "foo.metric", expression, nil))
}

func TestMatchGaugeDataPointByMetricAndHasLabel(t *testing.T) {
	expression := `MetricName == 'my.metric' && HasLabel("foo")`
	assert.True(t, testMatchGauge(t, "my.metric", expression, map[string]interface{}{"foo": ""}))
}

func TestMatchGaugeDataPointByMetricAndLabelValue(t *testing.T) {
	expression := `MetricName == 'my.metric' && Label("foo") == "bar"`
	assert.False(t, testMatchGauge(t, "my.metric", expression, map[string]interface{}{"foo": ""}))
}

func TestNonMatchGaugeDataPointByMetricAndLabelValue(t *testing.T) {
	expression := `MetricName == 'my.metric' && Label("foo") == "bar"`
	assert.False(t, testMatchGauge(t, "my.metric", expression, map[string]interface{}{"foo": ""}))
}

func testMatchGauge(t *testing.T, metricName, expression string, lbls map[string]interface{}) bool {
	matcher, err := NewMatcher(expression)
	require.NoError(t, err)
	m := pmetric.NewMetric()
	m.SetName(metricName)
	dps := m.SetEmptyGauge().DataPoints()
	pt := dps.AppendEmpty()
	if lbls != nil {
		pt.Attributes().FromRaw(lbls)
	}
	match, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	return match
}

func TestMatchSumByMetricName(t *testing.T) {
	assert.True(t, matchSum(t, "my.metric"))
}

func TestNonMatchSumByMetricName(t *testing.T) {
	assert.False(t, matchSum(t, "foo.metric"))
}

func matchSum(t *testing.T, metricName string) bool {
	matcher, err := NewMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pmetric.NewMetric()
	m.SetName(metricName)
	dps := m.SetEmptySum().DataPoints()
	dps.AppendEmpty()
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	return matched
}

func TestMatchHistogramByMetricName(t *testing.T) {
	assert.True(t, matchHistogram(t, "my.metric"))
}

func TestNonMatchHistogramByMetricName(t *testing.T) {
	assert.False(t, matchHistogram(t, "foo.metric"))
}

func matchHistogram(t *testing.T, metricName string) bool {
	matcher, err := NewMatcher(`MetricName == 'my.metric'`)
	require.NoError(t, err)
	m := pmetric.NewMetric()
	m.SetName(metricName)
	dps := m.SetEmptyHistogram().DataPoints()
	dps.AppendEmpty()
	matched, err := matcher.MatchMetric(m)
	assert.NoError(t, err)
	return matched
}
