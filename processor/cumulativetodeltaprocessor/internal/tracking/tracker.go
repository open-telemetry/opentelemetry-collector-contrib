// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracking // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"slices"
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
	New: func() any {
		return bytes.NewBuffer(make([]byte, initialBytes))
	},
}

type state struct {
	sync.Mutex
	prevPoint ValuePoint
}

type DeltaValue struct {
	StartTimestamp            pcommon.Timestamp
	FloatValue                float64
	IntValue                  int64
	HistogramValue            *HistogramPoint
	ExponentialHistogramPoint *ExponentialHistogramPoint
}

func NewMetricTracker(ctx context.Context, logger *zap.Logger, maxStaleness time.Duration, initialValue InitialValue) *MetricTracker {
	t := &MetricTracker{
		logger:       logger,
		maxStaleness: maxStaleness,
		initialValue: initialValue,
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
		return out, valid
	}

	// NaN is used to signal "stale" metrics.
	// These are ignored for now.
	// https://github.com/open-telemetry/opentelemetry-collector/pull/3423
	if metricID.IsFloatVal() && math.IsNaN(metricPoint.FloatValue) {
		return out, valid
	}

	b := identityBufferPool.Get().(*bytes.Buffer)
	b.Reset()
	metricID.Write(b)
	hashableID := b.String()
	identityBufferPool.Put(b)

	s, ok := t.states.LoadOrStore(hashableID, &state{
		prevPoint: metricPoint,
	})
	if !ok {
		switch metricID.MetricType {
		case pmetric.MetricTypeHistogram:
			val := metricPoint.HistogramValue.Clone()
			out.HistogramValue = &val
		case pmetric.MetricTypeExponentialHistogram:
			val := metricPoint.ExponentialHistogramValue.Clone()
			out.ExponentialHistogramPoint = &val
		case pmetric.MetricTypeSum:
			out.IntValue = metricPoint.IntValue
			out.FloatValue = metricPoint.FloatValue
		case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge, pmetric.MetricTypeSummary:
		}
		switch t.initialValue {
		case InitialValueAuto:
			if metricID.StartTimestamp < t.startTime || metricPoint.ObservedTimestamp == metricID.StartTimestamp {
				return out, valid
			}
			out.StartTimestamp = metricID.StartTimestamp
			valid = true
		case InitialValueKeep:
			valid = true
		case InitialValueDrop:
		}
		return out, valid
	}

	valid = true

	state := s.(*state)
	state.Lock()
	defer state.Unlock()

	out.StartTimestamp = state.prevPoint.ObservedTimestamp

	switch metricID.MetricType {
	case pmetric.MetricTypeHistogram:
		value := metricPoint.HistogramValue
		prevValue := state.prevPoint.HistogramValue
		if math.IsNaN(value.Sum) {
			value.Sum = prevValue.Sum
		}

		if len(value.BucketCounts) != len(prevValue.BucketCounts) {
			valid = false
			t.logger.Warn("Points in histogram series have different numbers of buckets; some data will be dropped")
		} else if !slices.Equal(value.BucketBounds, prevValue.BucketBounds) {
			valid = false
			t.logger.Warn("Points in histogram series have different bucket boundaries; some data will be dropped")
		}

		delta := value.Clone()

		// Calculate deltas unless histogram count was reset
		if valid && delta.Count >= prevValue.Count {
			delta.Count -= prevValue.Count
			delta.Sum -= prevValue.Sum
			for index, prevBucket := range prevValue.BucketCounts {
				delta.BucketCounts[index] -= prevBucket
			}
		}

		out.HistogramValue = &delta

	case pmetric.MetricTypeExponentialHistogram:
		value := metricPoint.ExponentialHistogramValue
		prevValue := state.prevPoint.ExponentialHistogramValue

		// Count and ZeroThreshold should only increase when merging, and Scale should only decrease.
		if value.Count < prevValue.Count || value.ZeroThreshold < prevValue.ZeroThreshold || value.Scale > prevValue.Scale {
			return out, false
		}

		delta := value.Clone()
		delta.Count -= prevValue.Count
		delta.Sum -= prevValue.Sum

		if value.ZeroThreshold > prevValue.ZeroThreshold {
			// Coarsen previous histogram zero bucket to match the new histogram.
			// Find the bucket the threshold falls into, i.e.
			// the greatest i such that 2**(2**(-scale) * index) < threshold
			scaleFactor := math.Ldexp(math.Log2E, int(prevValue.Scale))
			thresholdBucket := int32(math.Ceil(math.Log(value.ZeroThreshold)*scaleFactor) - 1)
			// If the bucket the threshold falls into is populated in the old histogram,
			// then it must also be populated in the new histogram, which has ill-defined semantics,
			// so we will assume this doesn't happen instead of adjusting the threshold as recommended in the spec.
			prevValue.ZeroCount += prevValue.Positive.TrimZeros(thresholdBucket)
			prevValue.ZeroCount += prevValue.Negative.TrimZeros(thresholdBucket)
		}
		delta.ZeroCount -= prevValue.ZeroCount

		if value.Scale < prevValue.Scale {
			// Coarsen previous histogram buckets to match the new histogram.
			bitsLost := prevValue.Scale - value.Scale
			prevValue.Scale = value.Scale
			prevValue.Positive = prevValue.Positive.Coarsen(bitsLost)
			prevValue.Negative = prevValue.Negative.Coarsen(bitsLost)
		}
		delta.Positive = value.Positive.Diff(&prevValue.Positive)
		delta.Negative = value.Negative.Diff(&prevValue.Negative)

		out.ExponentialHistogramPoint = &delta

	case pmetric.MetricTypeSum:
		if metricID.IsFloatVal() {
			value := metricPoint.FloatValue
			prevValue := state.prevPoint.FloatValue
			delta := value - prevValue

			// Detect reset (non-monotonic sums are not converted)
			if value < prevValue {
				valid = false
			}

			out.FloatValue = delta
		} else {
			value := metricPoint.IntValue
			prevValue := state.prevPoint.IntValue
			delta := value - prevValue

			// Detect reset (non-monotonic sums are not converted)
			if value < prevValue {
				valid = false
			}

			out.IntValue = delta
		}
	case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge, pmetric.MetricTypeSummary:
	}

	state.prevPoint = metricPoint
	return out, valid
}

func (t *MetricTracker) removeStale(staleBefore pcommon.Timestamp) {
	t.states.Range(func(key, value any) bool {
		s := value.(*state)

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
		lastObserved := s.prevPoint.ObservedTimestamp
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
