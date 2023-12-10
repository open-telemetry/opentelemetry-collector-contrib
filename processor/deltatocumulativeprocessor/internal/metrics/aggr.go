package metrics

import "sync"

type Aggregator interface {
	Consume(m Metric)
	Export() Map
}

type Lock struct {
	mu   sync.Mutex
	next Aggregator
}

func NewLock(next Aggregator) *Lock {
	return &Lock{next: next}
}

func (l *Lock) Consume(m Metric) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.next.Consume(m)
}

func (l *Lock) Export() Map {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.next.Export()
}
