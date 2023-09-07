// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterexpr

import (
	"sync"
	"testing"

	"github.com/antonmedv/expr/vm"
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
	matched, _ := matcher.match(env{}, &vm.VM{})
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
		assert.NoError(t, pt.Attributes().FromRaw(lbls))
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

func TestParallel(t *testing.T) {
	matcher, err := NewMatcher(`MetricName == 'my.metric' && MetricType == 'Sum'`)
	require.NoError(t, err)

	wg := &sync.WaitGroup{}
	start := make(chan struct{})
	testMetric := func(t *testing.T, count int) {
		defer wg.Done()
		<-start
		for i := 0; i < count; i++ {
			m := pmetric.NewMetric()
			m.SetName("my.metric")
			m.SetEmptySum().DataPoints().AppendEmpty()
			matched, err := matcher.MatchMetric(m)
			assert.NoError(t, err)
			assert.True(t, matched)
		}
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go testMetric(t, 20)
	}

	close(start)
	wg.Wait()
}
