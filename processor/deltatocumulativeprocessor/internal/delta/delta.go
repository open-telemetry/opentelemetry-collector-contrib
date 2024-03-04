// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package delta // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

func construct[D data.Point[D]]() streams.Aggregator[D] {
	acc := &Accumulator[D]{dps: make(map[streams.Ident]D)}
	return &Lock[D]{next: acc}
}

func Numbers() streams.Aggregator[data.Number] {
	return construct[data.Number]()
}

func Histograms() streams.Aggregator[data.Histogram] {
	return construct[data.Histogram]()
}

var _ streams.Aggregator[data.Number] = (*Accumulator[data.Number])(nil)

type Accumulator[D data.Point[D]] struct {
	dps map[streams.Ident]D
}

// Aggregate implements delta-to-cumulative aggregation as per spec:
// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#sums-delta-to-cumulative
func (a *Accumulator[D]) Aggregate(id streams.Ident, dp D) (D, error) {
	aggr, ok := a.dps[id]

	// new series: initialize with current sample
	if !ok {
		a.dps[id] = dp.Clone()
		return a.dps[id], nil
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

	a.dps[id] = aggr.Add(dp)
	return a.dps[id], nil
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
