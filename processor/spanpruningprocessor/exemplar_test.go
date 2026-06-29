// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

func TestExemplarSampleSize(t *testing.T) {
	tests := []struct {
		name       string
		n          int
		multiplier float64
		want       int
	}{
		{name: "zero population", n: 0, multiplier: 1.0, want: 0},
		{name: "zero multiplier", n: 100, multiplier: 0, want: 0},
		{name: "negative multiplier", n: 100, multiplier: -1, want: 0},
		{name: "small N=4 multiplier=1", n: 4, multiplier: 1.0, want: 2},
		{name: "N=100 multiplier=1", n: 100, multiplier: 1.0, want: 10},
		{name: "N=100 multiplier=2", n: 100, multiplier: 2.0, want: 20},
		{name: "N=100 multiplier=0.5", n: 100, multiplier: 0.5, want: 5},
		{name: "N=10000 multiplier=1", n: 10000, multiplier: 1.0, want: 100},
		{name: "clamped: K>N", n: 4, multiplier: 5.0, want: 4},
		{name: "ceil rounds up: N=10 multiplier=1", n: 10, multiplier: 1.0, want: int(math.Ceil(math.Sqrt(10)))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exemplarSampleSize(tt.n, tt.multiplier)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSampleExemplars_CountAndNoReplacement(t *testing.T) {
	nodes := makeNodes(100)

	remaining, exemplars := sampleExemplars(nodes, 1.0)

	wantK := exemplarSampleSize(100, 1.0)
	require.Len(t, exemplars, wantK)
	require.Len(t, remaining, 100-wantK)

	// Combined slices must cover the original set with no duplicates.
	seen := make(map[*spanNode]struct{}, 100)
	for _, n := range exemplars {
		_, dup := seen[n]
		require.False(t, dup, "duplicate exemplar selected")
		seen[n] = struct{}{}
	}
	for _, n := range remaining {
		_, dup := seen[n]
		require.False(t, dup, "node appears in both remaining and exemplars")
		seen[n] = struct{}{}
	}
	assert.Len(t, seen, 100)
}

func TestSampleExemplars_KGreaterOrEqualN(t *testing.T) {
	nodes := makeNodes(4)
	// multiplier=5 forces K to clamp to N.
	remaining, exemplars := sampleExemplars(nodes, 5.0)
	assert.Nil(t, remaining)
	assert.Len(t, exemplars, 4)
}

func TestSampleExemplars_ZeroK(t *testing.T) {
	nodes := makeNodes(10)
	remaining, exemplars := sampleExemplars(nodes, 0)
	assert.Equal(t, nodes, remaining)
	assert.Nil(t, exemplars)
}

func TestSampleExemplars_PreservesOrderOfRemaining(t *testing.T) {
	nodes := makeNodes(50)
	remaining, _ := sampleExemplars(nodes, 1.0)

	// Build a position map from the original ordering and verify remaining
	// slice is monotonically increasing by original index.
	pos := make(map[*spanNode]int, len(nodes))
	for i, n := range nodes {
		pos[n] = i
	}
	last := -1
	for _, n := range remaining {
		p := pos[n]
		require.Greater(t, p, last, "remaining slice not in original order")
		last = p
	}
}

func TestUpdateExemplarThreshold_ComposesWhenPriorThExists(t *testing.T) {
	// Construct a span whose TraceState declares 1/2 sampling (th encodes
	// rejection probability 1/2 -> sampling probability 1/2).
	priorTh, err := sampling.ProbabilityToThreshold(0.5)
	require.NoError(t, err)

	span := ptrace.NewSpan()
	span.TraceState().FromRaw("ot=th:" + priorTh.TValue())

	// Group of N=100, K=10 exemplars: new probability = 0.5 * 10/100 = 0.05.
	updateExemplarThreshold(span, 10, 100)

	w3c, err := sampling.NewW3CTraceState(span.TraceState().AsRaw())
	require.NoError(t, err)
	newTh, hasTh := w3c.OTelValue().TValueThreshold()
	require.True(t, hasTh, "expected th to remain present after composition")

	wantTh, err := sampling.ProbabilityToThreshold(0.05)
	require.NoError(t, err)
	assert.Equal(t, wantTh.TValue(), newTh.TValue())
}

func TestUpdateExemplarThreshold_NoopWhenNoPriorTh(t *testing.T) {
	span := ptrace.NewSpan()
	const original = "vendor=abc"
	span.TraceState().FromRaw(original)

	updateExemplarThreshold(span, 10, 100)

	assert.Equal(t, original, span.TraceState().AsRaw(), "TraceState should be unchanged when no prior th is present")
}

func TestUpdateExemplarThreshold_NoopWhenEmptyTraceState(t *testing.T) {
	span := ptrace.NewSpan()
	updateExemplarThreshold(span, 10, 100)
	assert.Empty(t, span.TraceState().AsRaw())
}

func TestUpdateExemplarThreshold_NoopWhenMalformed(t *testing.T) {
	span := ptrace.NewSpan()
	const broken = "ot=garbage:!!!"
	span.TraceState().FromRaw(broken)
	updateExemplarThreshold(span, 10, 100)
	assert.Equal(t, broken, span.TraceState().AsRaw())
}

// TestUpdateExemplarThreshold_NoopWhenUnrepresentable ensures we leave the
// prior threshold untouched when p_old * K/N would fall below MinSamplingProbability.
// Clamping would have catastrophically inflated the consumer-side adjusted count.
func TestUpdateExemplarThreshold_NoopWhenUnrepresentable(t *testing.T) {
	// Prior th encoding the smallest representable sampling probability.
	priorTh, err := sampling.ProbabilityToThreshold(sampling.MinSamplingProbability)
	require.NoError(t, err)

	span := ptrace.NewSpan()
	priorTS := "ot=th:" + priorTh.TValue()
	span.TraceState().FromRaw(priorTS)

	// Any further sub-sampling makes p_new < MinSamplingProbability.
	updateExemplarThreshold(span, 1, 100)

	assert.Equal(t, priorTS, span.TraceState().AsRaw(), "threshold should be unchanged when composed probability is unrepresentable")
}

// makeNodes returns n spanNodes with unique identity for set-membership tests.
func makeNodes(n int) []*spanNode {
	nodes := make([]*spanNode, n)
	for i := range nodes {
		nodes[i] = &spanNode{}
	}
	return nodes
}
