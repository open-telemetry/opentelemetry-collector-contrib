// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package idbatcher

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestBatcherNew(t *testing.T) {
	tests := []struct {
		name                      string
		numBatches                uint64
		newBatchesInitialCapacity uint64
		wantErr                   error
	}{
		{"invalid numBatches", 0, 0, ErrInvalidNumBatches},
		{"valid", 1, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.numBatches, tt.newBatchesInitialCapacity)
			require.ErrorIs(t, err, tt.wantErr)
			if got != nil {
				got.Stop()
			}
		})
	}
}

func TestTypicalConfig(t *testing.T) {
	concurrencyTest(t, 10, 100)
}

func TestMinBufferedChannels(t *testing.T) {
	concurrencyTest(t, 1, 0)
}

func BenchmarkConcurrentEnqueue(b *testing.B) {
	ids := generateSequentialIDs(1)
	batcher, err := New(10, 100)
	require.NoError(b, err, "Failed to create Batcher")

	ticker := time.NewTicker(time.Millisecond)
	var wg sync.WaitGroup
	defer func() {
		batcher.Stop()
		wg.Wait()
		ticker.Stop()
	}()
	ticked := &atomic.Int64{}
	received := &atomic.Int64{}
	wg.Go(func() {
		for range ticker.C {
			batch, more := batcher.CloseCurrentAndTakeFirstBatch()
			ticked.Add(1)
			received.Add(int64(len(batch)))
			if !more {
				return
			}
		}
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			batcher.AddToCurrentBatch(ids[0])
		}
	})
}

func concurrencyTest(t *testing.T, numBatches, newBatchesInitialCapacity uint64) {
	batcher, err := New(numBatches, newBatchesInitialCapacity)
	require.NoError(t, err, "Failed to create Batcher: %v", err)

	ticker := time.NewTicker(time.Millisecond)
	got := Batch{}
	duplicateIDs := 0
	asyncDequesComplete := make(chan bool)
	go func() {
		defer func() {
			asyncDequesComplete <- true
		}()

		var completedDequeues uint64
		for range ticker.C {
			g, more := batcher.CloseCurrentAndTakeFirstBatch()
			completedDequeues++
			if completedDequeues <= numBatches && len(g) != 0 {
				t.Error("Some of the first batches were not empty")
				return
			}
			for id := range g {
				if _, ok := got[id]; ok {
					duplicateIDs++
				}
				got[id] = struct{}{}
			}
			if !more {
				return
			}
		}
	}()

	ids := generateSequentialIDs(10000)
	wg := &sync.WaitGroup{}
	// Limit the concurrency here to avoid creating too many goroutines and hit
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9126
	concurrencyLimiter := make(chan struct{}, 128)
	defer close(concurrencyLimiter)
	for i := range ids {
		wg.Add(1)
		concurrencyLimiter <- struct{}{}
		go func(id pcommon.TraceID) {
			batcher.AddToCurrentBatch(id)
			wg.Done()
			<-concurrencyLimiter
		}(ids[i])
	}

	wg.Wait()
	batcher.Stop()
	// Wait for async process to be complete which will process all traces.
	<-asyncDequesComplete
	ticker.Stop()

	require.Zero(t, duplicateIDs, "Found duplicate ids in batcher")
	require.Len(t, got, len(ids), "Batcher got incorrect count of traces from batches")

	for _, id := range ids {
		_, ok := got[id]
		require.True(t, ok, "want id %v but id was not seen", id)
	}
}

func generateSequentialIDs(numIDs uint64) []pcommon.TraceID {
	ids := make([]pcommon.TraceID, numIDs)
	for i := range numIDs {
		traceID := [16]byte{}
		binary.BigEndian.PutUint64(traceID[:8], 0)
		binary.BigEndian.PutUint64(traceID[8:], i)
		ids[i] = pcommon.TraceID(traceID)
	}
	return ids
}
