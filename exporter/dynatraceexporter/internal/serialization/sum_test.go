// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serialization

import (
	"math"
	"testing"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

func Test_serializeSumPoint(t *testing.T) {
	t.Run("without timestamp", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetIntValue(5)

		got, err := serializeSumPoint("int_sum", "prefix", dimensions.NewNormalizedDimensionList(), pmetric.AggregationTemporalityDelta, dp, ttlmap.New(1, 1))
		assert.NoError(t, err)
		assert.Equal(t, "prefix.int_sum count,delta=5", got)
	})

	t.Run("float delta with prefix and dimension", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetDoubleValue(5.5)
		dp.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		prev := ttlmap.New(1, 1)

		got, err := serializeSumPoint("double_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.AggregationTemporalityDelta, dp, prev)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.double_sum,key=value count,delta=5.5 1626438600000", got)
	})

	t.Run("int delta with prefix and dimension", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetIntValue(5)
		dp.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeSumPoint("int_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.AggregationTemporalityDelta, dp, ttlmap.New(1, 1))
		assert.NoError(t, err)
		assert.Equal(t, "prefix.int_sum,key=value count,delta=5 1626438600000", got)
	})

	t.Run("float cumulative with prefix and dimension", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetDoubleValue(5.5)
		dp.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		dp2 := pmetric.NewNumberDataPoint()
		dp2.SetDoubleValue(7.0)
		dp2.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 31, 0, 0, time.UTC).UnixNano()))

		prev := ttlmap.New(1, 1)

		got, err := serializeSumPoint("double_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.AggregationTemporalityCumulative, dp, prev)
		assert.NoError(t, err)
		assert.Equal(t, "", got)

		got, err = serializeSumPoint("double_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.AggregationTemporalityCumulative, dp2, prev)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.double_sum,key=value count,delta=1.5 1626438660000", got)
	})

	t.Run("int cumulative with prefix and dimension", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetIntValue(5)
		dp.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		dp2 := pmetric.NewNumberDataPoint()
		dp2.SetIntValue(10)
		dp2.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 31, 0, 0, time.UTC).UnixNano()))

		prev := ttlmap.New(1, 1)

		got, err := serializeSumPoint("int_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.AggregationTemporalityCumulative, dp, prev)
		assert.NoError(t, err)
		assert.Equal(t, "", got)

		got, err = serializeSumPoint("int_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.AggregationTemporalityCumulative, dp2, prev)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.int_sum,key=value count,delta=5 1626438660000", got)
	})

	t.Run("different dimensions should be treated as separate counters", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetIntValue(5)
		dp.Attributes().PutStr("sort", "unstable")
		dp.Attributes().PutStr("group", "a")
		dp.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		dp2 := pmetric.NewNumberDataPoint()
		dp2.SetIntValue(10)
		dp2.Attributes().PutStr("sort", "unstable")
		dp2.Attributes().PutStr("group", "b")
		dp2.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		dp3 := pmetric.NewNumberDataPoint()
		dp3.SetIntValue(10)
		dp3.Attributes().PutStr("group", "a")
		dp3.Attributes().PutStr("sort", "unstable")
		dp3.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		dp4 := pmetric.NewNumberDataPoint()
		dp4.SetIntValue(20)
		dp4.Attributes().PutStr("group", "b")
		dp4.Attributes().PutStr("sort", "unstable")
		dp4.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		prev := ttlmap.New(1, 1)

		got, err := serializeSumPoint("int_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "a")), pmetric.AggregationTemporalityCumulative, dp, prev)
		got2, err2 := serializeSumPoint("int_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "b")), pmetric.AggregationTemporalityCumulative, dp2, prev)
		got3, err3 := serializeSumPoint("int_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "a")), pmetric.AggregationTemporalityCumulative, dp3, prev)
		got4, err4 := serializeSumPoint("int_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "b")), pmetric.AggregationTemporalityCumulative, dp4, prev)

		assert.NoError(t, err)
		assert.NoError(t, err2)
		assert.NoError(t, err3)
		assert.NoError(t, err4)
		assert.Equal(t, "", got)
		assert.Equal(t, "", got2)
		assert.Equal(t, "prefix.int_sum,key=a count,delta=5 1626438600000", got3)
		assert.Equal(t, "prefix.int_sum,key=b count,delta=10 1626438600000", got4)
	})

	t.Run("count values older than the previous count value are dropped", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetIntValue(5)
		dp.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		dp2 := pmetric.NewNumberDataPoint()
		dp2.SetIntValue(5)
		dp2.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 29, 0, 0, time.UTC).UnixNano()))

		prev := ttlmap.New(1, 1)

		got, err := serializeSumPoint("int_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.AggregationTemporalityCumulative, dp, prev)
		assert.NoError(t, err)
		assert.Equal(t, "", got)

		assert.Equal(t, dp, prev.Get("int_sum"))

		got, err = serializeSumPoint("int_sum", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), pmetric.AggregationTemporalityCumulative, dp2, prev)
		assert.NoError(t, err)
		assert.Equal(t, "", got)

		assert.Equal(t, dp, prev.Get("int_sum"))
	})
}

