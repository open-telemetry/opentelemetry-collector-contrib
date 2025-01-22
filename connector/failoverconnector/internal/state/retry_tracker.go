package state // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

import (
	"context"
	"sync/atomic"
	"time"
)

type RetryTracker struct {
	trackers     []retryCount
	maxRetries   int
	retryBackoff time.Duration
}

func NewRetryTracker(lenPriority int, maxRetries int, retryBackoff time.Duration) *RetryTracker {
	trackers := make([]retryCount, lenPriority)

	return &RetryTracker{
		trackers:     trackers,
		maxRetries:   maxRetries,
		retryBackoff: retryBackoff,
	}
}

func (r *RetryTracker) ExceededMaxRetries(idx int) bool {
	return r.maxRetries > 0 && idx < len(r.trackers) && ((&r.trackers[idx]).GetCount() >= r.maxRetries)
}

// CheckContinueRetry checks if retry should be suspended if all higher priority levels have exceeded their max retries
func (r *RetryTracker) CheckContinueRetry(idx int) bool {
	for i := 0; i < idx; i++ {
		if r.maxRetries == 0 || r.retryBackoff > 0 || (&r.trackers[i]).GetCount() < r.maxRetries {
			return true
		}
	}
	return false
}

func (r *RetryTracker) FailedRetry(idx int) {
	if r.ExceededMaxRetries(idx) || r.maxRetries == 0 {
		return
	}
	(&r.trackers[idx]).modifyRetryCount(r.maxRetries, r.retryBackoff)
}

func (r *RetryTracker) SuccessfullRetry(idx int) {
	(&r.trackers[idx]).resetCount()
}

func (r *RetryTracker) Shutdown() {
	for i := 0; i < len(r.trackers); i++ {
		(&r.trackers[i]).Cancel()
	}
}

type retryCount struct {
	count atomic.Int32
	cancelManager
}

func (r *retryCount) modifyRetryCount(maxRetries int, backoff time.Duration) {
	retryCount := r.IncrementCount()
	if retryCount >= maxRetries {
		ctx, cancel := context.WithCancel(context.Background())
		r.CancelAndSet(cancel)
		r.startBackoffTimer(ctx, backoff)
	}
}

func (r *retryCount) startBackoffTimer(ctx context.Context, backoff time.Duration) {
	go func() {
		ticker := time.NewTicker(backoff)
		defer ticker.Stop()

		select {
		case <-ticker.C:
			r.resetCount()
		case <-ctx.Done():
			return
		}
	}()
}

func (r *retryCount) GetCount() int {
	return int(r.count.Load())
}

func (r *retryCount) IncrementCount() int {
	return int(r.count.Add(1))
}

func (r *retryCount) resetCount() {
	r.Cancel()
	r.count.Store(0)
}

// For Testing
func (r *RetryTracker) SetRetryCountToValue(idx int, val int) {
	r.trackers[idx].count.Store(int32(val))
}

func (r *RetryTracker) ResetRetryCount(idx int) {
	r.trackers[idx].count.Store(0)
}
