// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracking

import (
	"context"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestMetricTracker_Convert(t *testing.T) {
	miSum := MetricIdentity{
		Resource:               pcommon.NewResource(),
		InstrumentationLibrary: pcommon.NewInstrumentationScope(),
		MetricType:             pmetric.MetricTypeSum,
		MetricIsMonotonic:      true,
		MetricName:             "",
		MetricUnit:             "",
		Attributes:             pcommon.NewMap(),
	}
	miIntSum := miSum
	miIntSum.MetricValueType = pmetric.NumberDataPointValueTypeInt
	miSum.MetricValueType = pmetric.NumberDataPointValueTypeDouble

	type subTest struct {
		name       string
		value      ValuePoint
		wantOut    DeltaValue
		noOut      bool
		wantReason string
	}

	future := time.Now().Add(1 * time.Hour)

	keepSubsequentTest := subTest{
		name: "keep subsequent value",
		value: ValuePoint{
			ObservedTimestamp: pcommon.NewTimestampFromTime(future.Add(time.Minute)),
			FloatValue:        225,
			IntValue:          225,
		},
		wantOut: DeltaValue{
			StartTimestamp: pcommon.NewTimestampFromTime(future),
			FloatValue:     125,
			IntValue:       125,
		},
	}

	tests := []struct {
		initValue       InitialValue
		metricStartTime pcommon.Timestamp
		tests           []subTest
	}{
		{
			initValue:       InitialValueKeep,
			metricStartTime: pcommon.NewTimestampFromTime(future.Add(-time.Minute)),
			tests: []subTest{
				{
					name: "keep initial value",
					value: ValuePoint{
						ObservedTimestamp: pcommon.NewTimestampFromTime(future),
						FloatValue:        100,
						IntValue:          100,
					},
					wantOut: DeltaValue{
						FloatValue: 100,
						IntValue:   100,
					},
				},
				keepSubsequentTest,
			},
		},
		{
			initValue: InitialValueDrop,
			tests: []subTest{
				{
					name: "drop initial value",
					value: ValuePoint{
						ObservedTimestamp: pcommon.NewTimestampFromTime(future),
						FloatValue:        100,
						IntValue:          100,
					},
					noOut:      true,
					wantReason: ReasonInitial,
				},
				keepSubsequentTest,
			},
		},
		{
			initValue: InitialValueAuto,
			tests: []subTest{
				{
					name: "drop on unset start time",
					value: ValuePoint{
						ObservedTimestamp: pcommon.NewTimestampFromTime(future),
						FloatValue:        100,
						IntValue:          100,
					},
					noOut:      true,
					wantReason: ReasonInitial,
				},
				keepSubsequentTest,
			},
		},
		{
			initValue:       InitialValueAuto,
			metricStartTime: pcommon.NewTimestampFromTime(future),
			tests: []subTest{
				{
					name: "drop on equal start and observed time",
					value: ValuePoint{
						ObservedTimestamp: pcommon.NewTimestampFromTime(future),
						FloatValue:        100,
						IntValue:          100,
					},
					noOut:      true,
					wantReason: ReasonInitial,
				},
				keepSubsequentTest,
			},
		},
		{
			initValue:       InitialValueAuto,
			metricStartTime: pcommon.NewTimestampFromTime(future),
			tests: []subTest{
				{
					name: "keep on observed after start",
					value: ValuePoint{
						ObservedTimestamp: pcommon.NewTimestampFromTime(future.Add(time.Minute)),
						FloatValue:        100.0,
						IntValue:          100,
					},
					wantOut: DeltaValue{
						StartTimestamp: pcommon.NewTimestampFromTime(future),
						FloatValue:     100.0,
						IntValue:       100,
					},
				},
				{
					name: "higher value converted",
					value: ValuePoint{
						ObservedTimestamp: pcommon.NewTimestampFromTime(future.Add(2 * time.Minute)),
						FloatValue:        225.0,
						IntValue:          225,
					},
					wantOut: DeltaValue{
						StartTimestamp: pcommon.NewTimestampFromTime(future.Add(time.Minute)),
						FloatValue:     125.0,
						IntValue:       125,
					},
				},
				{
					name: "lower value not converted - restart",
					value: ValuePoint{
						ObservedTimestamp: pcommon.NewTimestampFromTime(future.Add(3 * time.Minute)),
						FloatValue:        75.0,
						IntValue:          75,
					},
					noOut:      true,
					wantReason: ReasonReset,
				},
				{
					name: "Convert delta above previous not Converted Value",
					value: ValuePoint{
						ObservedTimestamp: pcommon.NewTimestampFromTime(future.Add(4 * time.Minute)),
						FloatValue:        300.0,
						IntValue:          300,
					},
					wantOut: DeltaValue{
						StartTimestamp: pcommon.NewTimestampFromTime(future.Add(3 * time.Minute)),
						FloatValue:     225.0,
						IntValue:       225,
					},
				},
				{
					name: "higher value converted - previous offset recorded",
					value: ValuePoint{
						ObservedTimestamp: pcommon.NewTimestampFromTime(future.Add(5 * time.Minute)),
						FloatValue:        325.0,
						IntValue:          325,
					},
					wantOut: DeltaValue{
						StartTimestamp: pcommon.NewTimestampFromTime(future.Add(4 * time.Minute)),
						FloatValue:     25.0,
						IntValue:       25,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.initValue.String(), func(t *testing.T) {
			m := NewMetricTracker(t.Context(), zap.NewNop(), 0, tt.initValue)

			miSum := miSum
			miSum.StartTimestamp = tt.metricStartTime
			miIntSum := miIntSum
			miIntSum.StartTimestamp = tt.metricStartTime

			for _, ttt := range tt.tests {
				t.Run(ttt.name, func(t *testing.T) {
					floatPoint := MetricPoint{
						Identity: miSum,
						Value:    ttt.value,
					}
					intPoint := MetricPoint{
						Identity: miIntSum,
						Value:    ttt.value,
					}

					gotOut, valid, reason := m.Convert(floatPoint)
					assert.Equal(t, ttt.wantReason, reason)
					if !ttt.noOut {
						require.True(t, valid)
						assert.Equal(t, ttt.wantOut.StartTimestamp, gotOut.StartTimestamp)
						assert.Equal(t, ttt.wantOut.FloatValue, gotOut.FloatValue)
					}

					gotOut, valid, reason = m.Convert(intPoint)
					assert.Equal(t, ttt.wantReason, reason)
					if !ttt.noOut {
						require.True(t, valid)
						assert.Equal(t, ttt.wantOut.StartTimestamp, gotOut.StartTimestamp)
						assert.Equal(t, ttt.wantOut.IntValue, gotOut.IntValue)
					}
				})
			}
		})
	}

	t.Run("Invalid metric identity", func(t *testing.T) {
		m := NewMetricTracker(t.Context(), zap.NewNop(), 0, InitialValueAuto)
		invalidID := miIntSum
		invalidID.MetricType = pmetric.MetricTypeGauge
		_, valid, reason := m.Convert(MetricPoint{
			Identity: invalidID,
			Value: ValuePoint{
				ObservedTimestamp: 0,
				FloatValue:        100.0,
				IntValue:          100,
			},
		})
		assert.False(t, valid, "Expected invalid for non cumulative metric")
		assert.Empty(t, reason, "Expected no reason for non cumulative metric")
	})

	t.Run("NaN float value", func(t *testing.T) {
		m := NewMetricTracker(t.Context(), zap.NewNop(), 0, InitialValueAuto)
		_, valid, reason := m.Convert(MetricPoint{
			Identity: miSum,
			Value: ValuePoint{
				ObservedTimestamp: pcommon.NewTimestampFromTime(future),
				FloatValue:        math.NaN(),
			},
		})
		assert.False(t, valid, "Expected invalid for NaN float value")
		assert.Empty(t, reason, "Expected no reason for NaN float value")
	})
}

func TestMetricTracker_Convert_ExponentialHistogramReset(t *testing.T) {
	miExpHist := MetricIdentity{
		Resource:               pcommon.NewResource(),
		InstrumentationLibrary: pcommon.NewInstrumentationScope(),
		MetricType:             pmetric.MetricTypeExponentialHistogram,
		MetricIsMonotonic:      true,
		Attributes:             pcommon.NewMap(),
	}

	expHist := func(count uint64) ValuePoint {
		return ValuePoint{
			ObservedTimestamp: pcommon.NewTimestampFromTime(time.Now()),
			ExponentialHistogramValue: &ExponentialHistogramPoint{
				Count:    count,
				Sum:      float64(count),
				Positive: ExponentialBuckets{Offset: 0, BucketCounts: []uint64{count}},
			},
		}
	}

	m := NewMetricTracker(t.Context(), zap.NewNop(), 0, InitialValueKeep)
	convert := func(count uint64) (DeltaValue, bool, string) {
		return m.Convert(MetricPoint{Identity: miExpHist, Value: expHist(count)})
	}

	// Baseline, then a normal increasing point.
	_, valid, _ := convert(100)
	require.True(t, valid)
	out, valid, reason := convert(150)
	require.True(t, valid)
	assert.Empty(t, reason)
	assert.EqualValues(t, 50, out.ExponentialHistogramPoint.Count)

	// Counter restart: cumulative count drops, so this point is a reset and is dropped.
	_, valid, reason = convert(20)
	assert.False(t, valid)
	assert.Equal(t, ReasonReset, reason)

	// The point after a reset must be converted against the restarted baseline (20),
	// exactly like the sum and plain-histogram paths recover on the very next point.
	out, valid, reason = convert(40)
	require.True(t, valid, "point after reset should convert, not be dropped as another reset")
	assert.Empty(t, reason)
	assert.EqualValues(t, 20, out.ExponentialHistogramPoint.Count)
}

func Test_metricTracker_removeStale(t *testing.T) {
	currentTime := pcommon.Timestamp(100)
	freshPoint := ValuePoint{
		ObservedTimestamp: currentTime,
	}
	stalePoint := ValuePoint{
		ObservedTimestamp: currentTime - 1,
	}

	type fields struct {
		MaxStaleness time.Duration
		States       map[string]*state
	}
	tests := []struct {
		name    string
		fields  fields
		wantOut map[string]*state
	}{
		{
			name: "Removes stale entry, leaves fresh entry",
			fields: fields{
				MaxStaleness: 0, // This logic isn't tested here
				States: map[string]*state{
					"stale": {
						prevPoint: stalePoint,
					},
					"fresh": {
						prevPoint: freshPoint,
					},
				},
			},
			wantOut: map[string]*state{
				"fresh": {
					prevPoint: freshPoint,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &MetricTracker{
				logger:       zap.NewNop(),
				maxStaleness: tt.fields.MaxStaleness,
			}
			for k, v := range tt.fields.States {
				tr.states.Store(k, v)
			}
			tr.removeStale(currentTime)

			gotOut := make(map[string]*state)
			tr.states.Range(func(key, value any) bool {
				gotOut[key.(string)] = value.(*state)
				return true
			})
			assert.Equal(t, tt.wantOut, gotOut)
		})
	}
}

func Test_metricTracker_sweeper(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	sweepEvent := make(chan pcommon.Timestamp)
	closed := &atomic.Bool{}

	onSweep := func(staleBefore pcommon.Timestamp) {
		sweepEvent <- staleBefore
	}

	tr := &MetricTracker{
		logger:       zap.NewNop(),
		maxStaleness: 1 * time.Millisecond,
	}

	start := time.Now()
	go func() {
		tr.sweeper(ctx, onSweep)
		closed.Store(true)
		close(sweepEvent)
	}()

	for i := 1; i <= 2; i++ {
		staleBefore := <-sweepEvent
		tickTime := time.Since(start) + tr.maxStaleness*time.Duration(i)
		require.False(t, closed.Load())
		assert.LessOrEqual(t, tr.maxStaleness, tickTime)
		assert.LessOrEqual(t, tr.maxStaleness, time.Since(staleBefore.AsTime()))
	}
	cancel()
	for range sweepEvent { //nolint:revive
	}
	assert.True(t, closed.Load(), "Sweeper did not terminate.")
}
