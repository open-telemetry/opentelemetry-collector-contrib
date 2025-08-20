// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/metadata"
)

var (
	zeroFlag    = pmetric.DefaultDataPointFlags
	noValueFlag = pmetric.DefaultDataPointFlags.WithNoRecordedValue(true)
)

type testSumMetric struct {
	metricNames  []string
	metricValues [][]float64
	isCumulative []bool
	isMonotonic  []bool
	flags        [][]pmetric.DataPointFlags
}

func (tm testSumMetric) addToMetrics(ms pmetric.MetricSlice, now time.Time) {
	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		sum := m.SetEmptySum()
		sum.SetIsMonotonic(tm.isMonotonic[i])

		if tm.isCumulative[i] {
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}

		for index, value := range tm.metricValues[i] {
			dp := m.Sum().DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Second)))
			dp.SetDoubleValue(value)
			if len(tm.flags) > i && len(tm.flags[i]) > index {
				dp.SetFlags(tm.flags[i][index])
			}
		}
	}
}

type testHistogramMetric struct {
	metricNames   []string
	metricCounts  [][]uint64
	metricSums    [][]float64
	metricMins    [][]float64
	metricMaxes   [][]float64
	metricBuckets [][][]uint64
	isCumulative  []bool
	flags         [][]pmetric.DataPointFlags
}

func (tm testHistogramMetric) addToMetrics(ms pmetric.MetricSlice, now time.Time) {
	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		hist := m.SetEmptyHistogram()
		m.SetName(name)

		if tm.isCumulative[i] {
			hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}

		for index, count := range tm.metricCounts[i] {
			dp := m.Histogram().DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Second)))
			dp.SetCount(count)

			sums := tm.metricSums[i]
			if len(sums) > 0 {
				dp.SetSum(sums[index])
			}
			if tm.metricMins != nil {
				mins := tm.metricMins[i]
				if len(mins) > 0 {
					dp.SetMin(mins[index])
				}
			}
			if tm.metricMaxes != nil {
				maxes := tm.metricMaxes[i]
				if len(maxes) > 0 {
					dp.SetMax(maxes[index])
				}
			}
			dp.BucketCounts().FromRaw(tm.metricBuckets[i][index])
			if len(tm.flags) > i && len(tm.flags[i]) > index {
				dp.SetFlags(tm.flags[i][index])
			}
		}
	}
}

type testExpHistogramMetric struct {
	metricNames           []string
	metricScales          [][]int32
	metricPosIndexOffsets [][]int32
	metricNegIndexOffsets [][]int32
	metricZeroThresholds  [][]float64
	isCumulative          []bool
	flags                 [][]pmetric.DataPointFlags

	metricCounts     [][]uint64
	metricSums       [][]float64
	metricMins       [][]float64
	metricMaxes      [][]float64
	metricZeroCounts [][]uint64
	metricPosCounts  [][][]uint64
	metricNegCounts  [][][]uint64
}

func (tm testExpHistogramMetric) addToMetrics(ms pmetric.MetricSlice, now time.Time) {
	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		hist := m.SetEmptyExponentialHistogram()

		if tm.isCumulative[i] {
			hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}

		for index, count := range tm.metricCounts[i] {
			dp := m.ExponentialHistogram().DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Second)))
			dp.SetCount(count)
			dp.SetScale(tm.metricScales[i][index])

			sums := tm.metricSums[i]
			if len(sums) > 0 {
				dp.SetSum(sums[index])
			}
			if tm.metricMins != nil {
				mins := tm.metricMins[i]
				if len(mins) > 0 {
					dp.SetMin(mins[index])
				}
			}
			if tm.metricMaxes != nil {
				maxes := tm.metricMaxes[i]
				if len(maxes) > 0 {
					dp.SetMax(maxes[index])
				}
			}
			if len(tm.flags) > i && len(tm.flags[i]) > index {
				dp.SetFlags(tm.flags[i][index])
			}

			dp.SetZeroCount(tm.metricZeroCounts[i][index])
			dp.SetZeroThreshold(tm.metricZeroThresholds[i][index])
			dp.Positive().BucketCounts().FromRaw(tm.metricPosCounts[i][index])
			dp.Positive().SetOffset(tm.metricPosIndexOffsets[i][index])
			dp.Negative().BucketCounts().FromRaw(tm.metricNegCounts[i][index])
			dp.Negative().SetOffset(tm.metricNegIndexOffsets[i][index])
		}
	}
}

type cumulativeToDeltaTest struct {
	name       string
	include    MatchMetrics
	exclude    MatchMetrics
	inMetrics  pmetric.Metrics
	outMetrics pmetric.Metrics
	wantError  error
}

