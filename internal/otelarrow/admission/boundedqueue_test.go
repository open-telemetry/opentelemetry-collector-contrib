// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/multierr"
)

func min(x, y int64) int64 {
	if x <= y {
		return x
	}
	return y
}

func max(x, y int64) int64 {
	if x >= y {
		return x
	}
	return y
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

var noopTelemetry = componenttest.NewNopTelemetrySettings()

func TestAcquireSimpleNoWaiters(t *testing.T) {
	maxLimitBytes := 1000
	maxLimitWaiters := 10
	numRequests := 40
	requestSize := 21

	bq := NewBoundedQueue(noopTelemetry, int64(maxLimitBytes), int64(maxLimitWaiters))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < numRequests; i++ {
		go func() {
			err := bq.Acquire(ctx, int64(requestSize))
			assert.NoError(t, err)
		}()
	}

	require.Never(t, func() bool {
		return bq.waiters.Len() > 0
	}, 2*time.Second, 10*time.Millisecond)

	for i := 0; i < numRequests; i++ {
		assert.NoError(t, bq.Release(int64(requestSize)))
		assert.Equal(t, int64(0), bq.currentWaiters)
	}

	assert.ErrorContains(t, bq.Release(int64(1)), "released more bytes than acquired")
	assert.NoError(t, bq.Acquire(ctx, int64(maxLimitBytes)))
}

func TestAcquireBoundedWithWaiters(t *testing.T) {
	tests := []struct {
		name            string
		maxLimitBytes   int64
		maxLimitWaiters int64
		numRequests     int64
		requestSize     int64
		timeout         time.Duration
	}{
		{
			name:            "below max waiters above max bytes",
			maxLimitBytes:   1000,
			maxLimitWaiters: 100,
			numRequests:     100,
			requestSize:     21,
			timeout:         5 * time.Second,
		},
		{
			name:            "above max waiters above max bytes",
			maxLimitBytes:   1000,
			maxLimitWaiters: 100,
			numRequests:     200,
			requestSize:     21,
			timeout:         5 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bq := NewBoundedQueue(noopTelemetry, tt.maxLimitBytes, tt.maxLimitWaiters)
			var blockedRequests int64
			numReqsUntilBlocked := tt.maxLimitBytes / tt.requestSize
			requestsAboveLimit := abs(tt.numRequests - numReqsUntilBlocked)
			tooManyWaiters := requestsAboveLimit > tt.maxLimitWaiters
			numRejected := max(requestsAboveLimit-tt.maxLimitWaiters, int64(0))

			// There should never be more blocked requests than maxLimitWaiters.
			blockedRequests = min(tt.maxLimitWaiters, requestsAboveLimit)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()
			var errs error
			for i := 0; i < int(tt.numRequests); i++ {
				go func() {
					err := bq.Acquire(ctx, tt.requestSize)
					bq.lock.Lock()
					defer bq.lock.Unlock()
					errs = multierr.Append(errs, err)
				}()
			}

			require.Eventually(t, func() bool {
				bq.lock.Lock()
				defer bq.lock.Unlock()
				return bq.waiters.Len() == int(blockedRequests)
			}, 3*time.Second, 10*time.Millisecond)

			assert.NoError(t, bq.Release(tt.requestSize))
			assert.Equal(t, bq.waiters.Len(), int(blockedRequests)-1)

			for i := 0; i < int(tt.numRequests-numRejected)-1; i++ {
				assert.NoError(t, bq.Release(tt.requestSize))
			}

			bq.lock.Lock()
			if tooManyWaiters {
				assert.ErrorContains(t, errs, ErrTooManyWaiters.Error())
			} else {
				assert.NoError(t, errs)
			}
			bq.lock.Unlock()

			// confirm all bytes were released by acquiring maxLimitBytes.
			assert.True(t, bq.TryAcquire(tt.maxLimitBytes))
		})
	}
}

func TestAcquireContextCanceled(t *testing.T) {
	maxLimitBytes := 1000
	maxLimitWaiters := 100
	numRequests := 100
	requestSize := 21
	numReqsUntilBlocked := maxLimitBytes / requestSize
	requestsAboveLimit := abs(int64(numRequests) - int64(numReqsUntilBlocked))

	blockedRequests := min(int64(maxLimitWaiters), requestsAboveLimit)

	exp := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exp))
	ts := noopTelemetry
	ts.TracerProvider = tp

	bq := NewBoundedQueue(ts, int64(maxLimitBytes), int64(maxLimitWaiters))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var errs error
	var wg sync.WaitGroup
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			err := bq.Acquire(ctx, int64(requestSize))
			bq.lock.Lock()
			defer bq.lock.Unlock()
			errs = multierr.Append(errs, err)
			wg.Done()
		}()
	}

	// Wait until all calls to Acquire() happen and we have the expected number of waiters.
	require.Eventually(t, func() bool {
		bq.lock.Lock()
		defer bq.lock.Unlock()
		return bq.waiters.Len() == int(blockedRequests)
	}, 3*time.Second, 10*time.Millisecond)

	cancel()
	wg.Wait()
	assert.ErrorContains(t, errs, "context canceled")

	// Expect spans named admission_blocked w/ context canceled.
	spans := exp.GetSpans()
	exp.Reset()
	assert.NotEmpty(t, spans)
	for _, span := range spans {
		assert.Equal(t, "admission_blocked", span.Name)
		assert.Equal(t, codes.Error, span.Status.Code)
		assert.Equal(t, "context canceled", span.Status.Description)
	}

	// Now all waiters should have returned and been removed.
	assert.Equal(t, 0, bq.waiters.Len())

	for i := 0; i < numReqsUntilBlocked; i++ {
		assert.NoError(t, bq.Release(int64(requestSize)))
		assert.Equal(t, int64(0), bq.currentWaiters)
	}
	assert.True(t, bq.TryAcquire(int64(maxLimitBytes)))

	// Expect no more spans, because admission was not blocked.
	spans = exp.GetSpans()
	require.Empty(t, spans)
}
