// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstestutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"
)

func TestSameMetrics(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	actual := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	diffs := diffMetricData(expected, actual)
	assert.Nil(t, diffs)
}

func TestDifferentValues(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	cfg := goldendataset.DefaultCfg()
	cfg.PtVal = 2
	actual := goldendataset.MetricsFromCfg(cfg)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 1)
}

func TestDifferentNumPts(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	cfg := goldendataset.DefaultCfg()
	cfg.NumPtsPerMetric = 2
	actual := goldendataset.MetricsFromCfg(cfg)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 1)
}

func TestDifferentPtValueTypes(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	cfg := goldendataset.DefaultCfg()
	cfg.MetricValueType = pmetric.NumberDataPointValueTypeDouble
	actual := goldendataset.MetricsFromCfg(cfg)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 1)
}

func TestHistogram(t *testing.T) {
	cfg1 := goldendataset.DefaultCfg()
	cfg1.MetricDescriptorType = pmetric.MetricTypeHistogram
	expected := goldendataset.MetricsFromCfg(cfg1)
	cfg2 := goldendataset.DefaultCfg()
	cfg2.MetricDescriptorType = pmetric.MetricTypeHistogram
	cfg2.PtVal = 2
	actual := goldendataset.MetricsFromCfg(cfg2)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 3)
}

// newHistogramMetrics returns metrics holding a single histogram data point with a sum, min and
// max set, after applying the given mutation to that point.
func newHistogramMetrics(mutate func(pmetric.HistogramDataPoint)) pmetric.Metrics {
	md := pmetric.NewMetrics()
	metric := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("test_histogram")
	pt := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
	pt.SetCount(3)
	pt.SetSum(6)
	pt.SetMin(1)
	pt.SetMax(3)
	mutate(pt)
	return md
}

// TestHistogramSumMinMax covers the HasSum, Min, HasMin, Max and HasMax fields of
// HistogramDataPoint, which the goldendataset-based tests above never vary.
func TestHistogramSumMinMax(t *testing.T) {
	// Each case asserts that the diffs it cares about are reported (Subset) and that no other diffs
	// slip in (Len), while staying independent of the order in which diffHistogramPt compares fields.
	// TestNoDiffForIdenticalHistogramSumMinMax covers the other direction, that no diff is reported
	// for points that are equal.
	//
	// The "* presence" cases set the field to zero on the expected side and leave it unset on the
	// actual side. Both read back as zero, so only the Has* comparison can tell them apart; that is
	// exactly the case a value-only comparison misses.
	tests := []struct {
		name           string
		mutateExpected func(pmetric.HistogramDataPoint)
		mutateActual   func(pmetric.HistogramDataPoint)
		wantMsgs       []string
	}{
		{
			name:         "different sum",
			mutateActual: func(pt pmetric.HistogramDataPoint) { pt.SetSum(7) },
			wantMsgs:     []string{"HistogramDataPoint Sum"},
		},
		{
			name:         "different min",
			mutateActual: func(pt pmetric.HistogramDataPoint) { pt.SetMin(0) },
			wantMsgs:     []string{"HistogramDataPoint Min"},
		},
		{
			name:         "different max",
			mutateActual: func(pt pmetric.HistogramDataPoint) { pt.SetMax(4) },
			wantMsgs:     []string{"HistogramDataPoint Max"},
		},
		{
			name:           "sum presence",
			mutateExpected: func(pt pmetric.HistogramDataPoint) { pt.SetSum(0) },
			mutateActual:   func(pt pmetric.HistogramDataPoint) { pt.RemoveSum() },
			wantMsgs:       []string{"HistogramDataPoint HasSum"},
		},
		{
			name:           "min presence",
			mutateExpected: func(pt pmetric.HistogramDataPoint) { pt.SetMin(0) },
			mutateActual:   func(pt pmetric.HistogramDataPoint) { pt.RemoveMin() },
			wantMsgs:       []string{"HistogramDataPoint HasMin"},
		},
		{
			name:           "max presence",
			mutateExpected: func(pt pmetric.HistogramDataPoint) { pt.SetMax(0) },
			mutateActual:   func(pt pmetric.HistogramDataPoint) { pt.RemoveMax() },
			wantMsgs:       []string{"HistogramDataPoint HasMax"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutateExpected := tt.mutateExpected
			if mutateExpected == nil {
				mutateExpected = func(pmetric.HistogramDataPoint) {}
			}
			expected := newHistogramMetrics(mutateExpected)
			actual := newHistogramMetrics(tt.mutateActual)

			diffs := DiffMetrics(nil, expected, actual)

			msgs := make([]string, 0, len(diffs))
			for _, d := range diffs {
				msgs = append(msgs, d.Msg)
			}
			assert.Subset(t, msgs, tt.wantMsgs)
			assert.Len(t, diffs, len(tt.wantMsgs))
		})
	}
}

func TestNoDiffForIdenticalHistogramSumMinMax(t *testing.T) {
	expected := newHistogramMetrics(func(pmetric.HistogramDataPoint) {})
	actual := newHistogramMetrics(func(pmetric.HistogramDataPoint) {})
	assert.Empty(t, DiffMetrics(nil, expected, actual))
}

func TestAttributes(t *testing.T) {
	cfg1 := goldendataset.DefaultCfg()
	cfg1.MetricDescriptorType = pmetric.MetricTypeHistogram
	cfg1.NumPtLabels = 1
	expected := goldendataset.MetricsFromCfg(cfg1)
	cfg2 := goldendataset.DefaultCfg()
	cfg2.MetricDescriptorType = pmetric.MetricTypeHistogram
	cfg2.NumPtLabels = 2
	actual := goldendataset.MetricsFromCfg(cfg2)
	diffs := DiffMetrics(nil, expected, actual)
	assert.Len(t, diffs, 1)
}

func TestExponentialHistogram(t *testing.T) {
	cfg1 := goldendataset.DefaultCfg()
	cfg1.MetricDescriptorType = pmetric.MetricTypeExponentialHistogram
	cfg1.PtVal = 1
	expected := goldendataset.MetricsFromCfg(cfg1)
	cfg2 := goldendataset.DefaultCfg()
	cfg2.MetricDescriptorType = pmetric.MetricTypeExponentialHistogram
	cfg2.PtVal = 3
	actual := goldendataset.MetricsFromCfg(cfg2)
	diffs := DiffMetrics(nil, expected, actual)
	assert.Len(t, diffs, 8)
}
