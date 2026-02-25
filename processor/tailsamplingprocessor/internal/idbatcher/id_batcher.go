// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package idbatcher defines a pipeline of fixed size in which the
// elements are batches of ids.
package idbatcher // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/idbatcher"

import (
	"errors"
	"math"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// ErrInvalidNumBatches occurs when an invalid number of batches is specified.
var ErrInvalidNumBatches = errors.New("invalid number of batches, it must be greater than zero")

// Batch is the type of batches held by the Batcher. It uses a set in order to merge batches efficiently.
type Batch map[pcommon.TraceID]struct{}

// Batcher behaves like a pipeline of batches that has a fixed number of batches in the pipe
// and a new batch being built outside of the pipe. Items can be concurrently added to the batch
// currently being built. When the batch being built is closed, the oldest batch in the pipe
// is pushed out so the one just closed can be put on the end of the pipe (this is done as an
// atomic operation). The caller is in control of when a batch is completed and a new one should
// be started.
type Batcher interface {
	// AddToCurrentBatch puts the given id on the batch being currently built. The client is in charge
	// of limiting the growth of the current batch if appropriate for its scenario. It can
	// either call CloseCurrentAndTakeFirstBatch earlier or stop adding new items depending on what is
	// required by the scenario.
	AddToCurrentBatch(id pcommon.TraceID) uint64

	// MoveToEarlierBatch tries to move the trace from the current batch to a
	// batch that is only a few batches from now. If the current batch will be
	// processed before the proposed batch it will do nothing. Returns the
	// batch that the trace will now be a part of (which may stay the same).
	MoveToEarlierBatch(id pcommon.TraceID, traceCurrentBatch, batchesFromNow uint64) uint64

	// RemoveFromBatch will remove the trace from the given batch.
	// If the batch is not in the range of batches then it is a noop.
	RemoveFromBatch(id pcommon.TraceID, batch uint64)

	// CloseCurrentAndTakeFirstBatch takes the batch at the front of the pipe, and moves the current
	// batch to the end of the pipe, creating a new batch to receive new items. This operation should
	// be atomic.
	// It returns the batch that was in front of the pipe and a boolean that if true indicates that
	// there are more batches to be retrieved.
	CloseCurrentAndTakeFirstBatch() (Batch, bool)

	// Stop informs that no more items are going to be batched and the pipeline can be read until it
	// is empty. After this method is called attempts to enqueue new items will panic.
	Stop()
}

var _ Batcher = (*batcher)(nil)

type batcher struct {
	takeID  uint64
	batches []Batch

	// mux protects any batch storing/moving ids.
	mux          sync.Mutex
	currentBatch Batch

	newBatchesInitialCapacity uint64
	lastBatchID               uint64
	stopped                   bool
}

// New creates a Batcher that will hold numBatches in its pipeline, having a channel with
// batchChannelSize to receive new items. New batches will be created with capacity set to
// newBatchesInitialCapacity.
func New(numBatches, newBatchesInitialCapacity uint64) (Batcher, error) {
	if numBatches < 1 {
		return nil, ErrInvalidNumBatches
	}
	if newBatchesInitialCapacity == 0 {
		// Always allocate a small map rather than sending a size hint of 0.
		// As the batcher runs it will allocate based on previous batch sizes.
		newBatchesInitialCapacity = 10
	}

	batcher := &batcher{
		batches:                   make([]Batch, numBatches),
		currentBatch:              make(Batch, newBatchesInitialCapacity),
		newBatchesInitialCapacity: newBatchesInitialCapacity,
		lastBatchID:               math.MaxUint64,
	}

	return batcher, nil
}

func (b *batcher) AddToCurrentBatch(id pcommon.TraceID) uint64 {
	b.mux.Lock()
	defer b.mux.Unlock()

	b.currentBatch[id] = struct{}{}
	return b.takeID + uint64(len(b.batches))
}

func (b *batcher) MoveToEarlierBatch(id pcommon.TraceID, traceCurrentBatch, batchesFromNow uint64) uint64 {
	b.mux.Lock()
	defer b.mux.Unlock()

	proposedBatch := b.takeID + batchesFromNow
	// Only move the batch if it is earlier.
	if proposedBatch >= traceCurrentBatch {
		return traceCurrentBatch
	}

	// Check if the trace's batch is the batch currently being added to.
	currentBatchID := b.takeID + uint64(len(b.batches))
	if traceCurrentBatch == currentBatchID {
		delete(b.currentBatch, id)
	} else {
		currentIdx := traceCurrentBatch % uint64(len(b.batches))
		delete(b.batches[currentIdx], id)
	}

	proposedIdx := proposedBatch % uint64(len(b.batches))
	if b.batches[proposedIdx] == nil {
		b.batches[proposedIdx] = make(Batch, b.newBatchesInitialCapacity)
	}
	b.batches[proposedIdx][id] = struct{}{}
	return proposedBatch
}

func (b *batcher) RemoveFromBatch(id pcommon.TraceID, batch uint64) {
	b.mux.Lock()
	defer b.mux.Unlock()

	currentBatchID := b.takeID + uint64(len(b.batches))
	if batch == currentBatchID {
		delete(b.currentBatch, id)
	} else if batch >= b.takeID && batch < currentBatchID {
		delete(b.batches[batch%uint64(len(b.batches))], id)
	}
	// Nothing to remove if we are outside of the batch range.
}

func (b *batcher) CloseCurrentAndTakeFirstBatch() (Batch, bool) {
	b.mux.Lock()
	defer b.mux.Unlock()

	if b.takeID < b.lastBatchID {
		takeIdx := b.takeID % uint64(len(b.batches))
		readBatch := b.batches[takeIdx]

		if !b.stopped {
			nextBatch := make(Batch, max(b.newBatchesInitialCapacity, uint64(len(readBatch))))
			b.batches[takeIdx] = b.currentBatch
			b.currentBatch = nextBatch
		}
		b.takeID++
		return readBatch, true
	}

	readBatch := b.currentBatch
	b.currentBatch = nil
	return readBatch, false
}

func (b *batcher) Stop() {
	b.mux.Lock()
	defer b.mux.Unlock()

	b.stopped = true
	b.lastBatchID = b.takeID + uint64(len(b.batches))
}
