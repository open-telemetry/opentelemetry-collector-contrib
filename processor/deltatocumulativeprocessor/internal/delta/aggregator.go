package delta

import (
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/identity"
)

type Aggregator interface {
	Aggregate(identity.Metric, pmetric.NumberDataPointSlice) error
	Value(identity.Metric) pmetric.NumberDataPointSlice
}

var _ Aggregator = (*Sum)(nil)

// Sum aggregrates all data points of same identity by addition
type Sum struct {
	mu      sync.Mutex
	metrics map[identity.Metric]*Metric
}

func NewSum() *Sum {
	return &Sum{}
}

func (s *Sum) Aggregate(id identity.Metric, dps pmetric.NumberDataPointSlice) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if dps.Len() == 0 {
		return nil
	}

	type delta struct {
		ty    ValueType
		float float64
		int   int64

		start time.Time
		last  time.Time
	}
	deltas := make(map[identity.Series]delta)

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		id := identity.OfSeries(dp.Attributes())
		delta := deltas[id]

		delta.ty = dp.ValueType()
		delta.float += dp.DoubleValue()
		delta.int += dp.IntValue()

		delta.start = dp.StartTimestamp().AsTime()
		ts := dp.Timestamp().AsTime()
		if ts.After(delta.last) {
			delta.last = ts
		}

		deltas[id] = delta
	}

	metric, ok := s.metrics[id]
	if !ok {
		metric = &Metric{series: make(map[identity.Series]*Value)}
		s.metrics[id] = metric
	}

	for id, delta := range deltas {
		aggr, ok := metric.series[id]
		if !ok {
			aggr = &Value{
				Type:  delta.ty,
				Start: delta.start,
			}
		}

		switch delta.ty {
		case TypeInt:
			aggr.Int += delta.int
		case TypeFloat:
			aggr.Float += delta.float
		}

		aggr.Last = delta.last
	}

	return nil
}

func (s *Sum) Value(id identity.Metric) pmetric.NumberDataPointSlice {
	s.mu.Lock()
	defer s.mu.Unlock()

	metric := s.metrics[id]

	dps := pmetric.NewNumberDataPointSlice()
	for sid, v := range metric.series {
		dp := dps.AppendEmpty()
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(v.Start))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(v.Last))
		switch v.Type {
		case TypeInt:
			dp.SetIntValue(v.Int)
		case TypeFloat:
			dp.SetDoubleValue(v.Float)
		}
		dp.Attributes().AsRaw()[metaSeriesKey] = sid
	}

	return dps
}

var _ Aggregator = (*Guard)(nil)

// Guard drops samples older than the last read
type Guard struct {
	mu sync.Mutex

	next  Aggregator
	reads map[identity.Metric]time.Time
}

func NewGuard(next Aggregator) *Guard {
	return &Guard{
		next:  next,
		reads: make(map[identity.Metric]time.Time),
	}
}

func (r *Guard) Aggregate(id identity.Metric, dps pmetric.NumberDataPointSlice) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
		ts := dp.Timestamp().AsTime()
		return ts.Before(r.reads[id])
	})

	return r.next.Aggregate(id, dps)
}

func (r *Guard) Value(id identity.Metric) pmetric.NumberDataPointSlice {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.reads[id] = time.Now()
	return r.next.Value(id)
}
