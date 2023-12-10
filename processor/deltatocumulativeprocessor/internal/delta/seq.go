package delta

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

var _ streams.Aggregator = (*Sequencer)(nil)

func NewSequencer(next streams.Aggregator) *Sequencer {
	return &Sequencer{
		next:  next,
		reads: make(map[streams.Ident]time.Time),
	}
}

type Sequencer struct {
	next  streams.Aggregator
	reads map[streams.Ident]time.Time
}

func (r *Sequencer) Aggregate(id streams.Ident, dp pmetric.NumberDataPoint) {
	if dp.Timestamp().AsTime().Before(r.reads[id]) {
		return
	}

	r.next.Aggregate(id, dp)
}

func (r *Sequencer) Value(id streams.Ident) pmetric.NumberDataPoint {
	r.reads[id] = time.Now()
	return r.next.Value(id)
}
