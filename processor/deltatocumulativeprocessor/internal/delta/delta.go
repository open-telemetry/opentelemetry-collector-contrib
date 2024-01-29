package delta

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
	// make the accumulator to start with the current sample, discarding any
	// earlier data. return after use
	reset := func() (D, error) {
		a.dps[id] = dp.Clone()
		return a.dps[id], nil
	}

	aggr, ok := a.dps[id]
	if !ok {
		return reset()
	}

	// different start time violates single-writer principle, reset counter
	if dp.StartTimestamp() != aggr.StartTimestamp() {
		return reset()
	}

	if dp.Timestamp() <= aggr.Timestamp() {
		return dp, ErrOutOfOrder{Last: aggr.Timestamp(), Sample: aggr.Timestamp()}
	}

	a.dps[id] = aggr.Add(dp)
	return a.dps[id], nil
}

type ErrOutOfOrder struct {
	Last   pcommon.Timestamp
	Sample pcommon.Timestamp
}

func (e ErrOutOfOrder) Error() string {
	return fmt.Sprintf("out of order: sample at t=%s, but series already at t=%s", e.Sample, e.Last)
}
