// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	m := NewMetricTracker(context.Background(), zap.NewNop(), 0)

	tests := []struct {
		name    string
		value   ValuePoint
		wantOut DeltaValue
		noOut   bool
	}{
		{
			name: "Initial Value Omitted",
			value: ValuePoint{
				ObservedTimestamp: 10,
				FloatValue:        100.0,
				IntValue:          100,
			},
			noOut: true,
		},
		{
			name: "Higher Value Converted",
			value: ValuePoint{
				ObservedTimestamp: 50,
				FloatValue:        225.0,
				IntValue:          225,
			},
			wantOut: DeltaValue{
				StartTimestamp: 10,
				FloatValue:     125.0,
				IntValue:       125,
			},
		},
		{
			name: "Lower Value not Converted - Restart",
			value: ValuePoint{
				ObservedTimestamp: 100,
				FloatValue:        75.0,
				IntValue:          75,
			},
			noOut: true,
		},
		{
			name: "Convert delta above previous not Converted Value",
			value: ValuePoint{
				ObservedTimestamp: 150,
				FloatValue:        300.0,
				IntValue:          300,
			},
			wantOut: DeltaValue{
				StartTimestamp: 100,
				FloatValue:     225.0,
				IntValue:       225,
			},
		},
		{
			name: "Higher Value Converted - Previous Offset Recorded",
			value: ValuePoint{
				ObservedTimestamp: 200,
				FloatValue:        325.0,
				IntValue:          325,
			},
			wantOut: DeltaValue{
				StartTimestamp: 150,
				FloatValue:     25.0,
				IntValue:       25,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			floatPoint := MetricPoint{
				Identity: miSum,
				Value:    tt.value,
			}
			intPoint := MetricPoint{
				Identity: miIntSum,
				Value:    tt.value,
			}

			gotOut, valid := m.Convert(floatPoint)
			if !tt.noOut {
				require.True(t, valid)
				assert.Equal(t, tt.wantOut.StartTimestamp, gotOut.StartTimestamp)
				assert.Equal(t, tt.wantOut.FloatValue, gotOut.FloatValue)
			}

			gotOut, valid = m.Convert(intPoint)
			if !tt.noOut {
				require.True(t, valid)
				assert.Equal(t, tt.wantOut.StartTimestamp, gotOut.StartTimestamp)
				assert.Equal(t, tt.wantOut.IntValue, gotOut.IntValue)
			}
		})
	}

	t.Run("Invalid metric identity", func(t *testing.T) {
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
