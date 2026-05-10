// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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

func TestCentralQueueLeaseTreatsZeroInflightBudgetAsUnlimited(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:        100,
		maxUncompressedBatchBytes: 100,
		targetCompressedBytes:     10,
		maxBatchDelay:             time.Second,
	})
	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 40, count: 1}))
	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-b"), compressedBytes: 10, uncompressedBytes: 40, count: 1}))

	first, err := q.tryLease(time.Now())
	require.NoError(t, err)
	require.NotNil(t, first)
	require.EqualValues(t, 40, q.inflightUncompressedBytes())

	second, err := q.tryLease(time.Now())
	require.NoError(t, err)
	require.NotNil(t, second)
	require.EqualValues(t, 80, q.inflightUncompressedBytes())

	first.done()
	second.done()
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

	retryLease, err := q.tryLease(time.Now().Add(centralQueueRetryDelayUpperBound(0)))
	require.NoError(t, err)
	require.NotNil(t, retryLease)
	retryLease.done()
	require.Zero(t, q.compressedBytes())
}

func TestCentralQueueLeaseCoalescesReadyItemsByRoutingKey(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
		maxBatchDelay:                time.Second,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("lane-b"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))

	lease, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.Equal(t, []byte("lane-a"), lease.window.routingKey)
	require.Len(t, lease.window.items, 2)
	require.Equal(t, 20, lease.window.compressedBytes)
	require.Equal(t, 40, lease.window.uncompressedBytes)
	require.Equal(t, centralQueueFlushReasonTargetReached, lease.window.flushReason)
	require.Equal(t, 1, q.len())
	require.EqualValues(t, 30, q.compressedBytes())

	lease.done()
	require.EqualValues(t, 10, q.compressedBytes())
}

func TestCentralQueueReadyWindowsGiveEachReadyLaneATurnBeforeHotLaneRepeats(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
		maxBatchDelay:                time.Second,
		maxReadyWindows:              2,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("hot"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("hot"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("hot"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("hot"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("cold"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("cold"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))

	first, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, []byte("hot"), first.window.routingKey)
	require.Equal(t, centralQueueFlushReasonTargetReached, first.window.flushReason)

	second, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, []byte("cold"), second.window.routingKey)
	require.Equal(t, centralQueueFlushReasonTargetReached, second.window.flushReason)

	first.done()
	second.done()
}

func TestCentralQueueReadyWindowsHandleInterleavedLaneItems(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
		maxBatchDelay:                time.Second,
		maxReadyWindows:              2,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-b"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-b"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))

	first, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, []byte("lane-a"), first.window.routingKey)
	require.Len(t, first.window.items, 2)

	second, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, []byte("lane-b"), second.window.routingKey)
	require.Len(t, second.window.items, 2)

	first.done()
	second.done()
	require.Zero(t, q.len())
	require.Zero(t, q.compressedBytes())
}

func TestCentralQueueReadyWindowsRecollectFallbackAfterTargetRemoval(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
		maxBatchDelay:                250 * time.Millisecond,
		maxReadyWindows:              2,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("target"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("target"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("fallback"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))

	lease, err := q.tryLease(now.Add(250 * time.Millisecond))
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.Equal(t, []byte("target"), lease.window.routingKey)
	require.Len(t, lease.window.items, 2)
	require.Equal(t, centralQueueFlushReasonTargetReached, lease.window.flushReason)

	fallbackLease, err := q.tryLease(now.Add(250 * time.Millisecond))
	require.NoError(t, err)
	require.NotNil(t, fallbackLease)
	require.Equal(t, []byte("fallback"), fallbackLease.window.routingKey)
	require.Len(t, fallbackLease.window.items, 1)
	require.Equal(t, centralQueueFlushReasonMaxDelayLowTraffic, fallbackLease.window.flushReason)

	lease.done()
	fallbackLease.done()
	require.Zero(t, q.len())
	require.Zero(t, q.compressedBytes())
}

func TestCentralQueueReadyWindowsDoNotLetHotTargetLaneStarveFallbackLane(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
		maxBatchDelay:                250 * time.Millisecond,
		maxReadyWindows:              2,
	})
	now := time.Unix(10, 0)
	for range 4 {
		require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("hot"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	}
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("fallback"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))

	first, err := q.tryLease(now.Add(250 * time.Millisecond))
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, []byte("hot"), first.window.routingKey)
	require.Equal(t, centralQueueFlushReasonTargetReached, first.window.flushReason)

	second, err := q.tryLease(now.Add(250 * time.Millisecond))
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, []byte("fallback"), second.window.routingKey)
	require.Equal(t, centralQueueFlushReasonMaxDelayLowTraffic, second.window.flushReason)

	first.done()
	second.done()
}

