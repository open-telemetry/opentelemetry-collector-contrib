// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serialization

import (
	"testing"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func Test_serializeHistogramPoint(t *testing.T) {
	hist := pmetric.NewHistogramDataPoint()
	hist.ExplicitBounds().FromRaw([]float64{0, 2, 4, 8})
	hist.BucketCounts().FromRaw([]uint64{0, 1, 0, 1, 0})
	hist.SetCount(2)
	hist.SetSum(9.5)
	hist.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

	t.Run("delta with prefix and dimension", func(t *testing.T) {
		got, err := serializeHistogramPoint("delta_hist", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), hist)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_hist,key=value gauge,min=0,max=8,sum=9.5,count=2 1626438600000", got)
	})

	t.Run("delta with non-empty first and last bucket", func(t *testing.T) {
		histWithNonEmptyFirstLast := pmetric.NewHistogramDataPoint()
		histWithNonEmptyFirstLast.ExplicitBounds().FromRaw([]float64{0, 2, 4, 8})
		histWithNonEmptyFirstLast.BucketCounts().FromRaw([]uint64{0, 1, 0, 1, 1})
		histWithNonEmptyFirstLast.SetCount(3)
		histWithNonEmptyFirstLast.SetSum(9.5)
		histWithNonEmptyFirstLast.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogramPoint("delta_nonempty_first_last_hist", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), histWithNonEmptyFirstLast)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_nonempty_first_last_hist,key=value gauge,min=0,max=8,sum=9.5,count=3 1626438600000", got)
	})

	t.Run("when average > highest boundary, max = average", func(t *testing.T) {
		// average = 15, highest boundary = 10
		histWitMaxGreaterAvg := pmetric.NewHistogramDataPoint()
		histWitMaxGreaterAvg.ExplicitBounds().FromRaw([]float64{0, 10})
		histWitMaxGreaterAvg.BucketCounts().FromRaw([]uint64{0, 0, 2})
		histWitMaxGreaterAvg.SetCount(2)
		histWitMaxGreaterAvg.SetSum(30)
		histWitMaxGreaterAvg.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogramPoint("delta_nonempty_first_last_hist", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), histWitMaxGreaterAvg)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_nonempty_first_last_hist,key=value gauge,min=10,max=15,sum=30,count=2 1626438600000", got)
	})

	t.Run("when average < lowest boundary, min = average", func(t *testing.T) {
		// average = 5, lowest boundary = 10
		histWitMinLessAvg := pmetric.NewHistogramDataPoint()
		histWitMinLessAvg.ExplicitBounds().FromRaw([]float64{10, 20})
		histWitMinLessAvg.BucketCounts().FromRaw([]uint64{2, 0, 0})
		histWitMinLessAvg.SetCount(2)
		histWitMinLessAvg.SetSum(10)
		histWitMinLessAvg.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogramPoint("delta_nonempty_first_last_hist", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), histWitMinLessAvg)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_nonempty_first_last_hist,key=value gauge,min=5,max=10,sum=10,count=2 1626438600000", got)
	})

	t.Run("when min is provided it should be used", func(t *testing.T) {
		minMaxHist := pmetric.NewHistogramDataPoint()
		minMaxHist.ExplicitBounds().FromRaw([]float64{10, 20})
		minMaxHist.BucketCounts().FromRaw([]uint64{2, 0, 0})
		minMaxHist.SetCount(2)
		minMaxHist.SetSum(10)
		minMaxHist.SetMin(3)
		minMaxHist.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogramPoint("min_max_hist", "prefix", dimensions.NewNormalizedDimensionList(), minMaxHist)
		assert.NoError(t, err)
		// min 3, max 10, sum 10 is impossible but passes consistency check because the estimated max 10 is greater than the mean 5
		// it is the best we can do without a better max estimate
		assert.Equal(t, "prefix.min_max_hist gauge,min=3,max=10,sum=10,count=2 1626438600000", got)
	})

	t.Run("when max is provided it should be used", func(t *testing.T) {
		minMaxHist := pmetric.NewHistogramDataPoint()
		minMaxHist.ExplicitBounds().FromRaw([]float64{10, 20})
		minMaxHist.BucketCounts().FromRaw([]uint64{2, 0, 0})
		minMaxHist.SetCount(2)
		minMaxHist.SetSum(10)
		minMaxHist.SetMax(7)
		minMaxHist.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogramPoint("min_max_hist", "prefix", dimensions.NewNormalizedDimensionList(), minMaxHist)
		assert.NoError(t, err)
		// min 5, max 7, sum 10 is impossible with count 2 but passes consistency check because the estimated min 10 is reduced to the mean 5
		// it is the best we can do without a better min estimate
		assert.Equal(t, "prefix.min_max_hist gauge,min=5,max=7,sum=10,count=2 1626438600000", got)
	})

	t.Run("when min and max is provided it should be used", func(t *testing.T) {
		minMaxHist := pmetric.NewHistogramDataPoint()
		minMaxHist.ExplicitBounds().FromRaw([]float64{10, 20})
		minMaxHist.BucketCounts().FromRaw([]uint64{2, 0, 0})
		minMaxHist.SetCount(2)
		minMaxHist.SetSum(10)
		minMaxHist.SetMin(3)
		minMaxHist.SetMax(7)
		minMaxHist.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeHistogramPoint("min_max_hist", "prefix", dimensions.NewNormalizedDimensionList(), minMaxHist)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.min_max_hist gauge,min=3,max=7,sum=10,count=2 1626438600000", got)
	})

	t.Run("when min is not provided it should be estimated", func(t *testing.T) {
		t.Run("values between first two boundaries", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{1, 2, 3, 4, 5})
			hist.BucketCounts().FromRaw([]uint64{0, 1, 0, 3, 2, 0})
			hist.SetCount(6)
			hist.SetSum(21.2)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 1.0, min, "use bucket min")
		})

		t.Run("first bucket has value", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{1, 2, 3, 4, 5})
			hist.BucketCounts().FromRaw([]uint64{1, 0, 0, 3, 0, 4})
			hist.SetCount(8)
			hist.SetSum(34.5)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 1.0, min, "use the first boundary as estimation instead of Inf")
		})

		t.Run("only the first bucket has values, use the mean", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{1, 2, 3, 4, 5})
			hist.BucketCounts().FromRaw([]uint64{3, 0, 0, 0, 0, 0})
			hist.SetCount(3)
			hist.SetSum(0.75)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 0.25, min)
		})
		t.Run("just one bucket from -Inf to Inf", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{})
			hist.BucketCounts().FromRaw([]uint64{4})
			hist.SetCount(4)
			hist.SetSum(8.8)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 2.2, min, "calculate the mean as min value")
		})
		t.Run("just one bucket from -Inf to Inf", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{})
			hist.BucketCounts().FromRaw([]uint64{1})
			hist.SetCount(1)
			hist.SetSum(1.2)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 1.2, min, "calculate the mean as min value")
		})
		t.Run("only the last bucket has a value", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{1, 2, 3, 4, 5})
			hist.BucketCounts().FromRaw([]uint64{0, 0, 0, 0, 0, 3})
			hist.SetCount(3)
			hist.SetSum(15.6)

			min, _, _ := histDataPointToSummary(hist)

			assert.Equal(t, 5.0, min, "use the lower bound")
		})
	})

	t.Run("when max is not provided it should be estimated", func(t *testing.T) {
		t.Run("values between the last two boundaries", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{1, 2, 3, 4, 5})
			hist.BucketCounts().FromRaw([]uint64{0, 1, 0, 3, 2, 0})
			hist.SetSum(21.2)
			hist.SetCount(6)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 5.0, max, "use bucket max")
		})

		t.Run("last bucket has value", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{1, 2, 3, 4, 5})
			hist.BucketCounts().FromRaw([]uint64{1, 0, 0, 3, 0, 4})
			hist.SetSum(34.5)
			hist.SetCount(8)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 5.0, max, "use the last boundary as estimation instead of Inf")
		})

		t.Run("only the last bucket has values", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{1, 2, 3, 4, 5})
			hist.BucketCounts().FromRaw([]uint64{0, 0, 0, 0, 0, 2})
			hist.SetSum(20.2)
			hist.SetCount(2)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 10.1, max, "use the mean (10.1) Otherwise, the max would be estimated as 5, and max >= avg would be violated")
		})

		t.Run("just one bucket from -Inf to Inf", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{})
			hist.BucketCounts().FromRaw([]uint64{4})
			hist.SetSum(8.8)
			hist.SetCount(4)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 2.2, max, "calculate the mean as max value")
		})

		t.Run("just one bucket from -Inf to Inf", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{})
			hist.BucketCounts().FromRaw([]uint64{1})
			hist.SetSum(1.2)
			hist.SetCount(1)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 1.2, max, "calculate the mean as max value")
		})

		t.Run("max is larger than sum", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{0, 5})
			hist.BucketCounts().FromRaw([]uint64{0, 2, 0})
			hist.SetSum(2.3)
			hist.SetCount(2)

			_, max, _ := histDataPointToSummary(hist)

			assert.Equal(t, 5.0, max, "use the estimated boundary")
		})
	})

	t.Run("when sum is not provided it should be estimated", func(t *testing.T) {
		t.Run("single bucket histogram", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{})
			hist.BucketCounts().FromRaw([]uint64{13})
			hist.SetCount(6)

			_, _, sum := histDataPointToSummary(hist)

			assert.Equal(t, 0.0, sum, "estimate zero (midpoint of [-Inf, Inf])")
		})

		t.Run("data in bounded buckets", func(t *testing.T) {
			hist := pmetric.NewHistogramDataPoint()
			hist.ExplicitBounds().FromRaw([]float64{1, 2, 3, 4, 5})
			hist.BucketCounts().FromRaw([]uint64{0, 3, 5, 0, 0, 0})
			hist.SetCount(6)

			_, _, sum := histDataPointToSummary(hist)

			assert.Equal(t, 3*1.5+5*2.5, sum, "estimate sum using bucket midpoints")
		})

		t.Run("data in unbounded buckets", func(t *testing.T) {
			t.Run("first bucket", func(t *testing.T) {
				hist := pmetric.NewHistogramDataPoint()
				hist.ExplicitBounds().FromRaw([]float64{1, 2, 3, 4, 5})
				hist.BucketCounts().FromRaw([]uint64{2, 3, 5, 0, 0, 0})
				hist.SetCount(6)

				_, _, sum := histDataPointToSummary(hist)

				assert.Equal(t, 1*2+3*1.5+5*2.5, sum, "use bucket upper bound")
			})

			t.Run("last bucket", func(t *testing.T) {
				hist := pmetric.NewHistogramDataPoint()
				hist.ExplicitBounds().FromRaw([]float64{1, 2, 3, 4, 5})
				hist.BucketCounts().FromRaw([]uint64{0, 3, 5, 0, 0, 2})
				hist.SetCount(6)

				_, _, sum := histDataPointToSummary(hist)

				assert.Equal(t, 3*1.5+5*2.5+2*5, sum, "use bucket upper bound")
			})
		})
	})
}

