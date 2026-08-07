// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"
)

// partitionBatch keeps the Kafka fetch and the partition generation together.
// The generation prevents records fetched before a rewind from being queued
// after the worker has reset that partition.
type partitionBatch struct {
	fetch      kgo.FetchTopicPartition
	generation uint64
}

// partitionMailbox stores fetched batches waiting for one partition worker.
// It also coordinates rewinds so batches fetched before a rewind are rejected.
type partitionMailbox struct {
	ctx context.Context

	mu sync.Mutex
	// batches is the bounded FIFO queue of batches waiting for the worker.
	batches []partitionBatch
	// notify wakes the worker when work or a rewind is available. Its single
	// buffered signal represents the state change, not each individual batch.
	notify chan struct{}
	// generation identifies the current fetch sequence. It increments after a
	// rewind so batches returned by a poll started before that rewind can be
	// recognized as stale and rejected.
	generation uint64
	// pendingRewind is the earliest record that the poll loop must fetch again.
	pendingRewind *kgo.Record
	// rewindInFlight prevents more than one control request for the same pending
	// rewind from being sent to the poll loop.
	rewindInFlight bool
}

// newPartitionMailbox creates a mailbox with capacity waiting batch slots.
func newPartitionMailbox(ctx context.Context, capacity int) *partitionMailbox {
	return &partitionMailbox{
		ctx:     ctx,
		batches: make([]partitionBatch, 0, capacity),
		notify:  make(chan struct{}, 1),
	}
}

// generationSnapshot returns the generation to attach to a poll started now.
func (m *partitionMailbox) generationSnapshot() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generation
}

// notifications returns the channel used to wake the partition worker.
func (m *partitionMailbox) notifications() <-chan struct{} {
	return m.notify
}

// enqueue adds a batch when it belongs to the active fetch generation.
func (m *partitionMailbox) enqueue(batch partitionBatch, pause func(partitionPauseReason)) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-m.ctx.Done():
		// The next partition owner will fetch again from the committed offset.
		return false
	default:
	}

	// PollRecords may return work from an older generation or after the mailbox
	// reaches capacity. Remember the earliest rejected record so it is fetched
	// again instead of being skipped.
	if batch.generation != m.generation || m.pendingRewind != nil || len(m.batches) == cap(m.batches) {
		m.requestRewindLocked(batch.fetch.Records[0], false)
		pause(partitionPauseRewind)
		return false
	}

	m.batches = append(m.batches, batch)
	m.notifyLocked()
	// Pause when the final slot is filled so franz-go discards any additional
	// records it buffered for this partition before another poll returns them.
	if len(m.batches) == cap(m.batches) {
		pause(partitionPauseBackpressure)
	}
	return true
}

// dequeue removes and returns the oldest waiting batch.
func (m *partitionMailbox) dequeue(resume func()) (partitionBatch, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.batches) == 0 {
		return partitionBatch{}, false
	}

	batch := m.batches[0]
	copy(m.batches, m.batches[1:])
	m.batches[len(m.batches)-1] = partitionBatch{}
	m.batches = m.batches[:len(m.batches)-1]
	// A pending rewind must move the fetch position before fetching resumes.
	// Calling resume under the lock also prevents it from overtaking an older
	// pause operation.
	if len(m.batches) < cap(m.batches) && m.pendingRewind == nil {
		resume()
	}
	return batch, true
}

// requestRewind records the earliest offset that must be fetched again,
// optionally discards queued batches, and pauses fetching.
func (m *partitionMailbox) requestRewind(record *kgo.Record, clearQueue bool, pause func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestRewindLocked(record, clearQueue)
	pause()
}

// requestRewindLocked updates rewind state while the caller holds m.mu.
func (m *partitionMailbox) requestRewindLocked(record *kgo.Record, clearQueue bool) {
	if clearQueue {
		clear(m.batches)
		m.batches = m.batches[:0]
	}
	// Multiple rejected fetches may arrive before the poll loop applies the
	// rewind. Keep the earliest offset so no rejected record is skipped.
	if m.pendingRewind == nil || record.Offset < m.pendingRewind.Offset {
		m.pendingRewind = record
	}
	m.notifyLocked()
}

// claimRewind marks the pending rewind as handed to the poll loop.
func (m *partitionMailbox) claimRewind() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// A notification does not reserve the rewind, so repeated worker wakeups
	// may reach this state. Only the first claim may send a control request.
	if m.pendingRewind == nil || m.rewindInFlight {
		return false
	}
	m.rewindInFlight = true
	return true
}

// applyRewind applies the pending offset change and starts a new generation.
func (m *partitionMailbox) applyRewind(apply func(*kgo.Record)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Treat an obsolete control as complete without changing the generation.
	if m.pendingRewind == nil {
		m.rewindInFlight = false
		return
	}
	apply(m.pendingRewind)
	m.pendingRewind = nil
	m.rewindInFlight = false
	m.generation++
}

// notifyLocked wakes the worker without blocking while the caller holds m.mu.
func (m *partitionMailbox) notifyLocked() {
	select {
	case m.notify <- struct{}{}:
	default:
	}
}
