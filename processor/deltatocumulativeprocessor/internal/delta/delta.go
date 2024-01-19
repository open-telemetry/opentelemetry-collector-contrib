package delta

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func construct[D data.Point[D]]() streams.Aggregator[D] {
	acc := &Accumulator[D]{dps: make(map[streams.Ident]D)}
	lock := &Lock[D]{next: acc}
	return lock
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

func (a *Accumulator[D]) Aggregate(id streams.Ident, dp D) (D, error) {
	reset := func() {
		a.dps[id] = dp.Clone()
	}

	aggr, ok := a.dps[id]
	if !ok {
		fmt.Printf("d2c: new stream %s", id)
		reset()
		return dp, nil
	}

	if dp.StartTimestamp() != aggr.StartTimestamp() {
		reset()
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