func TestCentralQueueReadyWindowsBuildTargetSizedWindowsBeforeConsumersLease(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
		maxBatchDelay:                time.Second,
		maxReadyWindows:              4,
	})
	now := time.Unix(10, 0)
	for range 8 {
		require.NoError(t, q.enqueueAt(centralQueueItem{
			signal:            signalKindLogs,
			routingKey:        []byte("lane-a"),
			compressedBytes:   10,
			uncompressedBytes: 10,
			count:             1,
		}, now))
	}

	for range 4 {
		lease, err := q.tryLease(now)
		require.NoError(t, err)
		require.NotNil(t, lease)
		require.Equal(t, []byte("lane-a"), lease.window.routingKey)
		require.Len(t, lease.window.items, 2)
		require.Equal(t, 20, lease.window.compressedBytes)
		require.Equal(t, centralQueueFlushReasonTargetReached, lease.window.flushReason)
		lease.done()
	}
	require.Zero(t, q.len())
	require.Zero(t, q.compressedBytes())
}

func TestCentralQueueReadyWindowsReserveInflightBudgetUntilLeasedWindowDone(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
		maxBatchDelay:                time.Second,
		maxReadyWindows:              2,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-b"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-b"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))

	lease, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.EqualValues(t, 80, q.inflightUncompressedBytes())
	requireCentralQueueReadyWindows(t, q, 1)

	lease.done()
	require.EqualValues(t, 40, q.inflightUncompressedBytes())
	requireCentralQueueReadyWindows(t, q, 1)
}

func TestCentralQueueLeasePrefersTargetWindowOverOlderUnderfilledWindow(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
		maxBatchDelay:                250 * time.Millisecond,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("older"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("target"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now.Add(10*time.Millisecond)))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("target"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now.Add(20*time.Millisecond)))

	lease, err := q.tryLease(now.Add(250 * time.Millisecond))
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.Equal(t, []byte("target"), lease.window.routingKey)
	require.Len(t, lease.window.items, 2)
	require.Equal(t, centralQueueFlushReasonTargetReached, lease.window.flushReason)
	require.Equal(t, 1, q.len())

	lease.done()
	require.EqualValues(t, 10, q.compressedBytes())
}

func TestCentralQueueLeaseDoesNotLeaseUnderfilledWindowWhenTargetWindowIsInflightBlocked(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 50,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
		maxBatchDelay:                250 * time.Millisecond,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("inflight"), compressedBytes: 20, uncompressedBytes: 30, count: 1}, now))
	inflightLease, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, inflightLease)

	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("older"), compressedBytes: 10, uncompressedBytes: 10, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("target"), compressedBytes: 10, uncompressedBytes: 15, count: 1}, now.Add(10*time.Millisecond)))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("target"), compressedBytes: 10, uncompressedBytes: 15, count: 1}, now.Add(20*time.Millisecond)))

	lease, err := q.tryLease(now.Add(250 * time.Millisecond))
	require.ErrorIs(t, err, errCentralQueueInflightFull)
	require.Nil(t, lease)
	require.Equal(t, 3, q.len())
	require.EqualValues(t, 30, q.inflightUncompressedBytes())

	inflightLease.done()
}

func TestCentralQueueLeaseDoesNotCoalesceDifferentRoutingKeys(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        100,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-b"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))

	lease, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.Equal(t, []byte("lane-a"), lease.window.routingKey)
	require.Len(t, lease.window.items, 1)
	require.Equal(t, centralQueueFlushReasonMaxDelayLowTraffic, lease.window.flushReason)
}

func TestCentralQueueLeaseMarksHardCapFlush(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    30,
		targetCompressedBytes:        100,
		maxBatchDelay:                time.Second,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))

	lease, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.Len(t, lease.window.items, 1)
	require.Equal(t, centralQueueFlushReasonHardCap, lease.window.flushReason)
}

