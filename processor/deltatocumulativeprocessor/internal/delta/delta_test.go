// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package delta_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
)

func TestErrOlderStart_Error(t *testing.T) {
	start := pcommon.Timestamp(200)
	sample := pcommon.Timestamp(100)
	err := delta.ErrOlderStart{
		Start:  start,
		Sample: sample,
	}
	expected := fmt.Sprintf(
		"dropped sample with start_time=%s, because series only starts at start_time=%s. consider checking for multiple processes sending the exact same series",
		sample.AsTime(),
		start.AsTime(),
	)
	assert.EqualError(t, err, expected)
}

func TestErrOutOfOrder_Error(t *testing.T) {
	last := pcommon.Timestamp(300)
	sample := pcommon.Timestamp(250)
	err := delta.ErrOutOfOrder{
		Last:   last,
		Sample: sample,
	}
	expected := fmt.Sprintf(
		"out of order: dropped sample from time=%s, because series is already at time=%s",
		sample.AsTime(),
		last.AsTime(),
	)
	assert.EqualError(t, err, expected)
}

func TestAggregate(t *testing.T) {
	t.Run("first sample copies state when state timestamp is zero", func(t *testing.T) {
		state := pmetric.NewNumberDataPoint()
		dp := pmetric.NewNumberDataPoint()
		dp.SetStartTimestamp(100)
		dp.SetTimestamp(200)
		dp.SetIntValue(42)

		aggregateCalled := false
		err := delta.Aggregate(state, dp, func(_, _ pmetric.NumberDataPoint) error {
			aggregateCalled = true
			return nil
		})

		require.NoError(t, err)
		assert.False(t, aggregateCalled)
		assert.Equal(t, pcommon.Timestamp(100), state.StartTimestamp())
		assert.Equal(t, pcommon.Timestamp(200), state.Timestamp())
		assert.Equal(t, int64(42), state.IntValue())
	})

	t.Run("returns ErrOlderStart when sample start timestamp is before state start timestamp", func(t *testing.T) {
		state := pmetric.NewNumberDataPoint()
		state.SetStartTimestamp(200)
		state.SetTimestamp(300)

		dp := pmetric.NewNumberDataPoint()
		dp.SetStartTimestamp(100)
		dp.SetTimestamp(400)

		err := delta.Aggregate(state, dp, func(_, _ pmetric.NumberDataPoint) error {
			t.Fatal("aggregate function should not have been called")
			return nil
		})

		var errOlderStart delta.ErrOlderStart
		require.ErrorAs(t, err, &errOlderStart)
		assert.Equal(t, pcommon.Timestamp(200), errOlderStart.Start)
		assert.Equal(t, pcommon.Timestamp(100), errOlderStart.Sample)
		assert.Equal(t, pcommon.Timestamp(300), state.Timestamp())
	})

	t.Run("returns ErrOutOfOrder when sample timestamp is less than state timestamp", func(t *testing.T) {
		state := pmetric.NewNumberDataPoint()
		state.SetStartTimestamp(100)
		state.SetTimestamp(300)

		dp := pmetric.NewNumberDataPoint()
		dp.SetStartTimestamp(100)
		dp.SetTimestamp(250)

		err := delta.Aggregate(state, dp, func(_, _ pmetric.NumberDataPoint) error {
			t.Fatal("aggregate function should not have been called")
			return nil
		})

		var errOutOfOrder delta.ErrOutOfOrder
		require.ErrorAs(t, err, &errOutOfOrder)
		assert.Equal(t, pcommon.Timestamp(300), errOutOfOrder.Last)
		assert.Equal(t, pcommon.Timestamp(250), errOutOfOrder.Sample)
		assert.Equal(t, pcommon.Timestamp(300), state.Timestamp())
	})

	t.Run("returns ErrOutOfOrder when sample timestamp is equal to state timestamp", func(t *testing.T) {
		state := pmetric.NewNumberDataPoint()
		state.SetStartTimestamp(100)
		state.SetTimestamp(300)

		dp := pmetric.NewNumberDataPoint()
		dp.SetStartTimestamp(100)
		dp.SetTimestamp(300)

		err := delta.Aggregate(state, dp, func(_, _ pmetric.NumberDataPoint) error {
			t.Fatal("aggregate function should not have been called")
			return nil
		})

		var errOutOfOrder delta.ErrOutOfOrder
		require.ErrorAs(t, err, &errOutOfOrder)
		assert.Equal(t, pcommon.Timestamp(300), errOutOfOrder.Last)
		assert.Equal(t, pcommon.Timestamp(300), errOutOfOrder.Sample)
		assert.Equal(t, pcommon.Timestamp(300), state.Timestamp())
	})

	t.Run("returns callback error without advancing state timestamp", func(t *testing.T) {
		state := pmetric.NewNumberDataPoint()
		state.SetStartTimestamp(100)
		state.SetTimestamp(200)

		dp := pmetric.NewNumberDataPoint()
		dp.SetStartTimestamp(100)
		dp.SetTimestamp(300)

		expectedErr := errors.New("aggregation failed")
		err := delta.Aggregate(state, dp, func(_, _ pmetric.NumberDataPoint) error {
			return expectedErr
		})

		require.ErrorIs(t, err, expectedErr)
		assert.Equal(t, pcommon.Timestamp(200), state.Timestamp())
	})

	t.Run("aggregates and updates state timestamp on success", func(t *testing.T) {
		state := pmetric.NewNumberDataPoint()
		state.SetStartTimestamp(100)
		state.SetTimestamp(200)
		state.SetIntValue(10)

		dp := pmetric.NewNumberDataPoint()
		dp.SetStartTimestamp(100)
		dp.SetTimestamp(300)
		dp.SetIntValue(15)

		err := delta.Aggregate(state, dp, func(s, d pmetric.NumberDataPoint) error {
			s.SetIntValue(s.IntValue() + d.IntValue())
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, pcommon.Timestamp(300), state.Timestamp())
		assert.Equal(t, int64(25), state.IntValue())
	})
}

func TestAggregator_Delegations(t *testing.T) {
	aggr := delta.Aggregator{
		Aggregator: data.Adder{},
	}

	t.Run("Numbers", func(t *testing.T) {
		state := pmetric.NewNumberDataPoint()
		dp := pmetric.NewNumberDataPoint()
		dp.SetStartTimestamp(100)
		dp.SetTimestamp(200)
		dp.SetIntValue(5)

		err := aggr.Numbers(state, dp)
		require.NoError(t, err)
		assert.Equal(t, pcommon.Timestamp(200), state.Timestamp())
		assert.Equal(t, int64(5), state.IntValue())

		dpNext := pmetric.NewNumberDataPoint()
		dpNext.SetStartTimestamp(100)
		dpNext.SetTimestamp(300)
		dpNext.SetIntValue(7)

		err = aggr.Numbers(state, dpNext)
		require.NoError(t, err)
		assert.Equal(t, pcommon.Timestamp(300), state.Timestamp())
		assert.Equal(t, int64(12), state.IntValue())
	})

	t.Run("Histograms", func(t *testing.T) {
		state := pmetric.NewHistogramDataPoint()
		dp := pmetric.NewHistogramDataPoint()
		dp.SetStartTimestamp(100)
		dp.SetTimestamp(200)
		dp.SetCount(10)
		dp.SetSum(100.0)

		err := aggr.Histograms(state, dp)
		require.NoError(t, err)
		assert.Equal(t, pcommon.Timestamp(200), state.Timestamp())
		assert.Equal(t, uint64(10), state.Count())
		assert.Equal(t, 100.0, state.Sum())

		dpNext := pmetric.NewHistogramDataPoint()
		dpNext.SetStartTimestamp(100)
		dpNext.SetTimestamp(300)
		dpNext.SetCount(5)
		dpNext.SetSum(50.0)

		err = aggr.Histograms(state, dpNext)
		require.NoError(t, err)
		assert.Equal(t, pcommon.Timestamp(300), state.Timestamp())
		assert.Equal(t, uint64(15), state.Count())
		assert.Equal(t, 150.0, state.Sum())
	})

	t.Run("Exponential", func(t *testing.T) {
		state := pmetric.NewExponentialHistogramDataPoint()
		dp := pmetric.NewExponentialHistogramDataPoint()
		dp.SetStartTimestamp(100)
		dp.SetTimestamp(200)
		dp.SetCount(20)
		dp.SetSum(200.0)

		err := aggr.Exponential(state, dp)
		require.NoError(t, err)
		assert.Equal(t, pcommon.Timestamp(200), state.Timestamp())
		assert.Equal(t, uint64(20), state.Count())
		assert.Equal(t, 200.0, state.Sum())

		dpNext := pmetric.NewExponentialHistogramDataPoint()
		dpNext.SetStartTimestamp(100)
		dpNext.SetTimestamp(300)
		dpNext.SetCount(10)
		dpNext.SetSum(100.0)

		err = aggr.Exponential(state, dpNext)
		require.NoError(t, err)
		assert.Equal(t, pcommon.Timestamp(300), state.Timestamp())
		assert.Equal(t, uint64(30), state.Count())
		assert.Equal(t, 300.0, state.Sum())
	})
}
