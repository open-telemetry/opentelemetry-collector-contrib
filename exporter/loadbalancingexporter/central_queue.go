// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"sync"
	"time"
)

const centralQueueLeasePollInterval = 10 * time.Millisecond
const (
	centralQueueInitialRetryDelay = 100 * time.Millisecond
	centralQueueMaxRetryDelay     = 5 * time.Second
)

type centralQueueSettings struct {
	maxCompressedBytes           int64
	maxInflightUncompressedBytes int64
	maxUncompressedBatchBytes    int
	telemetry                    *centralQueueTelemetry
}

type centralQueue struct {
	settings centralQueueSettings

	mu      sync.Mutex
	items   []centralQueueItem
	stopped bool

	currentCompressedBytes int64
	currentInflightBytes   int64
}

type centralQueueLease struct {
	queue *centralQueue
	item  centralQueueItem
	once  sync.Once
}

func newCentralQueue(settings centralQueueSettings) *centralQueue {
	return &centralQueue{settings: settings}
}

func (q *centralQueue) enqueue(item centralQueueItem) error {
	return q.enqueueAt(item, time.Now())
}

func (q *centralQueue) enqueueAt(item centralQueueItem, now time.Time) error {
	if q.settings.maxUncompressedBatchBytes > 0 && item.uncompressedBytes > q.settings.maxUncompressedBatchBytes {
		q.settings.telemetry.recordRejected(context.Background(), int64(item.compressedBytes))
		return errCentralQueueItemTooLarge
	}
	if q.settings.maxInflightUncompressedBytes > 0 && int64(item.uncompressedBytes) > q.settings.maxInflightUncompressedBytes {
		q.settings.telemetry.recordRejected(context.Background(), int64(item.compressedBytes))
		return errCentralQueueItemTooLarge
	}

	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return errCentralQueueStopped
	}
	if q.currentCompressedBytes+int64(item.compressedBytes) > q.settings.maxCompressedBytes {
		q.mu.Unlock()
		q.settings.telemetry.recordRejected(context.Background(), int64(item.compressedBytes))
		return errCentralQueueFull
	}
	if item.enqueuedAtUnixNano == 0 {
		item.enqueuedAtUnixNano = now.UnixNano()
	}
	q.items = append(q.items, item)
	q.currentCompressedBytes += int64(item.compressedBytes)
	snapshot := q.snapshotLocked()
	q.mu.Unlock()
	q.settings.telemetry.record(context.Background(), snapshot)
	return nil
}

func (q *centralQueue) lease(ctx context.Context) (*centralQueueLease, error) {
	ticker := time.NewTicker(centralQueueLeasePollInterval)
	defer ticker.Stop()

	for {
		if lease, err := q.tryLease(time.Now()); lease != nil || err != nil {
			return lease, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (q *centralQueue) tryLease(now time.Time) (*centralQueueLease, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		if q.stopped {
			return nil, errCentralQueueStopped
		}
		return nil, nil
	}

	nowUnixNano := now.UnixNano()
	readyInflightBlocked := false
	for i, item := range q.items {
		if item.nextAttemptUnixNano > nowUnixNano {
			continue
		}
		if q.currentInflightBytes+int64(item.uncompressedBytes) > q.settings.maxInflightUncompressedBytes {
			readyInflightBlocked = true
			continue
		}

		q.items = removeCentralQueueItem(q.items, i)
		q.currentInflightBytes += int64(item.uncompressedBytes)
		snapshot := q.snapshotLocked()
		q.settings.telemetry.record(context.Background(), snapshot)
		return &centralQueueLease{queue: q, item: item}, nil
	}
	if readyInflightBlocked {
		return nil, errCentralQueueInflightFull
	}
	return nil, nil
}

func removeCentralQueueItem(items []centralQueueItem, index int) []centralQueueItem {
	copy(items[index:], items[index+1:])
	items[len(items)-1] = centralQueueItem{}
	return items[:len(items)-1]
}

func (l *centralQueueLease) done() {
	l.once.Do(func() {
		l.queue.mu.Lock()
		l.queue.currentInflightBytes -= int64(l.item.uncompressedBytes)
		l.queue.currentCompressedBytes -= int64(l.item.compressedBytes)
		snapshot := l.queue.snapshotLocked()
		l.queue.mu.Unlock()
		l.queue.settings.telemetry.record(context.Background(), snapshot)
	})
}

func (l *centralQueueLease) requeue(nextAttempt time.Time) error {
	var err error
	l.once.Do(func() {
		item := l.item
		item.nextAttemptUnixNano = nextAttempt.UnixNano()
		l.queue.settings.telemetry.recordRetry(context.Background())

		l.queue.mu.Lock()
		l.queue.currentInflightBytes -= int64(item.uncompressedBytes)
		if l.queue.stopped {
			l.queue.currentCompressedBytes -= int64(item.compressedBytes)
			err = errCentralQueueStopped
		} else {
			l.queue.items = append(l.queue.items, item)
		}
		snapshot := l.queue.snapshotLocked()
		l.queue.mu.Unlock()
		l.queue.settings.telemetry.record(context.Background(), snapshot)
	})
	return err
}

func (q *centralQueue) stop() {
	q.mu.Lock()
	q.stopped = true
	q.mu.Unlock()
}

func (q *centralQueue) compressedBytes() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.currentCompressedBytes
}

func (q *centralQueue) inflightUncompressedBytes() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.currentInflightBytes
}

func (q *centralQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *centralQueue) snapshotLocked() centralQueueSnapshot {
	return centralQueueSnapshot{
		compressedBytes:      q.currentCompressedBytes,
		compressedCapacity:   q.settings.maxCompressedBytes,
		items:                int64(len(q.items)),
		inflightUncompressed: q.currentInflightBytes,
	}
}

func centralQueueRetryDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	shift := min(attempt, 6)
	delay := centralQueueInitialRetryDelay * time.Duration(1<<shift)
	return min(delay, centralQueueMaxRetryDelay)
}
