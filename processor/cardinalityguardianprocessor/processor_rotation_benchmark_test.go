// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"fmt"
	"hash/maphash"
	"testing"

	"go.uber.org/zap"
)

const rotationBenchmarkTrackerCount = 256

func BenchmarkCardinalityProcessorRotate_Sparse(b *testing.B) {
	benchmarkCardinalityProcessorRotate(b, false)
}

func BenchmarkCardinalityProcessorRotate_Dense(b *testing.B) {
	benchmarkCardinalityProcessorRotate(b, true)
}

func benchmarkCardinalityProcessorRotate(b *testing.B, dense bool) {
	p, trackers := newRotationBenchmarkProcessor(dense)

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
			tracker.insert(uint64(i) + uint64(trackerIndex))
		}
		b.StartTimer()
		p.rotate()
	}
}

func newRotationBenchmarkProcessor(dense bool) (*cardinalityProcessor, []*tracker) {
	p := &cardinalityProcessor{
		config: &Config{
			MaxCardinalityDeltaPerEpoch: 1000000,
			TopOffendersCount:           0,
		},
		logger: zap.NewNop(),
		seed:   maphash.MakeSeed(),
	}
	for i := range p.shards {
		p.shards[i] = &trackerShard{trackers: make(map[trackerKey]*tracker)}
	}

	trackers := make([]*tracker, rotationBenchmarkTrackerCount)
	valuesPerSketch := 32
	if dense {
		valuesPerSketch = 20000
	}

	for i := range trackers {
		tracker := newTracker()
		for value := range valuesPerSketch {
			hashValue := uint64(i)<<32 | uint64(value)
			tracker.current.InsertHash(hashValue)
			if dense {
				tracker.previous.InsertHash(hashValue)
			}
		}

		key := trackerKey{
			metricName: fmt.Sprintf("rotation_benchmark_metric_%d", i),
			attrKey:    "attribute",
		}
		shard := p.shards[i%numShards]
		shard.trackers[key] = tracker
		trackers[i] = tracker
	}
	p.trackerCount.Store(int64(len(trackers)))

	return p, trackers
}