func Test_serializeHistogram(t *testing.T) {
	emptyDims := dimensions.NewNormalizedDimensionList()
	t.Run("wrong aggregation temporality", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("metric_name")
		hist := metric.SetEmptyHistogram()
		hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		zapCore, observedLogs := observer.New(zap.WarnLevel)
		logger := zap.New(zapCore)

		lines := serializeHistogram(logger, "", metric, emptyDims, emptyDims, []string{})
		assert.Empty(t, lines)

		actualLogRecords := makeSimplifiedLogRecordsFromObservedLogs(observedLogs)

		expectedLogRecords := []simplifiedLogRecord{
			{
				message: "dropping cumulative histogram",
				attributes: map[string]string{
					"name": "metric_name",
				},
			},
		}

		assert.ElementsMatch(t, actualLogRecords, expectedLogRecords)
	})

	t.Run("serialize returns error", func(t *testing.T) {
		// just testing one case to make sure the error reporting works,
		// the actual testing is done in Test_serializeHistogramPoint
		metric := pmetric.NewMetric()
		metric.SetName("metric_name")
		hist := metric.SetEmptyHistogram()
		hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := hist.DataPoints().AppendEmpty()
		dp.SetMin(10)
		dp.SetMax(3)
		dp.SetCount(1)
		dp.SetSum(30)

		zapCore, observedLogs := observer.New(zap.WarnLevel)
		logger := zap.New(zapCore)

		lines := serializeHistogram(logger, "", metric, emptyDims, emptyDims, []string{})
		assert.Empty(t, lines)

		expectedLogRecords := []simplifiedLogRecord{
			{
				message: "Error serializing histogram data point",
				attributes: map[string]string{
					"name":  "metric_name",
					"error": "min (10.000) cannot be greater than max (3.000)",
				},
			},
		}

		assert.ElementsMatch(t, makeSimplifiedLogRecordsFromObservedLogs(observedLogs), expectedLogRecords)
	})

	t.Run("histogram serialized as summary", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("metric_name")
		hist := metric.SetEmptyHistogram()
		hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := hist.DataPoints().AppendEmpty()
		dp.SetMin(1)
		dp.SetMax(5)
		dp.SetCount(3)
		dp.SetSum(8)

		zapCore, observedLogs := observer.New(zap.WarnLevel)
		logger := zap.New(zapCore)

		lines := serializeHistogram(logger, "", metric, emptyDims, emptyDims, []string{})

		expectedLines := []string{
			"metric_name gauge,min=1,max=5,sum=8,count=3",
		}

		assert.ElementsMatch(t, lines, expectedLines)

		assert.Empty(t, observedLogs.All())
	})
}
