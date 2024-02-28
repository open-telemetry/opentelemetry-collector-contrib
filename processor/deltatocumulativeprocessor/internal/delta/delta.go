// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package delta // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

type Options struct {
	MaxStale time.Duration
}

func construct[D data.Point[D]](ctx context.Context, opts Options) streams.Aggregator[D] {
	dps := streams.EmptyMap[D]()
	if opts.MaxStale > 0 {
		dps = streams.ExpireAfter(ctx, dps, opts.MaxStale)
	}

	var (
		acc  = Accumulator[D]{dps: dps}
		lock = Lock[D]{next: &acc}
	)
	return &lock
}

func Numbers(ctx context.Context, opts Options) streams.Aggregator[data.Number] {
	return construct[data.Number](ctx, opts)
}

func Histograms(ctx context.Context, opts Options) streams.Aggregator[data.Histogram] {
	return construct[data.Histogram](ctx, opts)
}

var _ streams.Aggregator[data.Number] = (*Accumulator[data.Number])(nil)

type Accumulator[D data.Point[D]] struct {
	dps streams.Map[D]
}

// Aggregate implements delta-to-cumulative aggregation as per spec:
// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#sums-delta-to-cumulative
func (a *Accumulator[D]) Aggregate(_ context.Context, id streams.Ident, dp D) (D, error) {
	aggr, ok := a.dps.Load(id)

	// new series: initialize with current sample
	if !ok {
		clone := dp.Clone()
		a.dps.Store(id, clone)
		return clone, nil
	}

	// drop bad samples
	switch {
	case dp.StartTimestamp() < aggr.StartTimestamp():
		// belongs to older series
		return aggr, ErrOlderStart{Start: aggr.StartTimestamp(), Sample: dp.StartTimestamp()}
	case dp.Timestamp() <= aggr.Timestamp():
		// out of order
		return aggr, ErrOutOfOrder{Last: aggr.Timestamp(), Sample: dp.Timestamp()}
	}

	res := aggr.Add(dp)
	a.dps.Store(id, res)
	return res, nil
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