func TestCentralQueueLeaseWaitsForSmallWindowMaxDelay(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        100,
		maxBatchDelay:                250 * time.Millisecond,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))

	lease, err := q.tryLease(now.Add(100 * time.Millisecond))
	require.NoError(t, err)
	require.Nil(t, lease)

	lease, err = q.tryLease(now.Add(250 * time.Millisecond))
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.Len(t, lease.window.items, 1)
	require.Equal(t, centralQueueFlushReasonMaxDelayLowTraffic, lease.window.flushReason)
}

func TestCentralQueueRequeuesWholeCoalescedWindow(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))

	lease, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.Len(t, lease.window.items, 2)
	require.NoError(t, lease.requeue(now))
	require.Equal(t, 2, q.len())
	require.EqualValues(t, 20, q.compressedBytes())

	lease, err = q.tryLease(now)
	require.NoError(t, err)
	require.Nil(t, lease)

	lease, err = q.tryLease(now.Add(centralQueueRetryDelayUpperBound(0)))
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.Len(t, lease.window.items, 2)
	require.Equal(t, 1, lease.window.maxAttempt)
}

func TestCentralQueueRequeueUsesPerItemRetryDelay(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		targetCompressedBytes:        20,
	})
	now := time.Unix(10, 0)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1, attempt: 3}, now))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindMetrics, routingKey: []byte("lane-a"), compressedBytes: 10, uncompressedBytes: 20, count: 1}, now))

	lease, err := q.tryLease(now)
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.Len(t, lease.window.items, 2)
	require.Equal(t, 3, lease.window.maxAttempt)

	require.NoError(t, lease.requeue(now))

	q.mu.Lock()
	defer q.mu.Unlock()
	require.Len(t, q.items, 2)
	require.Equal(t, 4, q.items[0].attempt)
	requireNextAttemptWithinRetryDelay(t, now, 3, q.items[0].nextAttemptUnixNano)
	require.Equal(t, 1, q.items[1].attempt)
	requireNextAttemptWithinRetryDelay(t, now, 0, q.items[1].nextAttemptUnixNano)
}

func TestCentralQueueSnapshotReportsOldestQueuedItemAge(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
	})
	base := time.Unix(10, 0)
	snapshotAt := func(now time.Time) centralQueueSnapshot {
		q.mu.Lock()
		defer q.mu.Unlock()
		return q.snapshotLockedAt(now)
	}

	require.Zero(t, snapshotAt(base).oldestItemAgeMillis)
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, compressedBytes: 10, uncompressedBytes: 10, count: 1}, base))
	require.NoError(t, q.enqueueAt(centralQueueItem{signal: signalKindLogs, compressedBytes: 10, uncompressedBytes: 10, count: 1}, base.Add(100*time.Millisecond)))

	require.EqualValues(t, 250, snapshotAt(base.Add(250*time.Millisecond)).oldestItemAgeMillis)

	lease, err := q.tryLease(base.Add(250 * time.Millisecond))
	require.NoError(t, err)
	require.NotNil(t, lease)

	require.EqualValues(t, 150, snapshotAt(base.Add(250*time.Millisecond)).oldestItemAgeMillis)

	require.NoError(t, lease.requeue(base.Add(time.Second)))
	require.EqualValues(t, 300, snapshotAt(base.Add(300*time.Millisecond)).oldestItemAgeMillis)

	secondLease, err := q.tryLease(base.Add(300 * time.Millisecond))
	require.NoError(t, err)
	require.NotNil(t, secondLease)
	secondLease.done()
	require.EqualValues(t, 300, snapshotAt(base.Add(300*time.Millisecond)).oldestItemAgeMillis)

	retryReadyAt := base.Add(time.Second + centralQueueRetryDelayUpperBound(0))
	retryLease, err := q.tryLease(retryReadyAt)
	require.NoError(t, err)
	require.NotNil(t, retryLease)
	retryLease.done()
	require.Zero(t, snapshotAt(retryReadyAt).oldestItemAgeMillis)
}

func TestCentralQueueTelemetryOldestItemAgeAdvancesWithoutQueueMutation(t *testing.T) {
	reader := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, reader.Shutdown(context.WithoutCancel(t.Context())))
	})
	telemetry, err := newCentralQueueTelemetry(reader.NewTelemetrySettings(), signalKindLogs)
	require.NoError(t, err)
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		telemetry:                    telemetry,
	})

	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 10, uncompressedBytes: 10, count: 1}))

	require.Eventually(t, func() bool {
		metric, err := reader.GetMetric("otelcol_loadbalancer_central_queue_oldest_item_age")
		if err != nil {
			return false
		}
		gauge, ok := metric.Data.(metricdata.Gauge[int64])
		if !ok || len(gauge.DataPoints) != 1 {
			return false
		}
		return gauge.DataPoints[0].Value >= 10
	}, time.Second, 10*time.Millisecond)
}

