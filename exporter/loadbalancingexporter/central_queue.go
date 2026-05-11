// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"bytes"
	"container/heap"
	"context"
	"encoding/binary"
	"hash/crc32"
	"sort"
	"sync"
	"time"
)

const centralQueueLeasePollInterval = 10 * time.Millisecond
const (
	centralQueueInitialRetryDelay = 100 * time.Millisecond
	centralQueueMaxRetryDelay     = 5 * time.Second
	centralQueueMaxRetryJitter    = 100 * time.Millisecond
)

func centralQueueReadyWindowLimit(consumers int) int {
	if consumers <= 0 {
		return defaultCentralQueueNumConsumers
	}
	return consumers
}

func centralQueueLaneCount(numConsumers int) int {
	if numConsumers <= 0 {
		numConsumers = 1
	}
	target := max(defaultCentralQueueLaneCount, numConsumers*2)
	laneCount := 1
	for laneCount < target {
		laneCount <<= 1
	}
	return laneCount
}

type centralQueueSettings struct {
	maxCompressedBytes           int64
	maxInflightUncompressedBytes int64
	maxUncompressedBatchBytes    int
	targetCompressedBytes        int64
	maxBatchDelay                time.Duration
	maxReadyWindows              int
	telemetry                    *centralQueueTelemetry
}

type centralQueue struct {
	settings centralQueueSettings

	mu      sync.Mutex
	items   []centralQueueItem
	stopped bool
	ready   []centralQueueWindow

	currentCompressedBytes int64
	currentInflightBytes   int64
	enqueuedAtCounts       map[int64]int
	enqueuedAtHeapEntries  map[int64]struct{}
	oldestEnqueuedAt       centralQueueEnqueuedAtHeap
}

type centralQueueLease struct {
	queue  *centralQueue
	window centralQueueWindow
	item   centralQueueItem
	once   sync.Once
}

type centralQueueWindow struct {
	routingKey        []byte
	items             []centralQueueItem
	compressedBytes   int
	uncompressedBytes int
	count             int
	oldestEnqueuedAt  int64
	maxAttempt        int
	flushReason       centralQueueFlushReason
}

type centralQueueFlushReason string

const (
	centralQueueFlushReasonTargetReached      centralQueueFlushReason = "target_reached"
	centralQueueFlushReasonHardCap            centralQueueFlushReason = "hard_cap"
	centralQueueFlushReasonMaxDelayLowTraffic centralQueueFlushReason = "max_delay_low_traffic"
	centralQueueFlushReasonShutdown           centralQueueFlushReason = "shutdown"
)

type centralQueueWindowCandidate struct {
	window  centralQueueWindow
	indexes []int
}

func newCentralQueue(settings centralQueueSettings) *centralQueue {
	if settings.targetCompressedBytes <= 0 {
		settings.targetCompressedBytes = 1
	}
	if settings.maxReadyWindows <= 0 {
		settings.maxReadyWindows = 1
	}
	q := &centralQueue{settings: settings}
	q.settings.telemetry.observeOldestItemAge(q.oldestItemAgeMillis)
	q.settings.telemetry.observeSchedulerState(q.schedulerSnapshot)
	return q
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
	q.trackOldestEnqueuedAtLocked(item)
	snapshot := q.snapshotLockedAt(now)
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

	if lease := q.leaseReadyWindowLocked(); lease != nil {
		return lease, nil
	}

	if len(q.items) == 0 {
		if q.stopped {
			return nil, errCentralQueueStopped
		}
		return nil, nil
	}

	state := q.prepareReadyWindowsLocked(now)
	if lease := q.leaseReadyWindowLocked(); lease != nil {
		return lease, nil
	}
	if state == centralQueueSchedulerStateInflightBytes {
		return nil, errCentralQueueInflightFull
	}
	return nil, nil
}

