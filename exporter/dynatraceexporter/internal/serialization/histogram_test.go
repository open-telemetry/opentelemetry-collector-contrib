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

package serialization

import (
	"testing"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Test_serializeHistogram(t *testing.T) {
	hist := pmetric.NewHistogramDataPoint()
	hist.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{0, 2, 4, 8}))
	hist.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{0, 1, 0, 1, 0}))
	hist.SetCount(2)
	hist.SetSum(9.5)
	hist.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

	t.Run("delta with prefix and dimension", func(t *testing.T) {
		got, err := serializeHistogram("delta_hist", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.MetricAggregationTemporalityDelta, hist)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_hist,key=value gauge,min=0,max=8,sum=9.5,count=2 1626438600000", got)
	})

	t.Run("delta with non-empty first and last bucket", func(t *testing.T) {
		histWithNonEmptyFirstLast := pmetric.NewHistogramDataPoint()
		histWithNonEmptyFirstLast.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{0, 2, 4, 8}))
		histWithNonEmptyFirstLast.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{0, 1, 0, 1, 1}))
		histWithNonEmptyFirstLast.SetCount(3)
		histWithNonEmptyFirstLast.SetSum(9.5)
		histWithNonEmptyFirstLast.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogram("delta_nonempty_first_last_hist", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.MetricAggregationTemporalityDelta, histWithNonEmptyFirstLast)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_nonempty_first_last_hist,key=value gauge,min=0,max=8,sum=9.5,count=3 1626438600000", got)
	})

	t.Run("when average > highest boundary, max = average", func(t *testing.T) {
		// average = 15, highest boundary = 10
		histWitMaxGreaterAvg := pmetric.NewHistogramDataPoint()
		histWitMaxGreaterAvg.SetMExplicitBounds([]float64{0, 10})
		histWitMaxGreaterAvg.SetMBucketCounts([]uint64{0, 0, 2})
		histWitMaxGreaterAvg.SetCount(2)
		histWitMaxGreaterAvg.SetSum(30)
		histWitMaxGreaterAvg.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogram("delta_nonempty_first_last_hist", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.MetricAggregationTemporalityDelta, histWitMaxGreaterAvg)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_nonempty_first_last_hist,key=value gauge,min=10,max=15,sum=30,count=2 1626438600000", got)
	})

	t.Run("when average < lowest boundary, min = average", func(t *testing.T) {
		// average = 5, lowest boundary = 10
		histWitMinLessAvg := pmetric.NewHistogramDataPoint()
		histWitMinLessAvg.SetMExplicitBounds([]float64{10, 20})
		histWitMinLessAvg.SetMBucketCounts([]uint64{2, 0, 0})
		histWitMinLessAvg.SetCount(2)
		histWitMinLessAvg.SetSum(10)
		histWitMinLessAvg.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogram("delta_nonempty_first_last_hist", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.MetricAggregationTemporalityDelta, histWitMinLessAvg)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_nonempty_first_last_hist,key=value gauge,min=5,max=10,sum=10,count=2 1626438600000", got)
	})

	t.Run("cumulative with prefix and dimension", func(t *testing.T) {
		got, err := serializeHistogram("hist", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.MetricAggregationTemporalityCumulative, hist)
		assert.Error(t, err)
		assert.Equal(t, "", got)
	})

	t.Run("when min is provided it should be used", func(t *testing.T) {
		minMaxHist := pmetric.NewHistogramDataPoint()
		minMaxHist.SetMExplicitBounds([]float64{10, 20})
		minMaxHist.SetMBucketCounts([]uint64{2, 0, 0})
		minMaxHist.SetCount(2)
		minMaxHist.SetSum(10)
		minMaxHist.SetMin(3)
		minMaxHist.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogram("min_max_hist", "prefix", dimensions.NewNormalizedDimensionList(), pmetric.MetricAggregationTemporalityDelta, minMaxHist)
		assert.NoError(t, err)
		// min 3, max 10, sum 10 is impossible but passes consistency check because the estimated max 10 is greater than the mean 5
		// it is the best we can do without a better max estimate
		assert.Equal(t, "prefix.min_max_hist gauge,min=3,max=10,sum=10,count=2 1626438600000", got)
	})

	t.Run("when max is provided it should be used", func(t *testing.T) {
		minMaxHist := pmetric.NewHistogramDataPoint()
		minMaxHist.SetMExplicitBounds([]float64{10, 20})
		minMaxHist.SetMBucketCounts([]uint64{2, 0, 0})
		minMaxHist.SetCount(2)
		minMaxHist.SetSum(10)
		minMaxHist.SetMax(7)
		minMaxHist.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogram("min_max_hist", "prefix", dimensions.NewNormalizedDimensionList(), pmetric.MetricAggregationTemporalityDelta, minMaxHist)
		assert.NoError(t, err)
		// min 5, max 7, sum 10 is impossible with count 2 but passes consistency check because the estimated min 10 is reduced to the mean 5
		// it is the best we can do without a better min estimate
		assert.Equal(t, "prefix.min_max_hist gauge,min=5,max=7,sum=10,count=2 1626438600000", got)
	})

	t.Run("when min and max is provided it should be used", func(t *testing.T) {
		minMaxHist := pmetric.NewHistogramDataPoint()
		minMaxHist.SetMExplicitBounds([]float64{10, 20})
		minMaxHist.SetMBucketCounts([]uint64{2, 0, 0})
		minMaxHist.SetCount(2)
		minMaxHist.SetSum(10)
		minMaxHist.SetMin(3)
		minMaxHist.SetMax(7)
		minMaxHist.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogram("min_max_hist", "prefix", dimensions.NewNormalizedDimensionList(), pmetric.MetricAggregationTemporalityDelta, minMaxHist)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.min_max_hist gauge,min=3,max=7,sum=10,count=2 1626438600000", got)
	})

	t.Run("when min is not provided it should be estimated", func(t *testing.T) {
		t.Run("values between first two boundaries", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{1, 2, 3, 4, 5})
			hist.SetMBucketCounts([]uint64{0, 1, 0, 3, 2, 0})
			hist.SetCount(6)
			hist.SetSum(21.2)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 1.0, min, "use bucket min")
		})

		t.Run("first bucket has value", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{1, 2, 3, 4, 5})
			hist.SetMBucketCounts([]uint64{1, 0, 0, 3, 0, 4})
			hist.SetCount(8)
			hist.SetSum(34.5)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 1.0, min, "use the first boundary as estimation instead of Inf")
		})

		t.Run("only the first bucket has values, use the mean", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{1, 2, 3, 4, 5})
			hist.SetMBucketCounts([]uint64{3, 0, 0, 0, 0, 0})
			hist.SetCount(3)
			hist.SetSum(0.75)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 0.25, min)
		})
		t.Run("just one bucket from -Inf to Inf", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{})
			hist.SetMBucketCounts([]uint64{4})
			hist.SetCount(4)
			hist.SetSum(8.8)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 2.2, min, "calculate the mean as min value")
		})
		t.Run("just one bucket from -Inf to Inf", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{})
			hist.SetMBucketCounts([]uint64{1})
			hist.SetCount(1)
			hist.SetSum(1.2)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 1.2, min, "calculate the mean as min value")
		})
		t.Run("only the last bucket has a value", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{1, 2, 3, 4, 5})
			hist.SetMBucketCounts([]uint64{0, 0, 0, 0, 0, 3})
			hist.SetCount(3)
			hist.SetSum(15.6)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 5.0, min, "use the lower bound")
		})
	})

	t.Run("when max is not provided it should be estimated", func(t *testing.T) {
		t.Run("values between the last two boundaries", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{1, 2, 3, 4, 5})
			hist.SetMBucketCounts([]uint64{0, 1, 0, 3, 2, 0})
			hist.SetSum(21.2)
			hist.SetCount(6)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 5.0, max, "use bucket max")
		})

		t.Run("last bucket has value", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{1, 2, 3, 4, 5})
			hist.SetMBucketCounts([]uint64{1, 0, 0, 3, 0, 4})
			hist.SetSum(34.5)
			hist.SetCount(8)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 5.0, max, "use the last boundary as estimation instead of Inf")
		})

		t.Run("only the last bucket has values", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{1, 2, 3, 4, 5})
			hist.SetMBucketCounts([]uint64{0, 0, 0, 0, 0, 2})
			hist.SetSum(20.2)
			hist.SetCount(2)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 10.1, max, "use the mean (10.1) Otherwise, the max would be estimated as 5, and max >= avg would be violated")
		})

		t.Run("just one bucket from -Inf to Inf", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{})
			hist.SetMBucketCounts([]uint64{4})
			hist.SetSum(8.8)
			hist.SetCount(4)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 2.2, max, "calculate the mean as max value")
		})

		t.Run("just one bucket from -Inf to Inf", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{})
			hist.SetMBucketCounts([]uint64{1})
			hist.SetSum(1.2)
			hist.SetCount(1)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 1.2, max, "calculate the mean as max value")
		})

		t.Run("max is larger than sum", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{0, 5})
			hist.SetMBucketCounts([]uint64{0, 2, 0})
			hist.SetSum(2.3)
			hist.SetCount(2)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 5.0, max, "use the estimated boundary")
		})
	})

	t.Run("when sum is not provided it should be estimated", func(t *testing.T) {
		t.Run("single bucket histogram", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{})
			hist.SetMBucketCounts([]uint64{13})
			hist.SetCount(6)

			_, _, sum := histDataPointToSummary(hist)

			assert.Equal(t, 0.0, sum, "estimate zero (midpoint of [-Inf, Inf])")
		})

		t.Run("data in bounded buckets", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.SetMExplicitBounds([]float64{1, 2, 3, 4, 5})
			hist.SetMBucketCounts([]uint64{0, 3, 5, 0, 0, 0})
			hist.SetCount(6)

			_, _, sum := histDataPointToSummary(hist)

			assert.Equal(t, 3*1.5+5*2.5, sum, "estimate sum using bucket midpoints")
		})

		t.Run("data in unbounded buckets", func(t *testing.T) {
			t.Run("first bucket", func(t *testing.T) {
				hist := pmetric.NewHistogramDataPoint()
				hist.SetMExplicitBounds([]float64{1, 2, 3, 4, 5})
				hist.SetMBucketCounts([]uint64{2, 3, 5, 0, 0, 0})
				hist.SetCount(6)

				_, _, sum := histDataPointToSummary(hist)

				assert.Equal(t, 1*2+3*1.5+5*2.5, sum, "use bucket upper bound")
			})

			t.Run("last bucket", func(t *testing.T) {
				hist := pmetric.NewHistogramDataPoint()
				hist.SetMExplicitBounds([]float64{1, 2, 3, 4, 5})
				hist.SetMBucketCounts([]uint64{0, 3, 5, 0, 0, 2})
				hist.SetCount(6)

				_, _, sum := histDataPointToSummary(hist)

				assert.Equal(t, 3*1.5+5*2.5+2*5, sum, "use bucket upper bound")
			})
		})
	})
}
