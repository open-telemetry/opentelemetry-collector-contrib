// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package delta // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	exp "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

func New[D data.Point[D]]() Accumulator[D] {
	return Accumulator[D]{
		Map: make(exp.HashMap[D]),
	}
}

var _ streams.Map[data.Number] = (*Accumulator[data.Number])(nil)

type Accumulator[D data.Point[D]] struct {
	streams.Map[D]
}

func (a Accumulator[D]) Store(id streams.Ident, dp D) error {
	aggr, ok := a.Map.Load(id)

	// new series: initialize with current sample
	if !ok {
		clone := dp.Clone()
		return a.Map.Store(id, clone)
	}

	// drop bad samples
	switch {
	case dp.StartTimestamp() < aggr.StartTimestamp():
		// belongs to older series
		return ErrOlderStart{Start: aggr.StartTimestamp(), Sample: dp.StartTimestamp()}
	case dp.Timestamp() <= aggr.Timestamp():
		// out of order
		return ErrOutOfOrder{Last: aggr.Timestamp(), Sample: dp.Timestamp()}
	}

	res := aggr.Add(dp)
	return a.Map.Store(id, res)
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
