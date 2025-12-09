// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
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
	metricBounds  [][][]float64
	isCumulative  []bool
	flags         [][]pmetric.DataPointFlags
}

func (tm testHistogramMetric) addToMetrics(ms pmetric.MetricSlice, now time.Time) {
	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		hist := m.SetEmptyHistogram()

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
			if tm.metricBounds != nil {
				bounds := tm.metricBounds[i]
				if len(bounds) > 0 {
					dp.ExplicitBounds().FromRaw(bounds[index])
				}
			}
			dp.BucketCounts().FromRaw(tm.metricBuckets[i][index])
			if len(tm.flags) > i && len(tm.flags[i]) > index {
				dp.SetFlags(tm.flags[i][index])
			}
		}
	}
}

func ptr[T any](v T) *T {
	return &v
}

type testExponentialHistogramPoint struct {
	startTs         int64
	ts              int64
	count           uint64
	sum             *float64
	min             *float64
	max             *float64
	zeroCount       uint64
	zeroThreshold   float64
	scale           int32
	positiveOffset  int32
	positiveBuckets []uint64
	negativeOffset  int32
	negativeBuckets []uint64
}
type testExponentialHistogramMetric struct {
	name       string
	cumulative bool
	points     []testExponentialHistogramPoint
}
type testExponentialHistogramMetrics struct {
	metrics []testExponentialHistogramMetric
}

