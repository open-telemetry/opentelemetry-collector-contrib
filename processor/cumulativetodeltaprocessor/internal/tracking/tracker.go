// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracking // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Allocate a minimum of 64 bytes to the builder initially
const initialBytes = 64

type InitialValue int

const (
	InitialValueAuto InitialValue = iota
	InitialValueKeep
	InitialValueDrop
)

func (i *InitialValue) String() string {
	switch *i {
	case InitialValueAuto:
		return "auto"
	case InitialValueKeep:
		return "keep"
	case InitialValueDrop:
		return "drop"
	}
	return "unknown"
}

func (i *InitialValue) UnmarshalText(text []byte) error {
	switch string(text) {
	case "auto":
		*i = InitialValueAuto
	case "keep":
		*i = InitialValueKeep
	case "drop":
		*i = InitialValueDrop
	default:
		return fmt.Errorf("unknown initial_value: %s", text)
	}
	return nil
}

var identityBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, initialBytes))
	},
}

type State struct {
	sync.Mutex
	PrevPoint ValuePoint
}

type DeltaValue struct {
	StartTimestamp pcommon.Timestamp
	FloatValue     float64
	IntValue       int64
	HistogramValue *HistogramPoint
}

func NewMetricTracker(ctx context.Context, logger *zap.Logger, maxStaleness time.Duration, initalValue InitialValue) *MetricTracker {
	t := &MetricTracker{
		logger:       logger,
		maxStaleness: maxStaleness,
		initialValue: initalValue,
		startTime:    pcommon.NewTimestampFromTime(time.Now()),
	}
	if maxStaleness > 0 {
		go t.sweeper(ctx, t.removeStale)
	}
	return t
}

type MetricTracker struct {
	logger       *zap.Logger
	maxStaleness time.Duration
	states       sync.Map
	initialValue InitialValue
	startTime    pcommon.Timestamp
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

	s, ok := t.states.LoadOrStore(hashableID, &State{
		PrevPoint: metricPoint,
	})
	if !ok {
		switch metricID.MetricType {
		case pmetric.MetricTypeHistogram:
			val := metricPoint.HistogramValue.Clone()
			out.HistogramValue = &val
		case pmetric.MetricTypeSum:
			out.IntValue = metricPoint.IntValue
			out.FloatValue = metricPoint.FloatValue
		}
		switch t.initialValue {
		case InitialValueAuto:
			if metricID.StartTimestamp < t.startTime || metricPoint.ObservedTimestamp == metricID.StartTimestamp {
				return
			}
			out.StartTimestamp = metricID.StartTimestamp
			valid = true
		case InitialValueKeep:
			valid = true
		}
		return
	}

	valid = true

	state := s.(*State)
	state.Lock()
	defer state.Unlock()

	out.StartTimestamp = state.PrevPoint.ObservedTimestamp

	switch metricID.MetricType {
	case pmetric.MetricTypeHistogram:
		value := metricPoint.HistogramValue
		prevValue := state.PrevPoint.HistogramValue
		if math.IsNaN(value.Sum) {
			value.Sum = prevValue.Sum
		}

		if len(value.Buckets) != len(prevValue.Buckets) {
			valid = false
		}

		delta := value.Clone()

		// Calculate deltas unless histogram count was reset
		if valid && delta.Count >= prevValue.Count {
			delta.Count -= prevValue.Count
			delta.Sum -= prevValue.Sum
			for index, prevBucket := range prevValue.Buckets {
				delta.Buckets[index] -= prevBucket
			}
		}

		out.HistogramValue = &delta
	case pmetric.MetricTypeSum:
		if metricID.IsFloatVal() {
			value := metricPoint.FloatValue
			prevValue := state.PrevPoint.FloatValue
			delta := value - prevValue

			// Detect reset (non-monotonic sums are not converted)
			if value < prevValue {
				valid = false
			}

			out.FloatValue = delta
		} else {
			value := metricPoint.IntValue
			prevValue := state.PrevPoint.IntValue
			delta := value - prevValue

			// Detect reset (non-monotonic sums are not converted)
			if value < prevValue {
				valid = false
			}

			out.IntValue = delta
		}
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
