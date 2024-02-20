// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package delta // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

type Options struct {
	MaxStale time.Duration
}

func construct[D data.Point[D]](opts Options) streams.Aggregator[D] {
	stale := staleness.NewStaleness[D](opts.MaxStale, streams.EmptyMap[D]())
	acc := Accumulator[D]{
		dps: stale,
	}
	lock := Lock[D]{next: &acc}
	return &lock
}

func Numbers(opts Options) streams.Aggregator[data.Number] {
	return construct[data.Number](opts)
}

func Histograms(opts Options) streams.Aggregator[data.Histogram] {
	return construct[data.Histogram](opts)
}

var _ streams.Aggregator[data.Number] = (*Accumulator[data.Number])(nil)

type Accumulator[D data.Point[D]] struct {
	dps Tracker[D]
}

// Aggregate implements delta-to-cumulative aggregation as per spec:
// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#sums-delta-to-cumulative
func (a *Accumulator[D]) Aggregate(ctx context.Context, id streams.Ident, dp D) (D, error) {
	// make the accumulator start with the current sample, discarding any
	// earlier data. return after use
	reset := func() (D, error) {
		clone := dp.Clone()
		a.dps.Store(id, clone)
		return clone, nil
	}

	aggr, ok := a.dps.Load(id)

	// new series: reset
	if !ok {
		return reset()
	}
	// belongs to older series: drop
	if dp.StartTimestamp() < aggr.StartTimestamp() {
		return aggr, ErrOlderStart{Start: aggr.StartTimestamp(), Sample: dp.StartTimestamp()}
	}
	// belongs to later series: reset
	if dp.StartTimestamp() > aggr.StartTimestamp() {
		return reset()
	}
	// out of order: drop
	if dp.Timestamp() <= aggr.Timestamp() {
		return aggr, ErrOutOfOrder{Last: aggr.Timestamp(), Sample: dp.Timestamp()}
	}

	res := aggr.Add(dp)
	a.dps.Store(id, res)
	return res, nil
}

func (a *Accumulator[D]) Start(ctx context.Context) error {
	return a.dps.Start(ctx)
}

type ErrOlderStart struct {
	Start  pcommon.Timestamp
	Sample pcommon.Timestamp
}

func (e ErrOlderStart) Error() string {
	return fmt.Sprintf("dropped sample with start_time=%s, because series only starts at start_time=%s. consider checking for multiple processes sending the exact same series", e.Sample, e.Start)
}

type ErrOutOfOrder struct {
	Last   pcommon.Timestamp
	Sample pcommon.Timestamp
}

func (e ErrOutOfOrder) Error() string {
	return fmt.Sprintf("out of order: dropped sample from time=%s, because series is already at time=%s", e.Sample, e.Last)
}

type Tracker[T any] interface {
	streams.Map[T]
	Start(ctx context.Context) error
}
