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
	tests := []struct {
		name    string
		mutate  func(pmetric.HistogramDataPoint)
		wantMsg string
	}{
		{
			name:    "different sum",
			mutate:  func(pt pmetric.HistogramDataPoint) { pt.SetSum(7) },
			wantMsg: "HistogramDataPoint Sum",
		},
		{
			name:    "different min",
			mutate:  func(pt pmetric.HistogramDataPoint) { pt.SetMin(0) },
			wantMsg: "HistogramDataPoint Min",
		},
		{
			name:    "different max",
			mutate:  func(pt pmetric.HistogramDataPoint) { pt.SetMax(4) },
			wantMsg: "HistogramDataPoint Max",
		},
		{
			name:    "sum unset",
			mutate:  func(pt pmetric.HistogramDataPoint) { pt.RemoveSum() },
			wantMsg: "HistogramDataPoint HasSum",
		},
		{
			name:    "min unset",
			mutate:  func(pt pmetric.HistogramDataPoint) { pt.RemoveMin() },
			wantMsg: "HistogramDataPoint HasMin",
		},
		{
			name:    "max unset",
			mutate:  func(pt pmetric.HistogramDataPoint) { pt.RemoveMax() },
			wantMsg: "HistogramDataPoint HasMax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := newHistogramMetrics(func(pmetric.HistogramDataPoint) {})
			actual := newHistogramMetrics(tt.mutate)

			diffs := DiffMetrics(nil, expected, actual)

			msgs := make([]string, 0, len(diffs))
			for _, d := range diffs {
				msgs = append(msgs, d.Msg)
			}
			assert.Contains(t, msgs, tt.wantMsg)
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