func (q *centralQueue) prepareReadyWindowsLocked(now time.Time) centralQueueSchedulerState {
	state := centralQueueSchedulerStateWaiting
	for len(q.ready) < q.settings.maxReadyWindows {
		targetCandidates, fallbackCandidates, _ := q.collectWindowCandidatesLocked(now)
		if len(targetCandidates) == 0 && len(fallbackCandidates) == 0 {
			return state
		}

		scheduledTarget, blocked := q.scheduleReadyWindowCandidatesLocked(targetCandidates)
		if !scheduledTarget && blocked {
			return centralQueueSchedulerStateInflightBytes
		}
		if scheduledTarget {
			state = centralQueueSchedulerStateReady
		}
		if len(q.ready) >= q.settings.maxReadyWindows {
			state = centralQueueSchedulerStateReady
			continue
		}
		if scheduledTarget {
			_, fallbackCandidates, _ = q.collectWindowCandidatesLocked(now)
		}

		scheduledFallback, blocked := q.scheduleReadyWindowCandidatesLocked(fallbackCandidates)
		if blocked {
			return centralQueueSchedulerStateInflightBytes
		}
		if !scheduledTarget && !scheduledFallback {
			return state
		}
		state = centralQueueSchedulerStateReady
	}
	return centralQueueSchedulerStateReadyWindowLimit
}

// collectWindowCandidatesLocked evaluates one candidate per routing key so a hot
// lane cannot fill the bounded ready backlog before other ready lanes get a turn.
func (q *centralQueue) collectWindowCandidatesLocked(now time.Time) ([]centralQueueWindowCandidate, []centralQueueWindowCandidate, bool) {
	nowUnixNano := now.UnixNano()
	evaluatedRoutingKeys := make(map[string]struct{})
	targetCandidates := make([]centralQueueWindowCandidate, 0)
	fallbackCandidates := make([]centralQueueWindowCandidate, 0)
	hasReady := false
	for _, item := range q.items {
		if item.nextAttemptUnixNano > nowUnixNano {
			continue
		}
		hasReady = true
		routingKey := string(item.routingKey)
		if _, ok := evaluatedRoutingKeys[routingKey]; ok {
			continue
		}
		evaluatedRoutingKeys[routingKey] = struct{}{}
		candidate, ok := q.buildWindowCandidateLocked(item.routingKey, now)
		if !ok {
			continue
		}
		if candidate.window.flushReason != centralQueueFlushReasonTargetReached {
			fallbackCandidates = append(fallbackCandidates, candidate)
			continue
		}
		targetCandidates = append(targetCandidates, candidate)
	}
	return targetCandidates, fallbackCandidates, hasReady
}

func (q *centralQueue) scheduleReadyWindowCandidatesLocked(candidates []centralQueueWindowCandidate) (bool, bool) {
	if len(candidates) == 0 {
		return false, false
	}

	selected := make([]centralQueueWindowCandidate, 0, len(candidates))
	pendingInflightBytes := q.currentInflightBytes
	blocked := false
	for i := range candidates {
		candidate := &candidates[i]
		if len(q.ready)+len(selected) >= q.settings.maxReadyWindows {
			break
		}
		if q.windowInflightBlockedWithBase(candidate.window, pendingInflightBytes) {
			blocked = true
			continue
		}
		selected = append(selected, *candidate)
		pendingInflightBytes += int64(candidate.window.uncompressedBytes)
	}

	if len(selected) == 0 {
		return false, blocked
	}

	for i := range selected {
		candidate := &selected[i]
		q.ready = append(q.ready, candidate.window)
		q.currentInflightBytes += int64(candidate.window.uncompressedBytes)
	}

	indexesToRemove := make([]int, 0)
	for i := range selected {
		candidate := &selected[i]
		indexesToRemove = append(indexesToRemove, candidate.indexes...)
	}
	sort.Ints(indexesToRemove)
	q.removeWindowLocked(indexesToRemove, false)

	snapshot := q.snapshotLocked()
	q.settings.telemetry.record(context.Background(), snapshot)
	return true, blocked
}

