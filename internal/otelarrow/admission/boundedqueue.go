// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package admission // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission"

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrTooManyWaiters = status.Error(grpccodes.ResourceExhausted, "rejecting request, too much pending data")
var ErrRequestTooLarge = status.Error(grpccodes.InvalidArgument, "rejecting request, request is too large")

type BoundedQueue struct {
	maxLimitBytes   int64
	maxLimitWaiters int64
	currentBytes    int64
	currentWaiters  int64
	lock            sync.Mutex
	waiters         *orderedmap.OrderedMap[uuid.UUID, waiter]
	tracer          trace.Tracer
}

type waiter struct {
	readyCh      chan struct{}
	pendingBytes int64
	ID           uuid.UUID
}

func NewBoundedQueue(ts component.TelemetrySettings, maxLimitBytes, maxLimitWaiters int64) Queue {
	return &BoundedQueue{
		maxLimitBytes:   maxLimitBytes,
		maxLimitWaiters: maxLimitWaiters,
		waiters:         orderedmap.New[uuid.UUID, waiter](),
		tracer:          ts.TracerProvider.Tracer("github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow"),
	}
}

func (bq *BoundedQueue) admit(pendingBytes int64) (bool, error) {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	if pendingBytes > bq.maxLimitBytes { // will never succeed
		return false, ErrRequestTooLarge
	}

	if bq.currentBytes+pendingBytes <= bq.maxLimitBytes { // no need to wait to admit
		bq.currentBytes += pendingBytes
		return true, nil
	}

	// since we were unable to admit, check if we can wait.
	if bq.currentWaiters+1 > bq.maxLimitWaiters { // too many waiters
		return false, ErrTooManyWaiters
	}

	// if we got to this point we need to wait to acquire bytes, so update currentWaiters before releasing mutex.
	bq.currentWaiters++
	return false, nil
}

func (bq *BoundedQueue) Acquire(ctx context.Context, pendingBytes int64) error {
	success, err := bq.admit(pendingBytes)
	if err != nil || success {
		return err
	}

	// otherwise we need to wait for bytes to be released
	curWaiter := waiter{
		pendingBytes: pendingBytes,
		readyCh:      make(chan struct{}),
	}

	bq.lock.Lock()

	// generate unique key
	for {
		id := uuid.New()
		_, keyExists := bq.waiters.Get(id)
		if keyExists {
			continue
		}
		bq.waiters.Set(id, curWaiter)
		curWaiter.ID = id
		break
	}

	bq.lock.Unlock()
	ctx, span := bq.tracer.Start(ctx, "admission_blocked",
		trace.WithAttributes(attribute.Int64("pending", pendingBytes)))
	defer span.End()

	select {
	case <-curWaiter.readyCh:
		return nil
	case <-ctx.Done():
		// canceled before acquired so remove waiter.
		bq.lock.Lock()
		defer bq.lock.Unlock()
		err = fmt.Errorf("context canceled: %w ", ctx.Err())
		span.SetStatus(codes.Error, "context canceled")

		_, found := bq.waiters.Delete(curWaiter.ID)
		if !found {
			return err
		}

		bq.currentWaiters--
		return err
	}
}

func (bq *BoundedQueue) Release(pendingBytes int64) error {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	bq.currentBytes -= pendingBytes

	if bq.currentBytes < 0 {
		return fmt.Errorf("released more bytes than acquired")
	}

	for {
		if bq.waiters.Len() == 0 {
			return nil
		}
		next := bq.waiters.Oldest()
		nextWaiter := next.Value
		nextKey := next.Key
		if bq.currentBytes+nextWaiter.pendingBytes <= bq.maxLimitBytes {
			bq.currentBytes += nextWaiter.pendingBytes
			bq.currentWaiters--
			close(nextWaiter.readyCh)
			_, found := bq.waiters.Delete(nextKey)
			if !found {
				return fmt.Errorf("deleting waiter that doesn't exist")
			}
			continue
		}
		break
	}

	return nil
}

func (bq *BoundedQueue) TryAcquire(pendingBytes int64) bool {
	bq.lock.Lock()
	defer bq.lock.Unlock()
	if bq.currentBytes+pendingBytes <= bq.maxLimitBytes {
		bq.currentBytes += pendingBytes
		return true
	}
	return false
}
