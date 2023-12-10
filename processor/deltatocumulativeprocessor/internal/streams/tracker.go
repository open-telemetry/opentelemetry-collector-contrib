package streams

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
)

func NewTracker(aggr Aggregator) metrics.Aggregator {
	tracker := &Tracker{
		series: make(map[Ident]Meta),
		aggr:   aggr,
	}
	return metrics.NewLock(tracker)
}

type Tracker struct {
	series map[Ident]Meta
	aggr   Aggregator
}

func (t *Tracker) Consume(m metrics.Metric) {
	Samples(m, func(meta Meta, dp pmetric.NumberDataPoint) {
		id := meta.Identity()
		t.series[id] = meta
		t.aggr.Aggregate(id, dp)
	})
}

func (t *Tracker) Export() metrics.Map {
	var mm metrics.Map

	status := make(map[metrics.Ident]struct{})
	done := struct{}{}
	for id, meta := range t.series {
		if _, done := status[id.metric]; done {
			continue
		}

		res, sc, m := mm.For(id.metric)
		meta.metric.Resource().CopyTo(res)
		meta.metric.Scope().CopyTo(sc)
		meta.metric.CopyTo(m)

		status[id.metric] = done
	}

	for id, meta := range t.series {
		_, _, m := mm.For(id.metric)
		dp := m.Sum().DataPoints().AppendEmpty()
		meta.attrs.CopyTo(dp.Attributes())
		t.aggr.Value(id).CopyTo(dp)
	}

	return mm
}