func (q *centralQueue) buildWindowCandidateLocked(routingKey []byte, now time.Time) (centralQueueWindowCandidate, bool) {
	nowUnixNano := now.UnixNano()
	candidate := centralQueueWindowCandidate{
		window: centralQueueWindow{
			routingKey: append([]byte(nil), routingKey...),
			items:      make([]centralQueueItem, 0, 1),
		},
		indexes: make([]int, 0, 1),
	}

	blockedByHardLimit := false
	for i, item := range q.items {
		if !bytes.Equal(item.routingKey, routingKey) || item.nextAttemptUnixNano > nowUnixNano {
			continue
		}
		if len(candidate.window.items) > 0 && q.windowWouldExceedLimit(candidate.window, item) {
			blockedByHardLimit = true
			break
		}
		candidate.window.items = append(candidate.window.items, item)
		candidate.indexes = append(candidate.indexes, i)
		candidate.window.compressedBytes += item.compressedBytes
		candidate.window.uncompressedBytes += item.uncompressedBytes
		candidate.window.count += item.count
		if item.attempt > candidate.window.maxAttempt {
			candidate.window.maxAttempt = item.attempt
		}
		if candidate.window.oldestEnqueuedAt == 0 || item.enqueuedAtUnixNano < candidate.window.oldestEnqueuedAt {
			candidate.window.oldestEnqueuedAt = item.enqueuedAtUnixNano
		}
		if int64(candidate.window.compressedBytes) >= q.settings.targetCompressedBytes {
			candidate.window.flushReason = centralQueueFlushReasonTargetReached
			return candidate, true
		}
	}

	if len(candidate.window.items) == 0 {
		return centralQueueWindowCandidate{}, false
	}
	if q.stopped {
		candidate.window.flushReason = centralQueueFlushReasonShutdown
		return candidate, true
	}
	if blockedByHardLimit {
		candidate.window.flushReason = centralQueueFlushReasonHardCap
		return candidate, true
	}
	if q.settings.maxBatchDelay <= 0 {
		candidate.window.flushReason = centralQueueFlushReasonMaxDelayLowTraffic
		return candidate, true
	}
	oldest := time.Unix(0, candidate.window.oldestEnqueuedAt)
	if !oldest.IsZero() && now.Sub(oldest) >= q.settings.maxBatchDelay {
		candidate.window.flushReason = centralQueueFlushReasonMaxDelayLowTraffic
		return candidate, true
	}
	return centralQueueWindowCandidate{}, false
}

func (q *centralQueue) windowWouldExceedLimit(window centralQueueWindow, item centralQueueItem) bool {
	return q.settings.maxUncompressedBatchBytes > 0 &&
		window.uncompressedBytes+item.uncompressedBytes > q.settings.maxUncompressedBatchBytes
}

func (q *centralQueue) windowInflightBlockedLocked(window centralQueueWindow) bool {
	return q.windowInflightBlockedWithBase(window, q.currentInflightBytes)
}

func (q *centralQueue) windowInflightBlockedWithBase(window centralQueueWindow, inflightBytes int64) bool {
	return q.settings.maxInflightUncompressedBytes > 0 &&
		inflightBytes+int64(window.uncompressedBytes) > q.settings.maxInflightUncompressedBytes
}

func (q *centralQueue) leaseReadyWindowLocked() *centralQueueLease {
	if len(q.ready) == 0 {
		return nil
	}

	window := q.ready[0]
	copy(q.ready, q.ready[1:])
	q.ready[len(q.ready)-1] = centralQueueWindow{}
	q.ready = q.ready[:len(q.ready)-1]
	for _, item := range window.items {
		q.untrackOldestEnqueuedAtLocked(item)
	}
	snapshot := q.snapshotLocked()
	q.settings.telemetry.record(context.Background(), snapshot)
	q.settings.telemetry.recordWindow(context.Background(), window, q.settings.targetCompressedBytes)
	lease := &centralQueueLease{
		queue:  q,
		window: window,
	}
	if len(window.items) > 0 {
		lease.item = window.items[0]
	}
	return lease
}

func (q *centralQueue) removeItemLocked(index int) {
	removed := q.items[index]
	q.items = removeCentralQueueItem(q.items, index)
	q.untrackOldestEnqueuedAtLocked(removed)
}

func (q *centralQueue) removeWindowLocked(indexes []int, untrack bool) {
	for i := len(indexes) - 1; i >= 0; i-- {
		if untrack {
			q.removeItemLocked(indexes[i])
			continue
		}
		q.items = removeCentralQueueItem(q.items, indexes[i])
	}
}

func removeCentralQueueItem(items []centralQueueItem, index int) []centralQueueItem {
	copy(items[index:], items[index+1:])
	items[len(items)-1] = centralQueueItem{}
	return items[:len(items)-1]
}

func (l *centralQueueLease) done() {
	l.once.Do(func() {
		l.queue.mu.Lock()
		l.queue.currentInflightBytes -= int64(l.window.uncompressedBytes)
		l.queue.currentCompressedBytes -= int64(l.window.compressedBytes)
		snapshot := l.queue.snapshotLocked()
		l.queue.mu.Unlock()
		l.queue.settings.telemetry.record(context.Background(), snapshot)
	})
}