func Test_serializeSum(t *testing.T) {
	empty := dimensions.NewNormalizedDimensionList()
	t.Run("non-monotonic delta is dropped", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("metric_name")
		sum := metric.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		sum.SetIsMonotonic(false)
		prev := ttlmap.New(10, 10)

		zapCore, observedLogs := observer.New(zap.WarnLevel)
		logger := zap.New(zapCore)

		lines := serializeSum(logger, "", metric, empty, empty, prev, []string{})

		assert.Empty(t, lines)

		expectedLogs := []simplifiedLogRecord{
			{
				message: "dropping delta non-monotonic sum",
				attributes: map[string]string{
					"name": "metric_name",
				},
			},
		}
		assert.ElementsMatch(t, makeSimplifiedLogRecordsFromObservedLogs(observedLogs), expectedLogs)
	})

	t.Run("monotonic delta", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("metric_name")
		sum := metric.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		sum.SetIsMonotonic(true)

		dp := sum.DataPoints().AppendEmpty()
		t.Run("with valid value is exported as delta", func(t *testing.T) {
			// not checking Double, this is done in Test_serializeSumPoint
			dp.SetIntValue(12)

			prev := ttlmap.New(10, 10)

			zapCore, observedLogs := observer.New(zap.WarnLevel)
			logger := zap.New(zapCore)

			actualLines := serializeSum(logger, "", metric, empty, empty, prev, []string{})

			expectedLines := []string{
				"metric_name count,delta=12",
			}
			assert.ElementsMatch(t, actualLines, expectedLines)
			assert.Empty(t, observedLogs.All())
		})

		t.Run("with invalid value logs warning and returns no line", func(t *testing.T) {
			dp.SetDoubleValue(math.NaN())

			prev := ttlmap.New(10, 10)

			zapCore, observedLogs := observer.New(zap.WarnLevel)
			logger := zap.New(zapCore)

			actualLines := serializeSum(logger, "", metric, empty, empty, prev, []string{})

			expectedLogRecords := []simplifiedLogRecord{
				{
					message: "Error serializing sum data point",
					attributes: map[string]string{
						"name":       "metric_name",
						"value-type": "Double",
						"error":      "value is NaN.",
					},
				},
			}

			assert.Empty(t, actualLines)
			assert.ElementsMatch(t, makeSimplifiedLogRecordsFromObservedLogs(observedLogs), expectedLogRecords)
		})

	})

	t.Run("non-monotonic cumulative", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("metric_name")
		sum := metric.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.SetIsMonotonic(false)
		dp := sum.DataPoints().AppendEmpty()

		t.Run("with valid value is exported as gauge", func(t *testing.T) {
			// not checking Int here, this is done in Test_serializeSumPoint
			dp.SetDoubleValue(12.3)

			prev := ttlmap.New(10, 10)

			zapCore, observedLogs := observer.New(zap.WarnLevel)
			logger := zap.New(zapCore)

			actualLines := serializeSum(logger, "", metric, empty, empty, prev, []string{})

			expectedLines := []string{
				"metric_name gauge,12.3",
			}

			assert.ElementsMatch(t, actualLines, expectedLines)
			// no logs / errors expected.
			assert.Empty(t, observedLogs.All())
		})

		t.Run("with invalid value logs warning and returns no line", func(t *testing.T) {
			dp.SetDoubleValue(math.NaN())

			prev := ttlmap.New(10, 10)

			zapCore, observedLogs := observer.New(zap.WarnLevel)
			logger := zap.New(zapCore)

			actualLines := serializeSum(logger, "", metric, empty, empty, prev, []string{})

			expectedLogRecords := []simplifiedLogRecord{
				{
					message: "Error serializing non-monotonic Sum as gauge",
					attributes: map[string]string{
						"name":       "metric_name",
						"value-type": "Double",
						"error":      "value is NaN.",
					},
				},
			}

			assert.Empty(t, actualLines)
			assert.ElementsMatch(t, makeSimplifiedLogRecordsFromObservedLogs(observedLogs), expectedLogRecords)
		})

		invalidDp := sum.DataPoints().AppendEmpty()
		invalidDp.SetDoubleValue(math.NaN())

	})

	t.Run("monotonic cumulative", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("metric_name")
		sum := metric.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.SetIsMonotonic(true)
		dp1 := sum.DataPoints().AppendEmpty()
		dp2 := sum.DataPoints().AppendEmpty()

		t.Run("with two valid data points is converted to delta", func(t *testing.T) {
			// not checking Int here, this is done in Test_serializeSumPoint
			dp1.SetDoubleValue(5.2)
			dp2.SetDoubleValue(5.7)

			prev := ttlmap.New(10, 10)

			zapCore, observedLogs := observer.New(zap.WarnLevel)
			logger := zap.New(zapCore)

			actualLines := serializeSum(logger, "", metric, empty, empty, prev, []string{})

			expectedLines := []string{
				"metric_name count,delta=0.5",
			}

			assert.ElementsMatch(t, actualLines, expectedLines)
			assert.Empty(t, observedLogs.All())

		})

		t.Run("with invalid value logs error and exports no line", func(t *testing.T) {
			dp1.SetDoubleValue(5.2)
			dp2.SetDoubleValue(math.NaN())

			prev := ttlmap.New(10, 10)

			zapCore, observedLogs := observer.New(zap.WarnLevel)
			logger := zap.New(zapCore)

			actualLines := serializeSum(logger, "", metric, empty, empty, prev, []string{})

			expectedLogRecords := []simplifiedLogRecord{
				{
					message: "Error serializing sum data point",
					attributes: map[string]string{
						"name":       "metric_name",
						"value-type": "Double",
						"error":      "value is NaN.",
					},
				},
			}

			assert.Empty(t, actualLines)
			assert.ElementsMatch(t, makeSimplifiedLogRecordsFromObservedLogs(observedLogs), expectedLogRecords)
		})

		t.Run("conversion with incompatible types returns an error", func(t *testing.T) {
			// double and int are incompatible
			dp1.SetDoubleValue(5.2)
			dp2.SetIntValue(5)

			prev := ttlmap.New(10, 10)

			zapCore, observedLogs := observer.New(zap.WarnLevel)
			logger := zap.New(zapCore)

			actualLines := serializeSum(logger, "", metric, empty, empty, prev, []string{})

			expectedLogRecords := []simplifiedLogRecord{
				{
					message: "Error serializing sum data point",
					attributes: map[string]string{
						"name":       "metric_name",
						"value-type": "Int",
						"error":      "expected metric_name to be type MetricValueTypeDouble but got MericValueTypeInt - count reset",
					},
				},
			}

			assert.Empty(t, actualLines)
			assert.ElementsMatch(t, makeSimplifiedLogRecordsFromObservedLogs(observedLogs), expectedLogRecords)
		})
	})
}

func Test_convertTotalCounterToDelta_notMutating(t *testing.T) {
	dp := pmetric.NewNumberDataPoint()
	dp.SetIntValue(5)
	dp.Attributes().PutStr("attr2", "val2")
	dp.Attributes().PutStr("attr1", "val1")
	orig := pmetric.NewNumberDataPoint()
	dp.CopyTo(orig)
	_, err := convertTotalCounterToDelta("m", "prefix", dimensions.NormalizedDimensionList{}, dp, ttlmap.New(1, 1))
	assert.NoError(t, err)
	assert.Equal(t, orig, dp) // make sure the original data point is not mutated
}