func (tm testExponentialHistogramMetrics) generateMetrics(now time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	for _, metric := range tm.metrics {
		m := ms.AppendEmpty()
		m.SetName(metric.name)
		eh := m.SetEmptyExponentialHistogram()
		if metric.cumulative {
			eh.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			eh.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		for i := range metric.points {
			point := &metric.points[i]
			dp := eh.DataPoints().AppendEmpty()
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Duration(point.startTs) * time.Second)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Duration(point.ts) * time.Second)))
			dp.SetCount(point.count)
			if point.sum != nil {
				dp.SetSum(*point.sum)
			}
			if point.min != nil {
				dp.SetMin(*point.min)
			}
			if point.max != nil {
				dp.SetMax(*point.max)
			}
			dp.SetZeroCount(point.zeroCount)
			dp.SetZeroThreshold(point.zeroThreshold)
			dp.SetScale(point.scale)
			dp.Positive().SetOffset(point.positiveOffset)
			dp.Positive().BucketCounts().FromRaw(point.positiveBuckets)
			dp.Negative().SetOffset(point.negativeOffset)
			dp.Negative().BucketCounts().FromRaw(point.negativeBuckets)
		}
	}
	return md
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
	now := time.Now()

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
					{2.0, 2.0, 2.0},
				},
				metricMaxes: [][]float64{
					{0, 800.0, 825.0, 800.0},
					{3.0, 3.0, 3.0},
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
					{2.0, 2.0, 2.0},
				},
				metricMaxes: [][]float64{
					nil,
					{3.0, 3.0, 3.0},
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
			name: "cumulative_to_delta_histogram_change_bounds",
			inMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1"},
				metricCounts: [][]uint64{{0, 100, 200, 500}},
				metricSums:   [][]float64{{0, 100, 200, 500}},
				metricBuckets: [][][]uint64{{
					{0, 0, 0},
					{50, 25, 25},
					{100, 50, 50},
					{250, 125, 125},
				}},
				metricBounds: [][][]float64{{
					{1.0, 2.0},
					{1.0, 2.0},
					{1.5, 3.0},
					{1.5, 3.0}, // Change of bucket bounds: first data point will be ignored
				}},
				isCumulative: []bool{true},
			}),
			outMetrics: generateTestHistogramMetrics(testHistogramMetric{
				metricNames:  []string{"metric_1"},
				metricCounts: [][]uint64{{100, 300}},
				metricSums:   [][]float64{{100, 300}},
				metricBuckets: [][][]uint64{
					{{50, 25, 25}, {150, 75, 75}},
				},
				isCumulative: []bool{false},
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
				}),
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
				}),
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
		{
			name: "cumulative_to_delta_exponential_histogram",
			inMetrics: testExponentialHistogramMetrics{
				metrics: []testExponentialHistogramMetric{
					{
						name:       "metric_1",
						cumulative: true,
						points: []testExponentialHistogramPoint{
							{
								// 10 points with value 1
								startTs:         1,
								ts:              2,
								count:           10,
								sum:             ptr(10.0),
								min:             ptr(1.0),
								max:             ptr(1.0),
								scale:           0, // base == 2
								positiveOffset:  -1,
								positiveBuckets: []uint64{10}, // ]0.5, 1]
							},
							{
								// additionally, 10 points with value 2, and 10 with value -4
								startTs:         1,
								ts:              3,
								count:           30,
								sum:             ptr(-10.0),
								min:             ptr(-4.0),
								max:             ptr(1.0),
								scale:           0,
								positiveOffset:  -1,
								positiveBuckets: []uint64{10, 10}, // ]0.5, 1], ]1, 2]
								negativeOffset:  2,
								negativeBuckets: []uint64{10}, // [-4, -2[
							},
							{
								// then, nothing
								startTs:         1,
								ts:              4,
								count:           30,
								sum:             ptr(-10.0),
								min:             ptr(-4.0),
								max:             ptr(1.0),
								scale:           0,
								positiveOffset:  -1,
								positiveBuckets: []uint64{10, 10}, // ]0.5, 1], ]1, 2]
								negativeOffset:  2,
								negativeBuckets: []uint64{10}, // [-4, -2[
							},
						},
					},
				},
			}.generateMetrics(now),
			outMetrics: testExponentialHistogramMetrics{
				metrics: []testExponentialHistogramMetric{
					{
						name:       "metric_1",
						cumulative: false,
						points: []testExponentialHistogramPoint{
							{
								startTs:         1,
								ts:              2,
								count:           10,
								sum:             ptr(10.0),
								scale:           0,
								positiveOffset:  -1,
								positiveBuckets: []uint64{10},
							},
							{
								startTs:         2,
								ts:              3,
								count:           20,
								sum:             ptr(-20.0),
								scale:           0,
								positiveOffset:  0,
								positiveBuckets: []uint64{10},
								negativeOffset:  2,
								negativeBuckets: []uint64{10},
							},
							{
								startTs: 3,
								ts:      4,
								sum:     ptr(0.0),
							},
						},
					},
				},
			}.generateMetrics(now),
		},
		{
			name: "cumulative_to_delta_exponential_histogram_coarsen",
			inMetrics: testExponentialHistogramMetrics{
				metrics: []testExponentialHistogramMetric{
					{
						name:       "metric_1",
						cumulative: true,
						points: []testExponentialHistogramPoint{
							{
								// 10 points with value 1 and 5 points with value 2
								startTs:         1,
								ts:              2,
								count:           15,
								sum:             ptr(20.0),
								min:             ptr(1.0),
								max:             ptr(2.0),
								scale:           0, // base == 2
								positiveOffset:  -1,
								positiveBuckets: []uint64{10, 5}, // ]0.5, 1], ]1, 2]
							},
							{
								// additionally, 10 points with value 0.5, 10 points with value 2, and 10 with value -4
								// zeroThreshold is increased to 1, so the 1 points are now counted as zeros
								// scale is decreased to -1
								startTs:         1,
								ts:              3,
								count:           45,
								sum:             ptr(5.0),
								min:             ptr(-4.0),
								max:             ptr(2.0),
								zeroThreshold:   1.0,
								zeroCount:       20,
								scale:           -1,
								positiveOffset:  0,
								positiveBuckets: []uint64{15}, // ]1, 4]
								negativeOffset:  0,
								negativeBuckets: []uint64{10}, // [-4, -1[
							},
						},
					},
				},
			}.generateMetrics(now),
			outMetrics: testExponentialHistogramMetrics{
				metrics: []testExponentialHistogramMetric{
					{
						name:       "metric_1",
						cumulative: false,
						points: []testExponentialHistogramPoint{
							{
								startTs:         1,
								ts:              2,
								count:           15,
								sum:             ptr(20.0),
								scale:           0,
								positiveOffset:  -1,
								positiveBuckets: []uint64{10, 5},
							},
							{
								startTs:         2,
								ts:              3,
								count:           30,
								sum:             ptr(-15.0),
								zeroThreshold:   1.0,
								zeroCount:       10,
								scale:           -1,
								positiveOffset:  0,
								positiveBuckets: []uint64{10},
								negativeOffset:  0,
								negativeBuckets: []uint64{10},
							},
						},
					},
				},
			}.generateMetrics(now),
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
				t.Context(),
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
			ctx := t.Context()
			require.NoError(t, mgp.Start(ctx, componenttest.NewNopHost()))

			cErr := mgp.ConsumeMetrics(t.Context(), test.inMetrics)
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
					require.NoError(t, pmetrictest.CompareMetric(eM, aM))
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

func generateMixedTestMetrics(tsm testSumMetric, thm testHistogramMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()

	tsm.addToMetrics(ms, now)
	thm.addToMetrics(ms, now)

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
	p, err := createMetricsProcessor(b.Context(), params, cfg, c)
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
	assert.NoError(b, p.ConsumeMetrics(b.Context(), metrics))

	for b.Loop() {
		reset()
		assert.NoError(b, p.ConsumeMetrics(b.Context(), metrics))
	}
}
