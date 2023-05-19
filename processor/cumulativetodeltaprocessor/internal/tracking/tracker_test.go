// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracking

import (
	"context"
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
		name    string
		value   ValuePoint
		wantOut DeltaValue
		noOut   bool
	}

	future := time.Now().Add(1 * time.Hour)

	keepSubsequentTest := subTest{
		name: "keep subsequet value",
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
					noOut: true,
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
					noOut: true,
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
					noOut: true,
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
					noOut: true,
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
			m := NewMetricTracker(context.Background(), zap.NewNop(), 0, tt.initValue)

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

					gotOut, valid := m.Convert(floatPoint)
					if !ttt.noOut {
						require.True(t, valid)
						assert.Equal(t, ttt.wantOut.StartTimestamp, gotOut.StartTimestamp)
						assert.Equal(t, ttt.wantOut.FloatValue, gotOut.FloatValue)
					}

					gotOut, valid = m.Convert(intPoint)
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
		m := NewMetricTracker(context.Background(), zap.NewNop(), 0, InitialValueAuto)
		invalidID := miIntSum
		invalidID.MetricType = pmetric.MetricTypeGauge
		_, valid := m.Convert(MetricPoint{
			Identity: invalidID,
			Value: ValuePoint{
				ObservedTimestamp: 0,
				FloatValue:        100.0,
				IntValue:          100,
			},
		})
		if valid {
			t.Error("Expected invalid for non cumulative metric")
		}
	})
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
		States       map[string]*State
	}
	tests := []struct {
		name    string
		fields  fields
		wantOut map[string]*State
	}{
		{
			name: "Removes stale entry, leaves fresh entry",
			fields: fields{
				MaxStaleness: 0, // This logic isn't tested here
				States: map[string]*State{
					"stale": {
						PrevPoint: stalePoint,
					},
					"fresh": {
						PrevPoint: freshPoint,
					},
				},
			},
			wantOut: map[string]*State{
				"fresh": {
					PrevPoint: freshPoint,
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

			gotOut := make(map[string]*State)
			tr.states.Range(func(key, value interface{}) bool {
				gotOut[key.(string)] = value.(*State)
				return true
			})
			assert.Equal(t, tt.wantOut, gotOut)
		})
	}
}

func Test_metricTracker_sweeper(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
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
	for range sweepEvent {
	}
	if !closed.Load() {
		t.Errorf("Sweeper did not terminate.")
	}
}
