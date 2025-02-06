// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package idbatcher

import (
	"encoding/binary"
	"runtime"
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
		batchChannelSize          uint64
		wantErr                   error
	}{
		{"invalid numBatches", 0, 0, 1, ErrInvalidNumBatches},
		{"invalid batchChannelSize", 1, 0, 0, ErrInvalidBatchChannelSize},
		{"valid", 1, 0, 1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.numBatches, tt.newBatchesInitialCapacity, tt.batchChannelSize)
			require.ErrorIs(t, err, tt.wantErr)
			if got != nil {
				got.Stop()
			}
		})
	}
}

func TestTypicalConfig(t *testing.T) {
	concurrencyTest(t, 10, 100, uint64(4*runtime.NumCPU()))
}

func TestMinBufferedChannels(t *testing.T) {
	concurrencyTest(t, 1, 0, 1)
}

func BenchmarkConcurrentEnqueue(b *testing.B) {
	ids := generateSequentialIDs(1)
	batcher, err := New(10, 100, uint64(4*runtime.NumCPU()))
	defer batcher.Stop()
	require.NoError(b, err, "Failed to create Batcher")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	ticked := &atomic.Int64{}
	received := &atomic.Int64{}
	go func() {
		for range ticker.C {
			batch, _ := batcher.CloseCurrentAndTakeFirstBatch()
			ticked.Add(1)
			received.Add(int64(len(batch)))
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			batcher.AddToCurrentBatch(ids[0])
		}
	})
}

func concurrencyTest(t *testing.T, numBatches, newBatchesInitialCapacity, batchChannelSize uint64) {
	batcher, err := New(numBatches, newBatchesInitialCapacity, batchChannelSize)
	require.NoError(t, err, "Failed to create Batcher: %v", err)

	ticker := time.NewTicker(100 * time.Millisecond)
	stopTicker := make(chan bool)
	var got Batch
	go func() {
		var completedDequeues uint64
	outer:
		for {
			select {
			case <-ticker.C:
				g, _ := batcher.CloseCurrentAndTakeFirstBatch()
				completedDequeues++
				if completedDequeues <= numBatches && len(g) != 0 {
					t.Error("Some of the first batches were not empty")
					return
				}
				got = append(got, g...)
			case <-stopTicker:
				break outer
			}
		}
	}()

	ids := generateSequentialIDs(10000)
	wg := &sync.WaitGroup{}
	// Limit the concurrency here to avoid creating too many goroutines and hit
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9126
	concurrencyLimiter := make(chan struct{}, 128)
	defer close(concurrencyLimiter)
	for i := 0; i < len(ids); i++ {
		wg.Add(1)
		concurrencyLimiter <- struct{}{}
		go func(id pcommon.TraceID) {
			batcher.AddToCurrentBatch(id)
			wg.Done()
			<-concurrencyLimiter
		}(ids[i])
	}

	wg.Wait()
	stopTicker <- true
	ticker.Stop()
	batcher.Stop()

	// Get all ids added to the batcher
	for {
		batch, ok := batcher.CloseCurrentAndTakeFirstBatch()
		got = append(got, batch...)
		if !ok {
			break
		}
	}

	require.Equal(t, len(ids), len(got), "Batcher got incorrect count of traces from batches")

	idSeen := make(map[[16]byte]bool, len(ids))
	for _, id := range got {
		idSeen[id] = true
	}

	for i := 0; i < len(ids); i++ {
		require.True(t, idSeen[ids[i]], "want id %v but id was not seen", ids[i])
	}
}

func generateSequentialIDs(numIDs uint64) []pcommon.TraceID {
	ids := make([]pcommon.TraceID, numIDs)
	for i := uint64(0); i < numIDs; i++ {
		traceID := [16]byte{}
		binary.BigEndian.PutUint64(traceID[:8], 0)
		binary.BigEndian.PutUint64(traceID[8:], i)
		ids[i] = pcommon.TraceID(traceID)
	}
	return ids
}
