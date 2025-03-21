// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/datatest"
)

// SummaryDP is a test helper for summary data points
type SummaryDP struct {
	Sum       float64
	Count     uint64
	Quantiles []QuantileValue
}

type QuantileValue struct {
	Quantile float64
	Value    float64
}

// Into converts SummaryDP to pmetric.SummaryDataPoint
func (s SummaryDP) Into() pmetric.SummaryDataPoint {
	dp := pmetric.NewSummaryDataPoint()
	dp.SetSum(s.Sum)
	dp.SetCount(s.Count)

	// Only add quantiles if we have them
	if len(s.Quantiles) > 0 {
		quantiles := dp.QuantileValues()
		for _, q := range s.Quantiles {
			qv := quantiles.AppendEmpty()
			qv.SetQuantile(q.Quantile)
			qv.SetValue(q.Value)
		}
	}
	return dp
}

// equalSummaryDataPoints compares two summary data points, treating nil and empty quantile slices as equivalent
func equalSummaryDataPoints(expected, actual pmetric.SummaryDataPoint) bool {
	if expected.Sum() != actual.Sum() || expected.Count() != actual.Count() {
		return false
	}

	// Both have no quantiles (empty or nil) - consider them equal
	if expected.QuantileValues().Len() == 0 && actual.QuantileValues().Len() == 0 {
		return true
	}

	// Different number of quantiles
	if expected.QuantileValues().Len() != actual.QuantileValues().Len() {
		return false
	}

	// Compare each quantile
	for i := range expected.QuantileValues().Len() {
		expQ := expected.QuantileValues().At(i)
		actQ := actual.QuantileValues().At(i)

		if expQ.Quantile() != actQ.Quantile() || expQ.Value() != actQ.Value() {
			return false
		}
	}

	return true
}

func TestSummary(t *testing.T) {
	// Test cases for Summary data type
	cases := []struct {
		name   string
		dp, in SummaryDP
		want   SummaryDP
	}{{
		name: "noop",
	}, {
		name: "simple_sum_count",
		dp:   SummaryDP{Sum: 100.0, Count: 10},
		in:   SummaryDP{Sum: 50.0, Count: 5},
		want: SummaryDP{Sum: 150.0, Count: 15},
	}, {
		name: "dp_without_quantiles_add_in_quantiles",
		dp:   SummaryDP{Sum: 100.0, Count: 10},
		in: SummaryDP{
			Sum:   50.0,
			Count: 5,
			Quantiles: []QuantileValue{
				{Quantile: 0.5, Value: 25.0},
				{Quantile: 0.9, Value: 45.0},
			},
		},
		want: SummaryDP{
			Sum:   150.0,
			Count: 15,
			// From input datapoint as per the implementation
			Quantiles: []QuantileValue{
				{Quantile: 0.5, Value: 25.0},
				{Quantile: 0.9, Value: 45.0},
			},
		},
	}, {
		name: "in_without_quantiles_clear_dp_quantiles",
		dp: SummaryDP{
			Sum:   100.0,
			Count: 10,
			Quantiles: []QuantileValue{
				{Quantile: 0.5, Value: 50.0},
				{Quantile: 0.9, Value: 90.0},
			},
		},
		in: SummaryDP{Sum: 50.0, Count: 5},
		want: SummaryDP{
			Sum:       150.0,
			Count:     15,
			Quantiles: []QuantileValue{},
		},
	}, {
		name: "with_quantiles_replace",
		dp: SummaryDP{
			Sum:   100.0,
			Count: 10,
			Quantiles: []QuantileValue{
				{Quantile: 0.5, Value: 50.0},
				{Quantile: 0.9, Value: 90.0},
			},
		},
		in: SummaryDP{
			Sum:   50.0,
			Count: 5,
			Quantiles: []QuantileValue{
				{Quantile: 0.25, Value: 12.5},
				{Quantile: 0.75, Value: 37.5},
			},
		},
		want: SummaryDP{
			Sum:   150.0,
			Count: 15,
			// From input datapoint as per the implementation
			Quantiles: []QuantileValue{
				{Quantile: 0.25, Value: 12.5},
				{Quantile: 0.75, Value: 37.5},
			},
		},
	}, {
		name: "different_quantiles",
		dp: SummaryDP{
			Sum:   100.0,
			Count: 10,
			Quantiles: []QuantileValue{
				{Quantile: 0.5, Value: 50.0},
				{Quantile: 0.9, Value: 90.0},
			},
		},
		in: SummaryDP{
			Sum:   50.0,
			Count: 5,
			Quantiles: []QuantileValue{
				{Quantile: 0.25, Value: 12.5},
				{Quantile: 0.75, Value: 37.5},
			},
		},
		want: SummaryDP{
			Sum:   150.0,
			Count: 15,
			// From input datapoint as per the implementation
			Quantiles: []QuantileValue{
				{Quantile: 0.25, Value: 12.5},
				{Quantile: 0.75, Value: 37.5},
			},
		},
	}}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			var add Adder
			is := datatest.New(t)

			var (
				dp   = cs.dp.Into()
				in   = cs.in.Into()
				want = cs.want.Into()
			)
			err := add.Summary(dp, in)
			is.Equal(nil, err)

			// Use custom equality check instead of direct comparison
			if !equalSummaryDataPoints(want, dp) {
				t.Errorf("Summary data points not equal\nexpected: %v\nactual: %v", want, dp)
			}
		})
	}
}
