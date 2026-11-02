// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TestPartitionMailboxEnqueue proves enqueue accepts a batch, pauses when full, or rejects a rewind.
func TestPartitionMailboxEnqueue(t *testing.T) {
	cases := []struct {
		name              string
		capacity          int
		maxBytes          int
		initialOffset     *int
		pendingRewind     *int
		cancel            bool
		batch             kgo.FetchTopicPartition
		wantAccepted      bool
		wantPauseCalls    int
		wantPauseReason   partitionPauseReason
		wantPendingOffset *int64
		wantQueuedBytes   int
	}{
		{
			name:         "accepts batch",
			capacity:     2,
			batch:        mailboxBatch(10),
			wantAccepted: true,
		},
		{
			name:            "pauses when accepted batch fills mailbox",
			capacity:        1,
			batch:           mailboxBatch(10),
			wantAccepted:    true,
			wantPauseCalls:  1,
			wantPauseReason: partitionPauseBackpressure,
		},
		{
			name:              "rejects full mailbox",
			capacity:          1,
			initialOffset:     new(5),
			batch:             mailboxBatch(10),
			wantPauseCalls:    1,
			wantPauseReason:   partitionPauseRewind,
			wantPendingOffset: new(int64(10)),
		},
		{
			name:              "rejects while rewind pending",
			capacity:          2,
			pendingRewind:     new(5),
			batch:             mailboxBatch(10),
			wantPauseCalls:    1,
			wantPauseReason:   partitionPauseRewind,
			wantPendingOffset: new(int64(5)),
		},
		{
			name:         "rejects after cancellation",
			capacity:     1,
			cancel:       true,
			batch:        mailboxBatch(10),
			wantAccepted: false,
		},
		{
			name:            "pauses when batch reaches byte cap",
			capacity:        4,
			maxBytes:        256,
			batch:           mailboxBatch(10),
			wantAccepted:    true,
			wantPauseCalls:  1,
			wantPauseReason: partitionPauseBackpressure,
			wantQueuedBytes: 256,
		},
		{
			name:            "does not pause below byte cap",
			capacity:        4,
			maxBytes:        512,
			batch:           mailboxBatch(10),
			wantAccepted:    true,
			wantQueuedBytes: 256,
		},
		{
			name:            "admits batch above byte cap into empty mailbox",
			capacity:        2,
			maxBytes:        100,
			batch:           mailboxBatch(10),
			wantAccepted:    true,
			wantPauseCalls:  1,
			wantPauseReason: partitionPauseBackpressure,
			wantQueuedBytes: 256,
		},
		{
			name:              "rejects batch while over byte cap",
			capacity:          4,
			maxBytes:          256,
			initialOffset:     new(5),
			batch:             mailboxBatch(10),
			wantPauseCalls:    1,
			wantPauseReason:   partitionPauseRewind,
			wantPendingOffset: new(int64(10)),
			wantQueuedBytes:   256,
		},
		{
			name:            "ignores queued bytes when byte cap is zero",
			capacity:        4,
			maxBytes:        0,
			initialOffset:   new(5),
			batch:           mailboxBatch(10),
			wantAccepted:    true,
			wantQueuedBytes: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			mailbox := newPartitionMailbox(ctx, tc.capacity, tc.maxBytes)
			if tc.initialOffset != nil {
				require.True(t, mailbox.enqueue(mailboxBatch(int64(*tc.initialOffset)), func(partitionPauseReason) {}))
			}
			if tc.pendingRewind != nil {
				mailbox.requestRewind(recordAtOffset(int64(*tc.pendingRewind)), false, func() {})
			}
			if tc.cancel {
				cancel()
			} else {
				defer cancel()
			}

			pauseCalls := 0
			var pauseReason partitionPauseReason
			accepted := mailbox.enqueue(tc.batch, func(reason partitionPauseReason) {
				pauseCalls++
				pauseReason = reason
			})

			require.Equal(t, tc.wantAccepted, accepted)
			require.Equal(t, tc.wantPauseCalls, pauseCalls)
			require.Equal(t, tc.wantPauseReason, pauseReason)
			var pendingOffset *int64
			if mailbox.hasPendingOffsetChange() {
				pendingOffset = new(mailbox.takeOffsetChange().Offset)
			}
			require.Equal(t, tc.wantPendingOffset, pendingOffset)
			require.Equal(t, tc.wantQueuedBytes, queuedBytes(t, mailbox))
		})
	}
}

