// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/axiomhq/hyperloglog"
	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

const rotationBenchmarkTrackerCount = 256

func BenchmarkCardinalityProcessorRotate_Sparse(b *testing.B) {
	benchmarkCardinalityProcessorRotate(b, false)
}

func BenchmarkCardinalityProcessorRotate_Dense(b *testing.B) {
	benchmarkCardinalityProcessorRotate(b, true)
}

func benchmarkCardinalityProcessorRotate(b *testing.B, dense bool) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1000000,
		EpochDurationSeconds:        86400,
		TopOffendersCount:           0,
	}
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(b.Context(), cfg, set, consumertest.NewNop())
	if err != nil {
		b.Fatal(err)
	}
	p := proc.(*cardinalityProcessor)
	if err := p.Start(b.Context(), nil); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := p.Shutdown(b.Context()); err != nil {
			b.Errorf("shutdown benchmark processor: %v", err)
		}
	})

	trackers := make([]*tracker, rotationBenchmarkTrackerCount)
	valuesPerSketch := 32
	if dense {
		valuesPerSketch = 20000
	}

	for i := range trackers {
		tracker := newTracker()
		for value := range valuesPerSketch {
			hashValue := rotationBenchmarkHash(i, value)
			tracker.current.InsertHash(hashValue)
			tracker.previous.InsertHash(hashValue)
		}
		verifyRotationBenchmarkSketch(b, tracker.current, dense)
		verifyRotationBenchmarkSketch(b, tracker.previous, dense)

		key := trackerKey{
			metricName: fmt.Sprintf("rotation_benchmark_metric_%d", i),
			attrKey:    "attribute",
		}
		shard := p.shards[i%numShards]
		shard.trackers[key] = tracker
		trackers[i] = tracker
	}
	p.trackerCount.Store(int64(len(trackers)))

	// Align the initial state so the measured iterations start after a complete
	// epoch boundary. For the dense case this also makes both alternating
	// sketches dense, so Reset's dense clear path is exercised consistently.
	p.rotate()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		// Keep every tracker active without including the insert cost in the
		// rotation measurement. This prevents stale eviction from changing the
		// benchmark after two iterations.
		b.StopTimer()
		for trackerIndex, tracker := range trackers {
			tracker.insert(rotationBenchmarkHash(i, trackerIndex))
		}
		b.StartTimer()
		p.rotate()
	}
}

// rotationBenchmarkHash matches the processor's use of InsertHash: the input
// must be uniformly distributed, rather than a sequential counter.
func rotationBenchmarkHash(a, b int) uint64 {
	var input [16]byte
	binary.LittleEndian.PutUint64(input[:8], uint64(a))
	binary.LittleEndian.PutUint64(input[8:], uint64(b))
	return xxhash.Sum64(input[:])
}

func verifyRotationBenchmarkSketch(tb testing.TB, sketch *hyperloglog.Sketch, wantDense bool) {
	tb.Helper()
	data, err := sketch.MarshalBinary()
	require.NoError(tb, err, "marshal benchmark sketch")
	require.GreaterOrEqual(tb, len(data), 4, "marshal benchmark sketch returned too few bytes")

	wantRepresentation := byte(1) // sparse
	if wantDense {
		wantRepresentation = 0 // dense
	}
	require.Equal(tb, wantRepresentation, data[3], "benchmark sketch representation")
	if wantDense {
		require.Equal(tb, 8+(1<<14), len(data), "dense benchmark sketch marshal size")
	}
}

func TestRotationBenchmarkSketchMarshal(t *testing.T) {
	for _, test := range []struct {
		name            string
		valuesPerSketch int
		wantDense       bool
	}{
		{name: "sparse", valuesPerSketch: 32},
		{name: "dense", valuesPerSketch: 20000, wantDense: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			sketch := hyperloglog.New14()
			for value := range test.valuesPerSketch {
				sketch.InsertHash(rotationBenchmarkHash(0, value))
			}

			verifyRotationBenchmarkSketch(t, sketch, test.wantDense)
			data, err := sketch.MarshalBinary()
			require.NoError(t, err)

			var roundTrip hyperloglog.Sketch
			require.NoError(t, roundTrip.UnmarshalBinary(data))
			require.Equal(t, sketch.Estimate(), roundTrip.Estimate())
		})
	}
}