func TestCumulativeToDeltaProcessor(t *testing.T) {
	testCases := []cumulativeToDeltaTest{
		{
			name: "cumulative_to_delta_convert_nothing",
			exclude: MatchMetrics{
				Metrics: []string{".*"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isCumulative: []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isCumulative: []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
		},
		{
			name: "cumulative_to_delta_one_positive",
			include: MatchMetrics{
				Metrics: []string{"metric_1"},
				Config: filterset.Config{
					MatchType:    "strict",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{0, 100, 200, 500}, {4}},
				isCumulative: []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 100, 300}, {4}},
				isCumulative: []bool{false, true},
				isMonotonic:  []bool{true, true},
			}),
		},
		{
			name: "cumulative_to_delta_nan_value",
			include: MatchMetrics{
				Metrics: []string{"_1"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{0, 100, 200, math.NaN()}, {4}},
				isCumulative: []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 100, math.NaN()}, {4}},
				isCumulative: []bool{false, true},
				isMonotonic:  []bool{true, true},
			}),
		},
		{
			name: "cumulative_to_delta_nodata",
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{0, 100, 0, 200, 400}, {0, 100, 0, 0, 400}},
				isCumulative: []bool{true, true},
				isMonotonic:  []bool{true, true},
				flags: [][]pmetric.DataPointFlags{
					{zeroFlag, zeroFlag, noValueFlag, zeroFlag, zeroFlag},
					{zeroFlag, zeroFlag, noValueFlag, noValueFlag, zeroFlag},
				},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 100, 200}, {100, 300}},
				isCumulative: []bool{false, false},
				isMonotonic:  []bool{true, true},
				flags: [][]pmetric.DataPointFlags{
					{zeroFlag, zeroFlag, zeroFlag},
					{zeroFlag, zeroFlag},
				},
			}),
		},
		{
			name: "cumulative_to_delta_exclude_precedence",
			include: MatchMetrics{
				Metrics: []string{".*"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			exclude: MatchMetrics{
				Metrics: []string{".*"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isCumulative: []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isCumulative: []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
		},
		{
			name: "cumulative_to_delta_histogram_min_and_max",
			include: MatchMetrics{
				Metrics: []string{"metric_1"},
				Config: filterset.Config{
					MatchType:    "strict",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricCounts: [][]uint64{{0, 100, 200, 500}, {4}},
				metricSums:   [][]float64{{0, 100, 200, 500}, {4}},
				metricBuckets: [][][]uint64{
					{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					{{4, 4, 4}},
				},
				metricMins: [][]float64{
					{0, 5.0, 2.0, 3.0},
					{2.0},
				},
				metricMaxes: [][]float64{
					{0, 800.0, 825.0, 800.0},
					{3.0},
				},
				isCumulative: []bool{true, true},
			}),
			outMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricCounts: [][]uint64{{100, 100, 300}, {4}},
				metricSums:   [][]float64{{100, 100, 300}, {4}},
				metricBuckets: [][][]uint64{
					{{50, 25, 25}, {50, 25, 25}, {150, 75, 75}},
					{{4, 4, 4}},
				},
				metricMins: [][]float64{
					nil,
					{2.0},
				},
				metricMaxes: [][]float64{
					nil,
					{3.0},
				},
				isCumulative: []bool{false, true},
			}),
		},
		{
			name: "cumulative_to_delta_histogram_one_positive",
			include: MatchMetrics{
				Metrics: []string{"metric_1"},
				Config: filterset.Config{
					MatchType:    "strict",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricCounts: [][]uint64{{0, 100, 200, 500}, {4}},
				metricSums:   [][]float64{{0, 100, 200, 500}, {4}},
				metricBuckets: [][][]uint64{
					{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					{{4, 4, 4}},
				},
				isCumulative: []bool{true, true},
			}),
			outMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricCounts: [][]uint64{{100, 100, 300}, {4}},
				metricSums:   [][]float64{{100, 100, 300}, {4}},
				metricBuckets: [][][]uint64{
					{{50, 25, 25}, {50, 25, 25}, {150, 75, 75}},
					{{4, 4, 4}},
				},
				isCumulative: []bool{false, true},
			}),
		},
		{
			name: "cumulative_to_delta_histogram_nan_sum",
			include: MatchMetrics{
				Metrics: []string{"metric_1"},
				Config: filterset.Config{
					MatchType:    "strict",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricCounts: [][]uint64{{0, 100, 200, 500}, {4}},
				metricSums:   [][]float64{{0, 100, math.NaN(), 500}, {4}},
				metricBuckets: [][][]uint64{
					{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					{{4, 4, 4}},
				},
				isCumulative: []bool{true, true},
			}),
			outMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricCounts: [][]uint64{{100, 100, 300}, {4}},
				metricSums:   [][]float64{{100, math.NaN(), 400}, {4}},
				metricBuckets: [][][]uint64{
					{{50, 25, 25}, {50, 25, 25}, {150, 75, 75}},
					{{4, 4, 4}},
				},
				isCumulative: []bool{false, true},
			}),
		},
		{
			name: "cumulative_to_delta_histogram_novalue",
			inMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricCounts: [][]uint64{{0, 100, 0, 500}, {0, 2, 0, 0, 16}},
				metricSums:   [][]float64{{0, 100, 0, 500}, {0, 3, 0, 0, 81}},
				metricBuckets: [][][]uint64{
					{{0, 0, 0}, {50, 25, 25}, {0, 0, 0}, {250, 125, 125}},
					{{0, 0, 0}, {1, 1, 1}, {0, 0, 0}, {0, 0, 0}, {21, 40, 20}},
				},
				isCumulative: []bool{true, true},
				flags: [][]pmetric.DataPointFlags{
					{zeroFlag, zeroFlag, noValueFlag, zeroFlag},
					{zeroFlag, zeroFlag, noValueFlag, noValueFlag, zeroFlag},
				},
			}),
			outMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricCounts: [][]uint64{{100, 400}, {2, 14}},
				metricSums:   [][]float64{{100, 400}, {3, 78}},
				metricBuckets: [][][]uint64{
					{{50, 25, 25}, {200, 100, 100}},
					{{1, 1, 1}, {20, 39, 19}},
				},
				isCumulative: []bool{false, false},
				flags: [][]pmetric.DataPointFlags{
					{zeroFlag, zeroFlag},
					{zeroFlag, zeroFlag},
				},
			}),
		},
		{
			name: "cumulative_to_delta_histogram_one_positive_without_sums",
			include: MatchMetrics{
				Metrics: []string{"metric_1"},
				Config: filterset.Config{
					MatchType:    "strict",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricCounts: [][]uint64{{0, 100, 200, 500}, {4}},
				metricSums:   [][]float64{{}, {4}},
				metricBuckets: [][][]uint64{
					{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					{{4, 4, 4}},
				},
				isCumulative: []bool{true, true},
			}),
			outMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricCounts: [][]uint64{{100, 100, 300}, {4}},
				metricSums:   [][]float64{{}, {4}},
				metricBuckets: [][][]uint64{
					{{50, 25, 25}, {50, 25, 25}, {150, 75, 75}},
					{{4, 4, 4}},
				},
				isCumulative: []bool{false, true},
			}),
		},
		{
			name: "cumulative_to_delta_exphistogram_min_and_max",
			include: MatchMetrics{
				Metrics: []string{"metric_1"},
				Config: filterset.Config{
					MatchType:    "strict",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1", "metric_2"},
				metricScales:          [][]int32{{0, 0, 0, 0}, {1}},
				metricPosIndexOffsets: [][]int32{{5, 5, 5, 5}, {0}},
				metricNegIndexOffsets: [][]int32{{-2, -2, -2, -2}, {0}},
				metricZeroThresholds:  [][]float64{{0, 0, 0, 0}, {0}},
				isCumulative:          []bool{true, true},

				metricCounts:     [][]uint64{{0, 180, 220, 283}, {114}},
				metricSums:       [][]float64{{0, 150, 180, 225}, {90}},
				metricMins:       [][]float64{{0, 0.1, 0.15, 0.2}, {0.1}},
				metricMaxes:      [][]float64{{0, 10.0, 12.0, 15.0}, {8.0}},
				metricZeroCounts: [][]uint64{{0, 5, 10, 20}, {2}},
				metricPosCounts: [][][]uint64{
					{{0}, {100, 50, 25}, {120, 60, 30}, {150, 75, 38}},
					{{64, 32, 16}},
				},
				metricNegCounts: [][][]uint64{
					{{}, {}, {}, {}},
					{{}},
				},
			}),
			outMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1", "metric_2"},
				metricScales:          [][]int32{{0, 0, 0}, {1}},
				metricPosIndexOffsets: [][]int32{{5, 5, 5}, {0}},
				metricNegIndexOffsets: [][]int32{{-2, -2, -2}, {0}},
				metricZeroThresholds:  [][]float64{{0, 0, 0}, {0}},
				isCumulative:          []bool{false, true},

				metricCounts:     [][]uint64{{180, 40, 63}, {114}},
				metricSums:       [][]float64{{150, 30, 45}, {90}},
				metricMins:       [][]float64{nil, {0.1}},
				metricMaxes:      [][]float64{nil, {8.0}},
				metricZeroCounts: [][]uint64{{5, 8, 10}, {2}},
				metricPosCounts: [][][]uint64{
					{{100, 50, 25}, {20, 10, 5}, {30, 15, 8}},
					{{64, 32, 16}},
				},
				metricNegCounts: [][][]uint64{
					{{}, {}, {}},
					{{}},
				},
			}),
		},
		{
			name: "cumulative_to_delta_exphistogram_nan_sum",
			include: MatchMetrics{
				Metrics: []string{"metric_1"},
				Config: filterset.Config{
					MatchType:    "strict",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1", "metric_2"},
				metricScales:          [][]int32{{0, 0, 0, 0}, {1}},
				metricPosIndexOffsets: [][]int32{{-2, -2, -2, -2}, {0}},
				metricNegIndexOffsets: [][]int32{{5, 5, 5, 5}, {0}},
				metricZeroThresholds:  [][]float64{{0, 0, 0, 0}, {0}},
				isCumulative:          []bool{true, true},

				metricCounts:     [][]uint64{{0, 100, 200, 500}, {4}},
				metricSums:       [][]float64{{0, 100, math.NaN(), 500}, {4}},
				metricZeroCounts: [][]uint64{{0, 5, 8, 10}, {2}},
				metricPosCounts: [][][]uint64{
					{{0}, {100, 50, 25}, {120, 60, 30}, {150, 75, 38}},
					{{64, 32, 16}},
				},
				metricNegCounts: [][][]uint64{
					{{}, {}, {}, {}},
					{{}},
				},
			}),
			outMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1", "metric_2"},
				metricScales:          [][]int32{{0, 0, 0}, {1}},
				metricPosIndexOffsets: [][]int32{{-2, -2, -2}, {0}},
				metricNegIndexOffsets: [][]int32{{5, 5, 5}, {0}},
				metricZeroThresholds:  [][]float64{{0, 0, 0}, {0}},
				isCumulative:          []bool{false, true},

				metricZeroCounts: [][]uint64{{5, 3, 2}, {2}},
				metricCounts:     [][]uint64{{100, 100, 300}, {4}},
				metricSums:       [][]float64{{100, math.NaN(), 400}, {4}},
				metricPosCounts: [][][]uint64{
					{{100, 50, 25}, {20, 10, 5}, {30, 15, 8}},
					{{64, 32, 16}},
				},
				metricNegCounts: [][][]uint64{
					{{}, {}, {}},
					{{}},
				},
			}),
		},
		{
			name: "cumulative_to_delta_exphistogram_novalue",
			inMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1", "metric_2"},
				metricScales:          [][]int32{{0, 0, 0, 0}, {1, 1, 1, 1, 1}},
				metricPosIndexOffsets: [][]int32{{0, 0, 0, 0}, {0, 0, 0, 0, 0}},
				metricNegIndexOffsets: [][]int32{{0, 0, 0, 0}, {0, 0, 0, 0, 0}},
				metricZeroThresholds:  [][]float64{{0, 0, 0, 0}, {0, 0, 0, 0, 0}},
				isCumulative:          []bool{true, true},
				flags: [][]pmetric.DataPointFlags{
					{zeroFlag, zeroFlag, noValueFlag, zeroFlag},
					{zeroFlag, zeroFlag, noValueFlag, noValueFlag, zeroFlag},
				},

				metricCounts:     [][]uint64{{0, 100, 0, 500}, {0, 2, 0, 0, 16}},
				metricSums:       [][]float64{{0, 100, 0, 500}, {0, 3, 0, 0, 81}},
				metricZeroCounts: [][]uint64{{0, 5, 0, 10}, {0, 2, 5, 10, 12}},
				metricPosCounts: [][][]uint64{
					{{0, 0, 0}, {50, 25, 25}, {0, 0, 0}, {250, 125, 125}},
					{{0, 0, 0}, {1, 1, 1}, {0, 0, 0}, {0, 0, 0}, {21, 40, 20}},
				},
				metricNegCounts: [][][]uint64{
					{{0, 0, 0}, {100, 50, 25}, {0, 0, 0}, {150, 75, 38}},
					{{0, 0, 0}, {2, 2, 2}, {0, 0, 0}, {0, 0, 0}, {4, 3, 4}},
				},
			}),
			outMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1", "metric_2"},
				metricScales:          [][]int32{{0, 0}, {1, 1}},
				metricPosIndexOffsets: [][]int32{{0, 0}, {0, 0}},
				metricNegIndexOffsets: [][]int32{{0, 0}, {0, 0}},
				metricZeroThresholds:  [][]float64{{0, 0}, {0, 0}},
				isCumulative:          []bool{false, false},
				flags: [][]pmetric.DataPointFlags{
					{zeroFlag, zeroFlag},
					{zeroFlag, zeroFlag},
				},

				metricCounts:     [][]uint64{{100, 400}, {2, 14}},
				metricSums:       [][]float64{{100, 400}, {3, 78}},
				metricZeroCounts: [][]uint64{{0, 5}, {2, 5}},
				metricPosCounts: [][][]uint64{
					{{50, 25, 25}, {200, 100, 100}},
					{{1, 1, 1}, {20, 39, 19}},
				},
				metricNegCounts: [][][]uint64{
					{{100, 50, 25}, {50, 25, 13}},
					{{2, 2, 2}, {2, 1, 2}},
				},
			}),
		},
		{
			name: "cumulative_to_delta_exphistogram_one_positive_without_sums",
			include: MatchMetrics{
				Metrics: []string{"metric_1"},
				Config: filterset.Config{
					MatchType:    "strict",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1", "metric_2"},
				metricScales:          [][]int32{{0, 0, 0, 0}, {1}},
				metricPosIndexOffsets: [][]int32{{0, 0, 0, 0}, {0}},
				metricNegIndexOffsets: [][]int32{{0, 0, 0, 0}, {0}},
				metricZeroThresholds:  [][]float64{{0, 0, 0, 0}, {0}},
				metricZeroCounts:      [][]uint64{{0, 5, 105, 106}, {2}},
				isCumulative:          []bool{true, true},

				metricCounts: [][]uint64{{0, 100, 200, 500}, {4}},
				metricSums:   [][]float64{{}, {4}},
				metricPosCounts: [][][]uint64{
					{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					{{4, 4, 4}},
				},
				metricNegCounts: [][][]uint64{
					{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					{{4, 4, 4}},
				},
			}),
			outMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1", "metric_2"},
				metricScales:          [][]int32{{0, 0, 0}, {1}},
				metricPosIndexOffsets: [][]int32{{0, 0, 0}, {0}},
				metricNegIndexOffsets: [][]int32{{0, 0, 0}, {0}},
				metricZeroThresholds:  [][]float64{{0, 0, 0}, {0}},
				isCumulative:          []bool{false, true},

				metricZeroCounts: [][]uint64{{5, 100, 1}, {2}},
				metricCounts:     [][]uint64{{100, 100, 300}, {4}},
				metricSums:       [][]float64{{}, {4}},
				metricPosCounts: [][][]uint64{
					{{50, 25, 25}, {50, 25, 25}, {150, 75, 75}},
					{{4, 4, 4}},
				},
				metricNegCounts: [][][]uint64{
					{{50, 25, 25}, {50, 25, 25}, {150, 75, 75}},
					{{4, 4, 4}},
				},
			}),
		},
		{
			name: "cumulative_to_delta_exphistogram_incompatible",
			inMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1"},
				metricScales:          [][]int32{{0, 0, 1, 0, 0, 0}},
				metricPosIndexOffsets: [][]int32{{0, 0, 0, -1, 0, 0}},
				metricNegIndexOffsets: [][]int32{{0, 0, 0, 0, -1, 0}},
				metricZeroThresholds:  [][]float64{{0, 0, 0, 0, 0, 0.5}},
				isCumulative:          []bool{true},

				metricZeroCounts: [][]uint64{{0, 5, 75, 105, 106, 109}},
				metricCounts:     [][]uint64{{0, 205, 385, 505, 1269}},
				metricSums:       [][]float64{{0, 30, 35, 40, 50, 200}},
				metricPosCounts: [][][]uint64{
					{{0, 0, 0}, {50, 25, 25}, {75, 40, 40}, {100, 50, 50}, {250, 125, 125}, {280, 150, 150}},
				},
				metricNegCounts: [][][]uint64{
					{{0, 0, 0}, {50, 25, 25}, {75, 40, 40}, {100, 50, 50}, {250, 125, 125}, {280, 150, 150}},
				},
			}),
			outMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1"},
				metricScales:          [][]int32{{0}},
				metricPosIndexOffsets: [][]int32{{0}},
				metricNegIndexOffsets: [][]int32{{0}},
				metricZeroThresholds:  [][]float64{{0}},
				isCumulative:          []bool{false},

				metricZeroCounts: [][]uint64{{5}},
				metricCounts:     [][]uint64{{205}},
				metricSums:       [][]float64{{30}},
				metricPosCounts: [][][]uint64{
					{{50, 25, 25}},
				},
				metricNegCounts: [][][]uint64{
					{{50, 25, 25}},
				},
			}),
		},
		{
			name: "cumulative_to_delta_exphistogram_bucket_count_reset",
			inMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1"},
				metricScales:          [][]int32{{0, 0, 0, 0}},
				metricPosIndexOffsets: [][]int32{{5, 5, 5, 5}},
				metricNegIndexOffsets: [][]int32{{-2, -2, -2, -2}},
				metricZeroThresholds:  [][]float64{{0, 0, 0, 0}},
				isCumulative:          []bool{true},

				metricCounts:     [][]uint64{{0, 360, 370, 618}},
				metricSums:       [][]float64{{0, 150, 180, 225}},
				metricZeroCounts: [][]uint64{{0, 5, 10, 20}},
				metricPosCounts: [][][]uint64{
					{{0}, {100, 50, 25}, {180}, {210, 75, 38}},
				},
				metricNegCounts: [][][]uint64{
					{{0}, {80, 60, 40}, {180}, {180, 25, 100}},
				},
			}),
			outMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1"},
				metricScales:          [][]int32{{0, 0}},
				metricPosIndexOffsets: [][]int32{{5, 5}},
				metricNegIndexOffsets: [][]int32{{-2, -2}},
				metricZeroThresholds:  [][]float64{{0, 0}},
				isCumulative:          []bool{false},

				metricCounts:     [][]uint64{{360, 248}},
				metricSums:       [][]float64{{150, 45}},
				metricZeroCounts: [][]uint64{{5, 10}},
				metricPosCounts: [][][]uint64{
					{{100, 50, 25}, {30, 75, 38}},
				},
				metricNegCounts: [][][]uint64{
					{{80, 60, 40}, {0, 25, 100}},
				},
			}),
		},
		{
			name: "cumulative_to_delta_exphistogram_count_reset",
			inMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1"},
				metricScales:          [][]int32{{0, 0, 0, 0, 0, 0}},
				metricPosIndexOffsets: [][]int32{{5, 5, 5, 5, 5, 5}},
				metricNegIndexOffsets: [][]int32{{-2, -2, -2, -2, -2, -2}},
				metricZeroThresholds:  [][]float64{{0, 0, 0, 0, 0, 0}},
				isCumulative:          []bool{true},

				metricCounts:     [][]uint64{{0, 360, 320, 608, 685, 770}},
				metricSums:       [][]float64{{0, 150, 180, 225, 200, 100}},
				metricZeroCounts: [][]uint64{{0, 5, 10, 20, 10, 15}},
				metricPosCounts: [][][]uint64{
					{{0, 0, 0}, {100, 50, 25}, {50, 20, 30}, {150, 75, 38}, {175, 90, 50}, {190, 100, 75}},
				},
				metricNegCounts: [][][]uint64{
					{{0, 0, 0}, {80, 60, 40}, {100, 50, 60}, {150, 75, 100}, {180, 80, 100}, {200, 85, 105}},
				},
			}),
			outMetrics: generateTestExpHistogramMetrics(testExpHistogramMetric{
				metricNames:           []string{"metric_1"},
				metricScales:          [][]int32{{0, 0, 0}},
				metricPosIndexOffsets: [][]int32{{5, 5, 5}},
				metricNegIndexOffsets: [][]int32{{-2, -2, -2}},
				metricZeroThresholds:  [][]float64{{0, 0, 0}},
				isCumulative:          []bool{false},

				metricCounts:     [][]uint64{{360, 288, 85}},
				metricSums:       [][]float64{{150, 45, -100}},
				metricZeroCounts: [][]uint64{{5, 10, 5}},
				metricPosCounts: [][][]uint64{
					{{100, 50, 25}, {100, 55, 8}, {15, 10, 25}},
				},
				metricNegCounts: [][][]uint64{
					{{80, 60, 40}, {50, 25, 40}, {20, 5, 5}},
				},
			}),
		},

		{
			name: "cumulative_to_delta_all",
			include: MatchMetrics{
				Metrics: []string{".*"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{0, 100, 200, 500}, {0, 4, 5}},
				isCumulative: []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 100, 300}, {4, 1}},
				isCumulative: []bool{false, false},
				isMonotonic:  []bool{true, true},
			}),
		},
		{
			name: "cumulative_to_delta_remove_metric_1",
			include: MatchMetrics{
				Metrics: []string{".*"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			exclude: MatchMetrics{
				Metrics: []string{"metric_1"},
				Config: filterset.Config{
					MatchType:    "strict",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 200, 500}, {0, 4, 5}},
				isCumulative: []bool{true, true},
				isMonotonic:  []bool{true, true},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 200, 500}, {4, 1}},
				isCumulative: []bool{true, false},
				isMonotonic:  []bool{true, true},
			}),
		},
		{
			name: "cumulative_to_delta_remove_non_monotonic",
			include: MatchMetrics{
				Metrics: []string{".*"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{0, 100, 200, 500}, {4, 5}},
				isCumulative: []bool{true, true},
				isMonotonic:  []bool{true, false},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 100, 300}, {4, 5}},
				isCumulative: []bool{false, true},
				isMonotonic:  []bool{true, true},
			}),
		},
		{
			name: "cumulative_to_delta_restart_detected",
			include: MatchMetrics{
				Metrics: []string{".*"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1"},
				metricValues: [][]float64{{100, 105, 120, 100, 110}},
				isCumulative: []bool{true},
				isMonotonic:  []bool{true},
			}),
			outMetrics: generateTestSumMetrics(testSumMetric{
				metricNames:  []string{"metric_1"},
				metricValues: [][]float64{{5, 15, 10}},
				isCumulative: []bool{false},
				isMonotonic:  []bool{true},
			}),
		},
		{
			name:    "cumulative_to_delta_exclude_sum_metrics",
			include: MatchMetrics{},
			exclude: MatchMetrics{
				MetricTypes: []string{"sum"},
			},
			inMetrics: generateMixedTestMetrics(
				testSumMetric{
					metricNames:  []string{"metric_1"},
					metricValues: [][]float64{{0, 100, 200, 500}},
					isCumulative: []bool{true, true},
					isMonotonic:  []bool{true, true},
				},
				testHistogramMetric{
					metricNames:  []string{"metric_2"},
					metricCounts: [][]uint64{{0, 100, 200, 500}},
					metricSums:   [][]float64{{0, 100, 200, 500}},
					metricBuckets: [][][]uint64{
						{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					},
					metricMins: [][]float64{
						{0, 5.0, 2.0, 3.0},
					},
					metricMaxes: [][]float64{
						{0, 800.0, 825.0, 800.0},
					},
					isCumulative: []bool{true},
				},
				testExpHistogramMetric{
					metricNames:           []string{"metric_3"},
					metricScales:          [][]int32{{0, 0, 0, 0}},
					metricPosIndexOffsets: [][]int32{{5, 5, 5, 5}},
					metricNegIndexOffsets: [][]int32{{-2, -2, -2, -2}},
					metricZeroThresholds:  [][]float64{{0, 0, 0, 0}},
					isCumulative:          []bool{true},

					metricCounts:     [][]uint64{{0, 180, 220, 283}},
					metricSums:       [][]float64{{0, 150, 180, 225}},
					metricMins:       [][]float64{{0, 0.1, 0.15, 0.2}},
					metricMaxes:      [][]float64{{0, 10.0, 12.0, 15.0}},
					metricZeroCounts: [][]uint64{{0, 5, 10, 20}},
					metricPosCounts: [][][]uint64{
						{{0, 0, 0}, {100, 50, 25}, {120, 60, 30}, {150, 75, 38}},
					},
					metricNegCounts: [][][]uint64{
						{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					},
				},
			),
			outMetrics: generateMixedTestMetrics(
				testSumMetric{
					metricNames:  []string{"metric_1"},
					metricValues: [][]float64{{0, 100, 200, 500}},
					isCumulative: []bool{true},
					isMonotonic:  []bool{true},
				},
				testHistogramMetric{
					metricNames:  []string{"metric_2"},
					metricCounts: [][]uint64{{100, 100, 300}},
					metricSums:   [][]float64{{100, 100, 300}},
					metricBuckets: [][][]uint64{
						{{50, 25, 25}, {50, 25, 25}, {150, 75, 75}},
					},
					metricMins: [][]float64{
						nil,
					},
					metricMaxes: [][]float64{
						nil,
					},
					isCumulative: []bool{false},
				},
				testExpHistogramMetric{
					metricNames:           []string{"metric_3"},
					metricScales:          [][]int32{{0, 0, 0}},
					metricPosIndexOffsets: [][]int32{{5, 5, 5}},
					metricNegIndexOffsets: [][]int32{{-2, -2, -2}},
					metricZeroThresholds:  [][]float64{{0, 0, 0}},
					isCumulative:          []bool{false},

					metricCounts:     [][]uint64{{180, 40, 63}},
					metricSums:       [][]float64{{150, 30, 45}},
					metricZeroCounts: [][]uint64{{5, 8, 10}},
					metricPosCounts: [][][]uint64{
						{{100, 50, 25}, {20, 10, 5}, {30, 15, 8}},
					},
					metricNegCounts: [][][]uint64{
						{{50, 25, 25}, {50, 25, 25}, {150, 75, 75}},
					},
				},
			),
		},
		{
			name: "cumulative_to_delta_include_histogram_metrics",
			include: MatchMetrics{
				MetricTypes: []string{"histogram"},
			},
			inMetrics: generateMixedTestMetrics(
				testSumMetric{
					metricNames:  []string{"metric_1"},
					metricValues: [][]float64{{0, 100, 200, 500}},
					isCumulative: []bool{true, true},
					isMonotonic:  []bool{true, true},
				},
				testHistogramMetric{
					metricNames:  []string{"metric_2"},
					metricCounts: [][]uint64{{0, 100, 200, 500}},
					metricSums:   [][]float64{{0, 100, 200, 500}},
					metricBuckets: [][][]uint64{
						{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					},
					metricMins: [][]float64{
						{0, 5.0, 2.0, 3.0},
					},
					metricMaxes: [][]float64{
						{0, 800.0, 825.0, 800.0},
					},
					isCumulative: []bool{true},
				},
				testExpHistogramMetric{
					metricNames:           []string{"metric_3"},
					metricScales:          [][]int32{{0, 0, 0, 0}},
					metricPosIndexOffsets: [][]int32{{5, 5, 5, 5}},
					metricNegIndexOffsets: [][]int32{{-2, -2, -2, -2}},
					metricZeroThresholds:  [][]float64{{0, 0, 0, 0}},
					isCumulative:          []bool{true},

					metricCounts:     [][]uint64{{0, 180, 220, 283}},
					metricSums:       [][]float64{{0, 150, 180, 225}},
					metricMins:       [][]float64{{0, 0.1, 0.15, 0.2}},
					metricMaxes:      [][]float64{{0, 10.0, 12.0, 15.0}},
					metricZeroCounts: [][]uint64{{0, 5, 10, 20}},
					metricPosCounts: [][][]uint64{
						{{0, 0, 0}, {100, 50, 25}, {120, 60, 30}, {150, 75, 38}},
					},
					metricNegCounts: [][][]uint64{
						{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					},
				},
			),
			outMetrics: generateMixedTestMetrics(
				testSumMetric{
					metricNames:  []string{"metric_1"},
					metricValues: [][]float64{{0, 100, 200, 500}},
					isCumulative: []bool{true},
					isMonotonic:  []bool{true},
				},
				testHistogramMetric{
					metricNames:  []string{"metric_2"},
					metricCounts: [][]uint64{{100, 100, 300}},
					metricSums:   [][]float64{{100, 100, 300}},
					metricBuckets: [][][]uint64{
						{{50, 25, 25}, {50, 25, 25}, {150, 75, 75}},
					},
					metricMins: [][]float64{
						nil,
					},
					metricMaxes: [][]float64{
						nil,
					},
					isCumulative: []bool{false},
				},
				testExpHistogramMetric{
					metricNames:           []string{"metric_3"},
					metricScales:          [][]int32{{0, 0, 0, 0}},
					metricPosIndexOffsets: [][]int32{{5, 5, 5, 5}},
					metricNegIndexOffsets: [][]int32{{-2, -2, -2, -2}},
					metricZeroThresholds:  [][]float64{{0, 0, 0, 0}},
					isCumulative:          []bool{true},

					metricCounts:     [][]uint64{{0, 180, 220, 283}},
					metricSums:       [][]float64{{0, 150, 180, 225}},
					metricMins:       [][]float64{{0, 0.1, 0.15, 0.2}},
					metricMaxes:      [][]float64{{0, 10.0, 12.0, 15.0}},
					metricZeroCounts: [][]uint64{{0, 5, 10, 20}},
					metricPosCounts: [][][]uint64{
						{{0, 0, 0}, {100, 50, 25}, {120, 60, 30}, {150, 75, 38}},
					},
					metricNegCounts: [][][]uint64{
						{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					},
				},
			),
		},
		{
			name: "cumulative_to_delta_include_exp_histogram_metrics",
			include: MatchMetrics{
				MetricTypes: []string{"exponentialhistogram"},
			},
			inMetrics: generateMixedTestMetrics(
				testSumMetric{
					metricNames:  []string{"metric_1"},
					metricValues: [][]float64{{0, 100, 200, 500}},
					isCumulative: []bool{true, true},
					isMonotonic:  []bool{true, true},
				},
				testHistogramMetric{
					metricNames:  []string{"metric_2"},
					metricCounts: [][]uint64{{0, 100, 200, 500}},
					metricSums:   [][]float64{{0, 100, 200, 500}},
					metricBuckets: [][][]uint64{
						{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					},
					metricMins: [][]float64{
						{0, 5.0, 2.0, 3.0},
					},
					metricMaxes: [][]float64{
						{0, 800.0, 825.0, 800.0},
					},
					isCumulative: []bool{true},
				},
				testExpHistogramMetric{
					metricNames:           []string{"metric_3"},
					metricScales:          [][]int32{{0, 0, 0, 0}},
					metricPosIndexOffsets: [][]int32{{5, 5, 5, 5}},
					metricNegIndexOffsets: [][]int32{{-2, -2, -2, -2}},
					metricZeroThresholds:  [][]float64{{0, 0, 0, 0}},
					isCumulative:          []bool{true},

					metricCounts:     [][]uint64{{0, 180, 220, 283}},
					metricSums:       [][]float64{{0, 150, 180, 225}},
					metricMins:       [][]float64{{0, 0.1, 0.15, 0.2}},
					metricMaxes:      [][]float64{{0, 10.0, 12.0, 15.0}},
					metricZeroCounts: [][]uint64{{0, 5, 10, 20}},
					metricPosCounts: [][][]uint64{
						{{0, 0, 0}, {100, 50, 25}, {120, 60, 30}, {150, 75, 38}},
					},
					metricNegCounts: [][][]uint64{
						{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					},
				},
			),
			outMetrics: generateMixedTestMetrics(
				testSumMetric{
					metricNames:  []string{"metric_1"},
					metricValues: [][]float64{{0, 100, 200, 500}},
					isCumulative: []bool{true},
					isMonotonic:  []bool{true},
				},
				testHistogramMetric{
					metricNames:  []string{"metric_2"},
					metricCounts: [][]uint64{{0, 100, 200, 500}},
					metricSums:   [][]float64{{0, 100, 200, 500}},
					metricBuckets: [][][]uint64{
						{{0, 0, 0}, {50, 25, 25}, {100, 50, 50}, {250, 125, 125}},
					},
					metricMins: [][]float64{
						{0, 5.0, 2.0, 3.0},
					},
					metricMaxes: [][]float64{
						{0, 800.0, 825.0, 800.0},
					},
					isCumulative: []bool{true},
				},
				testExpHistogramMetric{
					metricNames:           []string{"metric_3"},
					metricScales:          [][]int32{{0, 0, 0}},
					metricPosIndexOffsets: [][]int32{{5, 5, 5}},
					metricNegIndexOffsets: [][]int32{{-2, -2, -2}},
					metricZeroThresholds:  [][]float64{{0, 0, 0}},
					isCumulative:          []bool{false},

					metricCounts:     [][]uint64{{180, 40, 63}},
					metricSums:       [][]float64{{150, 30, 45}},
					metricZeroCounts: [][]uint64{{5, 8, 10}},
					metricPosCounts: [][][]uint64{
						{{100, 50, 25}, {20, 10, 5}, {30, 15, 8}},
					},
					metricNegCounts: [][][]uint64{
						{{50, 25, 25}, {50, 25, 25}, {150, 75, 75}},
					},
				},
			),
		},
		{
			name: "cumulative_to_delta_unsupported_include_metric_type",
			include: MatchMetrics{
				MetricTypes: []string{"summary"},
			},
			wantError: errors.New("unsupported metric type filter: summary"),
		},
		{
			name: "cumulative_to_delta_unsupported_exclude_metric_type",
			include: MatchMetrics{
				MetricTypes: []string{"summary"},
			},
			wantError: errors.New("unsupported metric type filter: summary"),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the filter metric processor
			next := new(consumertest.MetricsSink)
			cfg := &Config{
				Include: test.include,
				Exclude: test.exclude,
			}
			factory := NewFactory()
			mgp, err := factory.CreateMetrics(
				context.Background(),
				processortest.NewNopSettings(metadata.Type),
				cfg,
				next,
			)

			if test.wantError != nil {
				require.ErrorContains(t, err, test.wantError.Error())
				require.Nil(t, mgp)
				return
			}
			assert.NotNil(t, mgp)
			assert.NoError(t, err)
			caps := mgp.Capabilities()

			assert.True(t, caps.MutatesData)
			ctx := context.Background()
			require.NoError(t, mgp.Start(ctx, nil))

			cErr := mgp.ConsumeMetrics(context.Background(), test.inMetrics)
			assert.NoError(t, cErr)
			got := next.AllMetrics()

			require.Len(t, got, 1)
			require.Equal(t, test.outMetrics.ResourceMetrics().Len(), got[0].ResourceMetrics().Len())

			expectedMetrics := test.outMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			actualMetrics := got[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

			require.Equal(t, expectedMetrics.Len(), actualMetrics.Len())

			for i := 0; i < expectedMetrics.Len(); i++ {
				eM := expectedMetrics.At(i)
				aM := actualMetrics.At(i)

				require.Equal(t, eM.Name(), aM.Name())

				if eM.Type() == pmetric.MetricTypeGauge {
					eDataPoints := eM.Gauge().DataPoints()
					aDataPoints := aM.Gauge().DataPoints()
					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())

					for j := 0; j < eDataPoints.Len(); j++ {
						require.Equal(t, eDataPoints.At(j).DoubleValue(), aDataPoints.At(j).DoubleValue())
					}
				}

				if eM.Type() == pmetric.MetricTypeSum {
					eDataPoints := eM.Sum().DataPoints()
					aDataPoints := aM.Sum().DataPoints()

					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())
					require.Equal(t, eM.Sum().AggregationTemporality(), aM.Sum().AggregationTemporality())

					for j := 0; j < eDataPoints.Len(); j++ {
						if math.IsNaN(eDataPoints.At(j).DoubleValue()) {
							assert.True(t, math.IsNaN(aDataPoints.At(j).DoubleValue()))
						} else {
							require.Equal(t, eDataPoints.At(j).DoubleValue(), aDataPoints.At(j).DoubleValue())
						}
						require.Equal(t, eDataPoints.At(j).Flags(), aDataPoints.At(j).Flags())
					}
				}

				if eM.Type() == pmetric.MetricTypeHistogram {
					eDataPoints := eM.Histogram().DataPoints()
					aDataPoints := aM.Histogram().DataPoints()

					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())
					require.Equal(t, eM.Histogram().AggregationTemporality(), aM.Histogram().AggregationTemporality())

					for j := 0; j < eDataPoints.Len(); j++ {
						require.Equal(t, eDataPoints.At(j).Count(), aDataPoints.At(j).Count())
						require.Equal(t, eDataPoints.At(j).HasSum(), aDataPoints.At(j).HasSum())
						require.Equal(t, eDataPoints.At(j).HasMin(), aDataPoints.At(j).HasMin())
						require.Equal(t, eDataPoints.At(j).HasMax(), aDataPoints.At(j).HasMax())
						if math.IsNaN(eDataPoints.At(j).Sum()) {
							require.True(t, math.IsNaN(aDataPoints.At(j).Sum()))
						} else {
							require.Equal(t, eDataPoints.At(j).Sum(), aDataPoints.At(j).Sum())
						}
						require.Equal(t, eDataPoints.At(j).BucketCounts(), aDataPoints.At(j).BucketCounts())
						require.Equal(t, eDataPoints.At(j).Flags(), aDataPoints.At(j).Flags())
					}
				}

				if eM.Type() == pmetric.MetricTypeExponentialHistogram {
					eDataPoints := eM.ExponentialHistogram().DataPoints()
					aDataPoints := aM.ExponentialHistogram().DataPoints()

					require.Equal(t, eDataPoints.Len(), aDataPoints.Len(), "Number of datapoints does not match")
					require.Equal(t, eM.ExponentialHistogram().AggregationTemporality(), aM.ExponentialHistogram().AggregationTemporality(), "Temporality does not match")

					for j := 0; j < eDataPoints.Len(); j++ {
						require.Equal(t, eDataPoints.At(j).Count(), aDataPoints.At(j).Count(), "Count does not match at index %d", j)
						require.Equal(t, eDataPoints.At(j).HasSum(), aDataPoints.At(j).HasSum(), "HasSum does not match at index %d", j)
						require.Equal(t, eDataPoints.At(j).HasMin(), aDataPoints.At(j).HasMin(), "HasMin does not match at index %d", j)
						require.Equal(t, eDataPoints.At(j).HasMax(), aDataPoints.At(j).HasMax(), "HasMax does not match at index %d", j)
						if math.IsNaN(eDataPoints.At(j).Sum()) {
							require.True(t, math.IsNaN(aDataPoints.At(j).Sum()), "Expected sum to be NaN at index %d", j)
						} else {
							require.Equal(t, eDataPoints.At(j).Sum(), aDataPoints.At(j).Sum(), "Sum does not match at index %d", j)
						}
						require.Equal(t, eDataPoints.At(j).Positive().BucketCounts(), aDataPoints.At(j).Positive().BucketCounts(), "Positive bucket counts do not match at index %d", j)
						require.Equal(t, eDataPoints.At(j).Negative().BucketCounts(), aDataPoints.At(j).Negative().BucketCounts(), "Negative bucket counts do not match at index %d", j)
						require.Equal(t, eDataPoints.At(j).Flags(), aDataPoints.At(j).Flags(), "Flags do not match at index %d", j)
					}
				}
			}

			require.NoError(t, mgp.Shutdown(ctx))
		})
	}
}

func generateTestSumMetrics(tm testSumMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	tm.addToMetrics(ms, now)

	return md
}

func generateTestHistogramMetrics(tm testHistogramMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	tm.addToMetrics(ms, now)

	return md
}

func generateTestExpHistogramMetrics(tm testExpHistogramMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	tm.addToMetrics(ms, now)

	return md
}

func generateMixedTestMetrics(tsm testSumMetric, thm testHistogramMetric, tehm testExpHistogramMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()

	tsm.addToMetrics(ms, now)
	thm.addToMetrics(ms, now)
	tehm.addToMetrics(ms, now)

	return md
}

func BenchmarkConsumeMetrics(b *testing.B) {
	c := consumertest.NewNop()
	params := processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
		BuildInfo: component.BuildInfo{},
	}
	cfg := createDefaultConfig().(*Config)
	p, err := createMetricsProcessor(context.Background(), params, cfg, c)
	require.NoError(b, err)

	metrics := pmetric.NewMetrics()
	rms := metrics.ResourceMetrics().AppendEmpty()
	r := rms.Resource()
	r.Attributes().PutBool("resource", true)
	ilms := rms.ScopeMetrics().AppendEmpty()
	ilms.Scope().SetName("test")
	ilms.Scope().SetVersion("0.1")
	m := ilms.Metrics().AppendEmpty()
	m.SetEmptySum().SetIsMonotonic(true)
	dp := m.Sum().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("tag", "value")

	reset := func() {
		m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp.SetDoubleValue(100.0)
	}

	// Load initial value
	reset()
	assert.NoError(b, p.ConsumeMetrics(context.Background(), metrics))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reset()
		assert.NoError(b, p.ConsumeMetrics(context.Background(), metrics))
	}
}
