// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
)

// BenchmarkProcessTrace_SmallTrace benchmarks processing a small trace (10 spans).
func BenchmarkProcessTrace_SmallTrace(b *testing.B) {
	benchmarkProcessTrace(b, 10, 5)
}

// BenchmarkProcessTrace_MediumTrace benchmarks processing a medium trace (100 spans).
func BenchmarkProcessTrace_MediumTrace(b *testing.B) {
	benchmarkProcessTrace(b, 100, 20)
}

// BenchmarkProcessTrace_LargeTrace benchmarks processing a large trace (1000 spans).
func BenchmarkProcessTrace_LargeTrace(b *testing.B) {
	benchmarkProcessTrace(b, 1000, 50)
}

// BenchmarkProcessTrace_SparseAggregation benchmarks sparse aggregation (~10% aggregate).
func BenchmarkProcessTrace_SparseAggregation(b *testing.B) {
	benchmarkProcessTraceSparse(b, 1000, 5)
}

// BenchmarkDeepTrace_Depth1 benchmarks deep trace with max_parent_depth=1.
func BenchmarkDeepTrace_Depth1(b *testing.B) {
	benchmarkDeepTrace(b, 20, 3, 5, 1000, 1)
}

// BenchmarkDeepTrace_Depth5 benchmarks deep trace with max_parent_depth=5.
func BenchmarkDeepTrace_Depth5(b *testing.B) {
	benchmarkDeepTrace(b, 20, 3, 5, 1000, 5)
}

// BenchmarkDeepTrace_Depth10 benchmarks deep trace with max_parent_depth=10.
func BenchmarkDeepTrace_Depth10(b *testing.B) {
	benchmarkDeepTrace(b, 20, 3, 5, 1000, MaxParentDepthLimit)
}

// BenchmarkBuildTraceTree benchmarks tree construction.
func BenchmarkBuildTraceTree(b *testing.B) {
	proc := newBenchmarkProcessor(b, 5)
	spans := generateTestSpans(1000, 50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = proc.buildTraceTree(spans)
	}
}

// BenchmarkGroupLeafNodes benchmarks leaf node grouping.
func BenchmarkGroupLeafNodes(b *testing.B) {
	proc := newBenchmarkProcessor(b, 5)
	spans := generateTestSpans(1000, 50)
	tree := proc.buildTraceTree(spans)
	leaves := tree.getLeaves()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, leaf := range leaves {
			leaf.groupKey = ""
		}
		_ = proc.groupLeafNodesByKey(leaves)
	}
}

// BenchmarkFindEligibleParents benchmarks parent candidate discovery.
func BenchmarkFindEligibleParents(b *testing.B) {
	proc := newBenchmarkProcessor(b, 5)
	spans := generateTestSpans(1000, 50)
	tree := proc.buildTraceTree(spans)
	leaves := tree.getLeaves()

	for _, leaf := range leaves {
		leaf.markedForRemoval = true
	}
	candidates := collectParentCandidates(leaves)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, c := range candidates {
			c.markedForRemoval = false
		}
		_ = proc.findEligibleParentNodesFromCandidates(candidates)
	}
}

// newBenchmarkProcessor creates a processor configured for benchmarking.
func newBenchmarkProcessor(b *testing.B, maxParentDepth int) *spanPruningProcessor {
	b.Helper()

	cfg := createDefaultConfig().(*Config)
	cfg.GroupByAttributes = []string{"http.*", "db.*"}
	cfg.MinSpansToAggregate = 5
	cfg.MaxParentDepth = maxParentDepth

	set := processortest.NewNopSettings(metadata.Type)
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		b.Fatal(err)
	}

	proc, err := newSpanPruningProcessor(set, cfg, telemetryBuilder)
	if err != nil {
		b.Fatal(err)
	}
	return proc
}

func benchmarkProcessTrace(b *testing.B, numSpans, minSpans int) {
	cfg := createDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = minSpans
	cfg.GroupByAttributes = []string{"http.*"}

	set := processortest.NewNopSettings(metadata.Type)
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		b.Fatal(err)
	}

	proc, err := newSpanPruningProcessor(set, cfg, telemetryBuilder)
	if err != nil {
		b.Fatal(err)
	}

	td := generateTestTrace(numSpans, minSpans)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloned := ptrace.NewTraces()
		td.CopyTo(cloned)
		_, err := proc.processTraces(context.Background(), cloned)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkProcessTraceSparse(b *testing.B, numSpans, minSpans int) {
	cfg := createDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = minSpans
	cfg.GroupByAttributes = []string{"db.*"}
	cfg.MaxParentDepth = 3

	set := processortest.NewNopSettings(metadata.Type)
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		b.Fatal(err)
	}

	proc, err := newSpanPruningProcessor(set, cfg, telemetryBuilder)
	if err != nil {
		b.Fatal(err)
	}

	td := generateSparseTrace(numSpans, minSpans)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloned := ptrace.NewTraces()
		td.CopyTo(cloned)
		_, err := proc.processTraces(context.Background(), cloned)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkDeepTrace(b *testing.B, depth, branchingFactor, leafsPerBranch, maxSpans, maxParentDepth int) {
	cfg := createDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.*"}
	cfg.MaxParentDepth = maxParentDepth

	set := processortest.NewNopSettings(metadata.Type)
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		b.Fatal(err)
	}

	proc, err := newSpanPruningProcessor(set, cfg, telemetryBuilder)
	if err != nil {
		b.Fatal(err)
	}

	td := generateDeepTrace(depth, branchingFactor, leafsPerBranch, maxSpans)
	b.ReportMetric(float64(td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len()), "spans")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloned := ptrace.NewTraces()
		td.CopyTo(cloned)
		_, err := proc.processTraces(context.Background(), cloned)
		if err != nil {
			b.Fatal(err)
		}
	}
}