func (l *centralQueueLease) requeue(now time.Time) error {
	var err error
	l.once.Do(func() {
		l.queue.settings.telemetry.recordRetry(context.Background())

		l.queue.mu.Lock()
		l.queue.currentInflightBytes -= int64(l.window.uncompressedBytes)
		if l.queue.stopped {
			l.queue.currentCompressedBytes -= int64(l.window.compressedBytes)
			err = errCentralQueueStopped
		} else {
			for _, item := range l.window.items {
				nextAttempt := now.Add(centralQueueRetryDelayWithJitter(item))
				item.attempt++
				item.nextAttemptUnixNano = nextAttempt.UnixNano()
				l.queue.items = append(l.queue.items, item)
				l.queue.trackOldestEnqueuedAtLocked(item)
			}
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
	q.settings.telemetry.stopObservingOldestItemAge()
	q.settings.telemetry.stopObservingSchedulerState()
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
	return len(q.items) + q.readyItemCountLocked()
}

func (q *centralQueue) snapshotLocked() centralQueueSnapshot {
	return q.snapshotLockedAt(time.Now())
}

func (q *centralQueue) snapshotLockedAt(now time.Time) centralQueueSnapshot {
	return centralQueueSnapshot{
		compressedBytes:              q.currentCompressedBytes,
		compressedCapacity:           q.settings.maxCompressedBytes,
		items:                        int64(len(q.items) + q.readyItemCountLocked()),
		inflightUncompressed:         q.currentInflightBytes,
		inflightUncompressedCapacity: q.settings.maxInflightUncompressedBytes,
		oldestItemAgeMillis:          q.oldestItemAgeMillisLocked(now),
	}
}

func (q *centralQueue) readyItemCountLocked() int {
	count := 0
	for _, window := range q.ready {
		count += len(window.items)
	}
	return count
}

func (q *centralQueue) schedulerSnapshot() centralQueueSchedulerSnapshot {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.schedulerSnapshotLocked(time.Now())
}

func (q *centralQueue) schedulerSnapshotLocked(now time.Time) centralQueueSchedulerSnapshot {
	snapshot := centralQueueSchedulerSnapshot{
		readyWindows:     int64(len(q.ready)),
		readyWindowLimit: int64(q.settings.maxReadyWindows),
		state:            centralQueueSchedulerStateWaiting,
	}
	for _, window := range q.ready {
		snapshot.readyUncompressed += int64(window.uncompressedBytes)
	}
	switch {
	case len(q.ready) >= q.settings.maxReadyWindows:
		snapshot.state = centralQueueSchedulerStateReadyWindowLimit
	case len(q.ready) > 0:
		snapshot.state = centralQueueSchedulerStateReady
	case len(q.items) == 0 && q.stopped:
		snapshot.state = centralQueueSchedulerStateStopped
	case len(q.items) == 0:
		snapshot.state = centralQueueSchedulerStateQueueEmpty
	default:
		targetCandidates, fallbackCandidates, hasReady := q.collectWindowCandidatesLocked(now)
		if !hasReady || len(targetCandidates)+len(fallbackCandidates) == 0 {
			snapshot.state = centralQueueSchedulerStateWaiting
			return snapshot
		}
		for i := range targetCandidates {
			candidate := &targetCandidates[i]
			if !q.windowInflightBlockedLocked(candidate.window) {
				snapshot.state = centralQueueSchedulerStateReady
				return snapshot
			}
		}
		if len(targetCandidates) > 0 {
			snapshot.state = centralQueueSchedulerStateInflightBytes
			return snapshot
		}
		for i := range fallbackCandidates {
			candidate := &fallbackCandidates[i]
			if !q.windowInflightBlockedLocked(candidate.window) {
				snapshot.state = centralQueueSchedulerStateReady
				return snapshot
			}
		}
		snapshot.state = centralQueueSchedulerStateInflightBytes
	}
	return snapshot
}

func (q *centralQueue) oldestItemAgeMillis() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.oldestItemAgeMillisLocked(time.Now())
}

func (q *centralQueue) oldestItemAgeMillisLocked(now time.Time) int64 {
	q.pruneOldestEnqueuedAtLocked()
	if len(q.oldestEnqueuedAt) == 0 {
		return 0
	}
	age := now.Sub(time.Unix(0, q.oldestEnqueuedAt[0]))
	if age <= 0 {
		return 0
	}
	return age.Milliseconds()
}

func (q *centralQueue) trackOldestEnqueuedAtLocked(item centralQueueItem) {
	if item.enqueuedAtUnixNano == 0 {
		return
	}
	if q.enqueuedAtCounts == nil {
		q.enqueuedAtCounts = map[int64]int{}
	}
	if q.enqueuedAtHeapEntries == nil {
		q.enqueuedAtHeapEntries = map[int64]struct{}{}
	}
	if _, ok := q.enqueuedAtHeapEntries[item.enqueuedAtUnixNano]; !ok {
		heap.Push(&q.oldestEnqueuedAt, item.enqueuedAtUnixNano)
		q.enqueuedAtHeapEntries[item.enqueuedAtUnixNano] = struct{}{}
	}
	q.enqueuedAtCounts[item.enqueuedAtUnixNano]++
}

func (q *centralQueue) untrackOldestEnqueuedAtLocked(item centralQueueItem) {
	if item.enqueuedAtUnixNano == 0 || q.enqueuedAtCounts == nil {
		return
	}
	count := q.enqueuedAtCounts[item.enqueuedAtUnixNano]
	if count <= 1 {
		delete(q.enqueuedAtCounts, item.enqueuedAtUnixNano)
	} else {
		q.enqueuedAtCounts[item.enqueuedAtUnixNano] = count - 1
	}
	q.pruneOldestEnqueuedAtLocked()
}

func (q *centralQueue) pruneOldestEnqueuedAtLocked() {
	for len(q.oldestEnqueuedAt) > 0 && q.enqueuedAtCounts[q.oldestEnqueuedAt[0]] == 0 {
		enqueuedAt := heap.Pop(&q.oldestEnqueuedAt).(int64)
		delete(q.enqueuedAtHeapEntries, enqueuedAt)
	}
}

type centralQueueEnqueuedAtHeap []int64

func (h centralQueueEnqueuedAtHeap) Len() int {
	return len(h)
}

func (h centralQueueEnqueuedAtHeap) Less(i, j int) bool {
	return h[i] < h[j]
}

func (h centralQueueEnqueuedAtHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *centralQueueEnqueuedAtHeap) Push(x any) {
	*h = append(*h, x.(int64))
}

func (h *centralQueueEnqueuedAtHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func centralQueueRetryDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	shift := min(attempt, 6)
	delay := centralQueueInitialRetryDelay * time.Duration(1<<shift)
	return min(delay, centralQueueMaxRetryDelay)
}

func centralQueueRetryDelayWithJitter(item centralQueueItem) time.Duration {
	delay := centralQueueRetryDelay(item.attempt)
	jitterLimit := centralQueueRetryJitterLimit(delay)
	if jitterLimit <= 0 {
		return delay
	}
	hashInput := make([]byte, len(item.routingKey)+16)
	copy(hashInput, item.routingKey)
	binary.BigEndian.PutUint64(hashInput[len(item.routingKey):], uint64(item.enqueuedAtUnixNano))
	binary.BigEndian.PutUint64(hashInput[len(item.routingKey)+8:], uint64(item.attempt))
	jitter := time.Duration(crc32.ChecksumIEEE(hashInput) % uint32(jitterLimit+1))
	return delay + jitter
}

func centralQueueRetryJitterLimit(delay time.Duration) time.Duration {
	return min(delay/10, centralQueueMaxRetryJitter)
}

func centralQueueLaneRoutingKey(signal signalKind, routingKey []byte, laneCount int) []byte {
	if laneCount <= 0 {
		return append([]byte(nil), routingKey...)
	}
	hashInput := make([]byte, len(routingKey)+len(signal)+1)
	copy(hashInput, string(signal))
	hashInput[len(signal)] = 0
	copy(hashInput[len(signal)+1:], routingKey)
	lane := crc32.ChecksumIEEE(hashInput) % uint32(laneCount)
	laneRoutingKey := make([]byte, len(signal)+1+4)
	copy(laneRoutingKey, string(signal))
	laneRoutingKey[len(signal)] = 0
	binary.BigEndian.PutUint32(laneRoutingKey[len(signal)+1:], lane)
	return laneRoutingKey
}
