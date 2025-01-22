package state

import "sync/atomic"

type retryCountTracker struct {
	count atomic.Int32
	cancelManager
}

func (r *retryCountTracker) GetCount() int {
	return int(r.count.Load())
}

func (r *retryCountTracker) IncrementCount() int {
	return int(r.count.Add(1))
}

func (r *retryCountTracker) resetCount() {
	r.Cancel()
	r.count.Store(0)
}
