package delta

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var _ streams.Aggregator = (*Accumulator)(nil)

func NewAccumulator() *Accumulator {
	return &Accumulator{vals: make(map[streams.Ident]Value)}
}

type Accumulator struct {
	vals map[streams.Ident]Value
}

func (a *Accumulator) Aggregate(id streams.Ident, dp pmetric.NumberDataPoint) {
	v, ok := a.vals[id]
	if !ok {
		v.Start = dp.StartTimestamp()
		v.Type = dp.ValueType()
	}

	switch v.Type {
	case TypeFloat:
		v.Float += dp.DoubleValue()
	case TypeInt:
		v.Int += dp.IntValue()
	}

	a.vals[id] = v
}

func (a *Accumulator) Value(id streams.Ident) pmetric.NumberDataPoint {
	var dp pmetric.NumberDataPoint

	v, ok := a.vals[id]
	if !ok {
		return dp
	}

	dp.SetStartTimestamp(v.Start)
	dp.SetTimestamp(v.Last)
	switch v.Type {
	case TypeFloat:
		dp.SetDoubleValue(v.Float)
	case TypeInt:
		dp.SetIntValue(v.Int)
	}

	return dp
}