func TestCentralQueueStopUnregistersOldestItemAgeObserver(t *testing.T) {
	reader := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, reader.Shutdown(context.WithoutCancel(t.Context())))
	})
	telemetry, err := newCentralQueueTelemetry(reader.NewTelemetrySettings(), signalKindLogs)
	require.NoError(t, err)
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
		telemetry:                    telemetry,
	})

	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 10, uncompressedBytes: 10, count: 1}))
	require.Eventually(t, func() bool {
		metric, err := reader.GetMetric("otelcol_loadbalancer_central_queue_oldest_item_age")
		if err != nil {
			return false
		}
		gauge, ok := metric.Data.(metricdata.Gauge[int64])
		return ok && len(gauge.DataPoints) == 1
	}, time.Second, 10*time.Millisecond)

	q.stop()

	require.Eventually(t, func() bool {
		metric, err := reader.GetMetric("otelcol_loadbalancer_central_queue_oldest_item_age")
		if err != nil {
			return true
		}
		gauge, ok := metric.Data.(metricdata.Gauge[int64])
		return ok && len(gauge.DataPoints) == 0
	}, time.Second, 10*time.Millisecond)
}

func TestCentralQueueRetriesDoNotGrowOldestEnqueuedHeap(t *testing.T) {
	q := newCentralQueue(centralQueueSettings{
		maxCompressedBytes:           100,
		maxInflightUncompressedBytes: 100,
		maxUncompressedBatchBytes:    100,
	})
	base := time.Unix(10, 0)
	blockedOldest := centralQueueItem{
		signal:              signalKindLogs,
		compressedBytes:     10,
		uncompressedBytes:   10,
		count:               1,
		nextAttemptUnixNano: base.Add(time.Hour).UnixNano(),
		enqueuedAtUnixNano:  base.UnixNano(),
	}
	retryingItem := centralQueueItem{
		signal:             signalKindLogs,
		compressedBytes:    10,
		uncompressedBytes:  10,
		count:              1,
		enqueuedAtUnixNano: base.Add(time.Millisecond).UnixNano(),
	}
	require.NoError(t, q.enqueueAt(blockedOldest, base))
	require.NoError(t, q.enqueueAt(retryingItem, base))

	now := base.Add(time.Second)
	for range 10 {
		lease, err := q.tryLease(now)
		require.NoError(t, err)
		require.NotNil(t, lease)
		require.NoError(t, lease.requeue(now))
		now = now.Add(centralQueueMaxRetryDelay + centralQueueMaxRetryJitter)
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	require.Len(t, q.enqueuedAtCounts, 2)
	require.Len(t, q.oldestEnqueuedAt, 2)
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
		targetCompressedBytes:        100,
	})
	require.NoError(t, q.enqueue(centralQueueItem{signal: signalKindLogs, compressedBytes: 10, uncompressedBytes: 10, count: 1}))
	q.stop()

	lease, err := q.lease(t.Context())
	require.NoError(t, err)
	require.Equal(t, centralQueueFlushReasonShutdown, lease.window.flushReason)
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
	require.GreaterOrEqual(t, retryAfterEnqueue, centralQueueRetryDelay(0))
	require.LessOrEqual(t, retryAfterEnqueue, centralQueueRetryDelayUpperBound(0)+50*time.Millisecond)
}

func requireNextAttemptWithinRetryDelay(t *testing.T, now time.Time, attempt int, nextAttemptUnixNano int64) {
	t.Helper()
	nextAttempt := time.Unix(0, nextAttemptUnixNano)
	require.GreaterOrEqual(t, nextAttempt.Sub(now), centralQueueRetryDelay(attempt))
	require.LessOrEqual(t, nextAttempt.Sub(now), centralQueueRetryDelayUpperBound(attempt))
}

func centralQueueRetryDelayUpperBound(attempt int) time.Duration {
	delay := centralQueueRetryDelay(attempt)
	return delay + centralQueueRetryJitterLimit(delay)
}

func requireCentralQueueReadyWindows(t *testing.T, q *centralQueue, expected int) {
	t.Helper()

	q.mu.Lock()
	defer q.mu.Unlock()
	require.Len(t, q.ready, expected)
}
