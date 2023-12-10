package delta

import (
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var _ streams.Aggregator = (*Lock)(nil)

func NewLock(next streams.Aggregator) *Lock {
	return &Lock{next: next}
}

type Lock struct {
	mu   sync.Mutex
	next streams.Aggregator
}

func (l *Lock) Aggregate(id streams.Ident, dp pmetric.NumberDataPoint) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.next.Aggregate(id, dp)
}

func (l *Lock) Value(id streams.Ident) pmetric.NumberDataPoint {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.next.Value(id)
}
