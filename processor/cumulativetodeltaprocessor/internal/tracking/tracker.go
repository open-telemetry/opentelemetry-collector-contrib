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

package tracking // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"

import (
	"bytes"
	"context"
	"math"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

// Allocate a minimum of 64 bytes to the builder initially
const initialBytes = 64

var identityBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, initialBytes))
	},
}

type State struct {
	sync.Mutex
	Identity  MetricIdentity
	PrevPoint ValuePoint
}

type DeltaValue struct {
	StartTimestamp pcommon.Timestamp
	FloatValue     float64
	IntValue       int64
}

func NewMetricTracker(ctx context.Context, logger *zap.Logger, maxStaleness time.Duration) *MetricTracker {
	t := &MetricTracker{logger: logger, maxStaleness: maxStaleness}
	if maxStaleness > 0 {
		go t.sweeper(ctx, t.removeStale)
	}
	return t
}

type MetricTracker struct {
	logger       *zap.Logger
	maxStaleness time.Duration
	states       sync.Map
}

func (t *MetricTracker) Convert(in MetricPoint) (out DeltaValue, valid bool) {
	metricID := in.Identity
	metricPoint := in.Value
	if !metricID.IsSupportedMetricType() {
		return
	}

	// NaN is used to signal "stale" metrics.
	// These are ignored for now.
	// https://github.com/open-telemetry/opentelemetry-collector/pull/3423
	if metricID.IsFloatVal() && math.IsNaN(metricPoint.FloatValue) {
		return
	}

	b := identityBufferPool.Get().(*bytes.Buffer)
	b.Reset()
	metricID.Write(b)
	hashableID := b.String()
	identityBufferPool.Put(b)

	var s interface{}
	var ok bool
	if s, ok = t.states.Load(hashableID); !ok {
		s, ok = t.states.LoadOrStore(hashableID, &State{
			Identity:  metricID,
			PrevPoint: metricPoint,
		})
	}

	if !ok {
		if metricID.MetricIsMonotonic {
			out = DeltaValue{
				StartTimestamp: metricPoint.ObservedTimestamp,
				FloatValue:     metricPoint.FloatValue,
				IntValue:       metricPoint.IntValue,
			}
			valid = true
		}
		return
	}
	valid = true

	state := s.(*State)
	state.Lock()
	defer state.Unlock()

	out.StartTimestamp = state.PrevPoint.ObservedTimestamp

	if metricID.IsFloatVal() {
		value := metricPoint.FloatValue
		prevValue := state.PrevPoint.FloatValue
		delta := value - prevValue

		// Detect reset on a monotonic counter
		if metricID.MetricIsMonotonic && value < prevValue {
			delta = value
		}

		out.FloatValue = delta
	} else {
		value := metricPoint.IntValue
		prevValue := state.PrevPoint.IntValue
		delta := value - prevValue

		// Detect reset on a monotonic counter
		if metricID.MetricIsMonotonic && value < prevValue {
			delta = value
		}

		out.IntValue = delta
	}

	state.PrevPoint = metricPoint
	return
}

func (t *MetricTracker) removeStale(staleBefore pcommon.Timestamp) {
	t.states.Range(func(key, value interface{}) bool {
		s := value.(*State)

		// There is a known race condition here.
		// Because the state may be in the process of updating at the
		// same time as the stale removal, there is a chance that we
		// will remove a "stale" state that is in the process of
		// updating. This can only happen when datapoints arrive around
		// the expiration time.
		//
		// In this case, the possible outcomes are:
		//	* Updating goroutine wins, point will not be stale
		//	* Stale removal wins, updating goroutine will still see
		//	  the removed state but the state after the update will
		//	  not be persisted. The next update will load an entirely
		//	  new state.
		s.Lock()
		lastObserved := s.PrevPoint.ObservedTimestamp
		s.Unlock()
		if lastObserved < staleBefore {
			t.logger.Debug("removing stale state key", zap.String("key", key.(string)))
			t.states.Delete(key)
		}
		return true
	})
}

func (t *MetricTracker) sweeper(ctx context.Context, remove func(pcommon.Timestamp)) {
	ticker := time.NewTicker(t.maxStaleness)
	for {
		select {
		case currentTime := <-ticker.C:
			staleBefore := pcommon.NewTimestampFromTime(currentTime.Add(-t.maxStaleness))
			remove(staleBefore)
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}
