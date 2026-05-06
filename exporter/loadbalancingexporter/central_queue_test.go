// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCentralQueueAdmitsByCompressedBytes(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           10,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
	})

	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 4, uncompressedBytes: 40, count: 1}))
	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 6, uncompressedBytes: 60, count: 1}))
	require.ErrorIs(t, q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 1, uncompressedBytes: 10, count: 1}), errCentralQueueFull)
	require.EqualValues(t, 10, q.compressedBytes())
}

func TestCentralQueueLeaseReservesInflightUncompressedBytes(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 50,
		maxUncompressedBatchBytes:    100,
	})
	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindMetrics, compressedBytes: 10, uncompressedBytes: 40, count: 1}))
	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindMetrics, compressedBytes: 10, uncompressedBytes: 40, count: 1}))

	lease, err := q.lease(t.Context())
	require.NoError(t, err)
	require.EqualValues(t, 40, q.inflightUncompressedBytes())

	_, err = q.lease(t.Context())
	require.ErrorIs(t, err, errCentralQueueInflightFull)

	lease.done()
	require.EqualValues(t, 0, q.inflightUncompressedBytes())
}

func TestCentralQueueLeaseReservesCompressedBytesUntilDoneOrRequeue(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           10,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
	})
	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 10, uncompressedBytes: 40, count: 1}))

	lease, err := q.lease(t.Context())
	require.NoError(t, err)
	require.EqualValues(t, 10, q.compressedBytes())
	require.ErrorIs(t, q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 1, uncompressedBytes: 1, count: 1}), errCentralQueueFull)

	require.NoError(t, lease.requeue(time.Now()))
	require.Equal(t, 1, q.len())
	require.EqualValues(t, 10, q.compressedBytes())

	retryLease, err := q.lease(t.Context())
	require.NoError(t, err)
	retryLease.done()
	require.Zero(t, q.compressedBytes())
}

func TestCentralQueueRejectsOversizedUncompressedItem(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    50,
	})

	err := q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 10, uncompressedBytes: 51, count: 1})
	require.ErrorIs(t, err, errCentralQueueItemTooLarge)
	require.Zero(t, q.compressedBytes())
}

func TestCentralQueueRejectsItemLargerThanInflightBudget(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 50,
		maxUncompressedBatchBytes:    100,
	})

	err := q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 10, uncompressedBytes: 51, count: 1})
	require.ErrorIs(t, err, errCentralQueueItemTooLarge)
	require.Zero(t, q.compressedBytes())
}

func TestCentralQueueLeaseReturnsContextErrorWhenEmpty(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := q.lease(ctx)
	require.ErrorIs(t, err, ctx.Err())
}

func TestCentralQueueStopAllowsDrainingExistingItems(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
	})
	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 10, uncompressedBytes: 10, count: 1}))
	q.stop()

	lease, err := q.lease(t.Context())
	require.NoError(t, err)
	lease.done()

	_, err = q.lease(t.Context())
	require.ErrorIs(t, err, errCentralQueueStopped)
}

func requireCentralQueueFirstRetryDelay(t *testing.T, q *centralQueue) {
	t.Helper()

	var item centralQueueItem
	require.Eventually(t, func() bool {
		q.mu.Lock()
		defer q.mu.Unlock()
		if len(q.items) != 1 || q.items[0].attempt != 1 || q.items[0].nextAttemptUnixNano == 0 {
			return false
		}
		item = q.items[0]
		return true
	}, time.Second, time.Millisecond)

	retryAfterEnqueue := time.Unix(0, item.nextAttemptUnixNano).Sub(time.Unix(0, item.enqueuedAtUnixNano))
	require.LessOrEqual(t, retryAfterEnqueue, centralQueueInitialRetryDelay+50*time.Millisecond)
}