// TestPartitionMailboxDequeue proves dequeue returns the first batch and resumes when below capacity.
func TestPartitionMailboxDequeue(t *testing.T) {
	cases := []struct {
		name            string
		capacity        int
		offsets         []int64
		pendingRewind   bool
		wantOffset      int64
		wantOK          bool
		wantResumeCalls int
	}{
		{
			name:     "empty mailbox",
			capacity: 1,
		},
		{
			name:            "returns first batch and resumes below capacity",
			capacity:        2,
			offsets:         []int64{5, 10},
			wantOffset:      5,
			wantOK:          true,
			wantResumeCalls: 1,
		},
		{
			name:          "does not resume while rewind pending",
			capacity:      2,
			offsets:       []int64{5, 10},
			pendingRewind: true,
			wantOffset:    5,
			wantOK:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mailbox := newPartitionMailbox(t.Context(), tc.capacity, 0)
			for _, offset := range tc.offsets {
				require.True(t, mailbox.enqueue(mailboxBatch(offset), func(partitionPauseReason) {}))
			}
			if tc.pendingRewind {
				mailbox.requestRewind(recordAtOffset(20), false, func() {})
			}

			resumeCalls := 0
			batch, ok := mailbox.dequeue(func() {
				resumeCalls++
			})

			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.wantResumeCalls, resumeCalls)
			if tc.wantOK {
				require.Equal(t, tc.wantOffset, batch.Records[0].Offset)
			}
		})
	}
}

// TestPartitionMailboxDequeueBytes proves dequeue subtracts each batch's stored
// size and resumes only once both caps have room.
func TestPartitionMailboxDequeueBytes(t *testing.T) {
	cases := []struct {
		name            string
		sizes           []int
		dequeues        int
		wantResumeCalls int
		wantQueuedBytes int
	}{
		{
			name:            "does not resume while bytes stay at cap",
			sizes:           []int{256, 768},
			dequeues:        1,
			wantResumeCalls: 0,
			wantQueuedBytes: 768,
		},
		{
			name:            "drains to zero bytes",
			sizes:           []int{256, 768},
			dequeues:        2,
			wantResumeCalls: 1,
			wantQueuedBytes: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Each batch is admitted while the queue is still below the 500-byte cap.
			mailbox := newPartitionMailbox(t.Context(), 4, 500)
			for _, size := range tc.sizes {
				require.True(t, mailbox.enqueue(sizedBatch(size), func(partitionPauseReason) {}))
			}

			resumeCalls := 0
			for range tc.dequeues {
				_, ok := mailbox.dequeue(func() {
					resumeCalls++
				})
				require.True(t, ok)
			}

			require.Equal(t, tc.wantResumeCalls, resumeCalls)
			require.Equal(t, tc.wantQueuedBytes, queuedBytes(t, mailbox))
		})
	}
}

