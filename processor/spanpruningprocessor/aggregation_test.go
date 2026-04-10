// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestFindLongestDurationNode_Empty(t *testing.T) {
	result := findLongestDurationNode(nil)
	assert.Nil(t, result)

	result = findLongestDurationNode([]*spanNode{})
	assert.Nil(t, result)
}

func TestFindLongestDurationNode_SingleNode(t *testing.T) {
	nodes := createSpanNodesWithDurations(t, []int64{100})

	result := findLongestDurationNode(nodes)
	require.NotNil(t, result)
	assert.Equal(t, nodes[0], result)
}

func TestFindLongestDurationNode_LongestFirst(t *testing.T) {
	// Longest duration is first in the slice
	nodes := createSpanNodesWithDurations(t, []int64{500, 100, 200})

	result := findLongestDurationNode(nodes)
	require.NotNil(t, result)
	assert.Equal(t, nodes[0], result, "should return first node (500ns)")
}

func TestFindLongestDurationNode_LongestMiddle(t *testing.T) {
	// Longest duration is in the middle
	nodes := createSpanNodesWithDurations(t, []int64{100, 500, 200})

	result := findLongestDurationNode(nodes)
	require.NotNil(t, result)
	assert.Equal(t, nodes[1], result, "should return middle node (500ns)")
}

func TestFindLongestDurationNode_LongestLast(t *testing.T) {
	// Longest duration is last in the slice
	nodes := createSpanNodesWithDurations(t, []int64{100, 200, 500})

	result := findLongestDurationNode(nodes)
	require.NotNil(t, result)
	assert.Equal(t, nodes[2], result, "should return last node (500ns)")
}

func TestFindLongestDurationNode_EqualDurations(t *testing.T) {
	// All durations are equal - should return first
	nodes := createSpanNodesWithDurations(t, []int64{100, 100, 100})

	result := findLongestDurationNode(nodes)
	require.NotNil(t, result)
	assert.Equal(t, nodes[0], result, "should return first node when all equal")
}

func TestFindLongestDurationNode_LargeDurations(t *testing.T) {
	// Test with large duration values (milliseconds in nanoseconds)
	durations := []int64{
		1_000_000,   // 1ms
		500_000_000, // 500ms
		100_000_000, // 100ms
	}
	nodes := createSpanNodesWithDurations(t, durations)

	result := findLongestDurationNode(nodes)
	require.NotNil(t, result)
	assert.Equal(t, nodes[1], result, "should return node with 500ms duration")
}

func TestFindLongestDurationNode_ZeroDuration(t *testing.T) {
	// Test with zero duration spans
	nodes := createSpanNodesWithDurations(t, []int64{0, 100, 0})

	result := findLongestDurationNode(nodes)
	require.NotNil(t, result)
	assert.Equal(t, nodes[1], result, "should return node with non-zero duration")
}

func TestFindLongestDurationNode_ManyNodes(t *testing.T) {
	// Test with many nodes to verify iteration works correctly
	durations := make([]int64, 100)
	for i := range durations {
		durations[i] = int64(i * 10)
	}
	// Set one in the middle to be the longest
	durations[50] = 99999

	nodes := createSpanNodesWithDurations(t, durations)

	result := findLongestDurationNode(nodes)
	require.NotNil(t, result)
	assert.Equal(t, nodes[50], result, "should return node at index 50 with longest duration")
}

// createSpanNodesWithDurations creates span nodes with specified durations in nanoseconds
func createSpanNodesWithDurations(t *testing.T, durationsNs []int64) []*spanNode {
	t.Helper()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	nodes := make([]*spanNode, 0, len(durationsNs))
	baseTime := int64(1000000000)

	for i, duration := range durationsNs {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("test")
		span.SetStartTimestamp(pcommon.Timestamp(baseTime))
		span.SetEndTimestamp(pcommon.Timestamp(baseTime + duration))

		nodes = append(nodes, &spanNode{
			span:       span,
			scopeSpans: ss,
		})
	}

	return nodes
}