// TestPartitionMailboxSerializesPauseBeforeResume proves resume waits until the pause callback ends.
func TestPartitionMailboxSerializesPauseBeforeResume(t *testing.T) {
	cases := []struct {
		name   string
		resume func(*partitionMailbox, chan struct{})
	}{
		{
			name: "dequeue",
			resume: func(mailbox *partitionMailbox, resumed chan struct{}) {
				_, _ = mailbox.dequeue(func() {
					close(resumed)
				})
			},
		},
		{
			name: "rewind resume",
			resume: func(mailbox *partitionMailbox, resumed chan struct{}) {
				mailbox.resumeAfterOffsetChange(func() {
					close(resumed)
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mailbox := newPartitionMailbox(t.Context(), 1, 0)
			pauseStarted := make(chan struct{})
			releasePause := make(chan struct{})
			resumed := make(chan struct{})

			enqueueDone := make(chan struct{})
			go func() {
				defer close(enqueueDone)
				mailbox.enqueue(mailboxBatch(1), func(partitionPauseReason) {
					close(pauseStarted)
					<-releasePause
				})
			}()
			<-pauseStarted

			go tc.resume(mailbox, resumed)

			require.Never(t, func() bool {
				select {
				case <-resumed:
					return true
				default:
					return false
				}
			}, 100*time.Millisecond, 10*time.Millisecond)

			close(releasePause)
			<-enqueueDone
			<-resumed
		})
	}
}

// TestPartitionMailboxRequestRewind proves rewind keeps the earliest offset and can clear the queue.
func TestPartitionMailboxRequestRewind(t *testing.T) {
	cases := []struct {
		name            string
		existing        *int
		requested       int64
		clearQueue      bool
		wantOffset      int64
		wantQueueSize   int
		wantQueuedBytes int
	}{
		{
			name:            "retains earlier existing offset",
			existing:        new(5),
			requested:       8,
			wantOffset:      5,
			wantQueueSize:   1,
			wantQueuedBytes: 256,
		},
		{
			name:            "replaces with earlier requested offset",
			existing:        new(8),
			requested:       5,
			wantOffset:      5,
			wantQueueSize:   1,
			wantQueuedBytes: 256,
		},
		{
			name:          "clears queued batches",
			requested:     5,
			clearQueue:    true,
			wantOffset:    5,
			wantQueueSize: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mailbox := newPartitionMailbox(t.Context(), 2, 1000)
			require.True(t, mailbox.enqueue(mailboxBatch(1), func(partitionPauseReason) {}))
			if tc.existing != nil {
				mailbox.requestRewind(recordAtOffset(int64(*tc.existing)), false, func() {})
			}

			pauseCalls := 0
			mailbox.requestRewind(recordAtOffset(tc.requested), tc.clearQueue, func() {
				pauseCalls++
			})

			require.Equal(t, 1, pauseCalls)
			require.True(t, mailbox.hasPendingOffsetChange())
			require.Equal(t, tc.wantOffset, mailbox.takeOffsetChange().Offset)
			require.Equal(t, tc.wantQueueSize, queued(t, mailbox))
			require.Equal(t, tc.wantQueuedBytes, queuedBytes(t, mailbox))
		})
	}
}

// TestPartitionMailboxDiscardResetsQueuedBytes proves discard drops the queued bytes with the batches.
func TestPartitionMailboxDiscardResetsQueuedBytes(t *testing.T) {
	mailbox := newPartitionMailbox(t.Context(), 2, 1000)
	require.True(t, mailbox.enqueue(sizedBatch(256), func(partitionPauseReason) {}))
	require.True(t, mailbox.enqueue(sizedBatch(256), func(partitionPauseReason) {}))
	require.Equal(t, 512, queuedBytes(t, mailbox))

	mailbox.discard()

	require.Equal(t, 0, queued(t, mailbox))
	require.Equal(t, 0, queuedBytes(t, mailbox))
}

func queuedBytes(t *testing.T, m *partitionMailbox) int {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.queuedBytes
}

func queued(t *testing.T, m *partitionMailbox) int {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.batches)
}

func mailboxBatch(offset int64) kgo.FetchTopicPartition {
	return kgo.FetchTopicPartition{
		FetchPartition: kgo.FetchPartition{
			Records: []*kgo.Record{recordAtOffset(offset)},
		},
	}
}

// sizedBatch returns a one-record batch whose inflightSize is size.
func sizedBatch(size int) kgo.FetchTopicPartition {
	rec := recordAtOffset(0)
	rec.Value = make([]byte, size-recordOverhead)
	return kgo.FetchTopicPartition{
		FetchPartition: kgo.FetchPartition{
			Records: []*kgo.Record{rec},
		},
	}
}

func recordAtOffset(offset int64) *kgo.Record {
	return &kgo.Record{
		Offset: offset,
	}
}
