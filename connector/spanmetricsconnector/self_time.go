// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"

import (
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// noParent marks a node with no resolvable parent inside the batch, either
// because the span is a root or because its parent is absent.
const noParent = int32(-1)

// selfTimeSpanNode is one span in the tree of a single trace. Children are held
// as a range into a shared slice rather than a per-node slice, so building the
// tree costs one allocation for the whole trace instead of one per parent.
type selfTimeSpanNode struct {
	spanID    pcommon.SpanID
	parentID  pcommon.SpanID
	parentIdx int32
	childOff  int32
	childLen  int32
	startNS   int64
	endNS     int64
}

// selfTimeInterval is a half-open [start, end) child interval clamped to its
// parent.
type selfTimeInterval struct {
	start, end int64
}

// selfTimeScratch holds the working buffers for the self-time computation. One
// instance is reused across every trace in a batch, so a batch of many small
// traces does not re-allocate the tree structures for each one.
type selfTimeScratch struct {
	nodes    []selfTimeSpanNode
	indexBy  map[pcommon.SpanID]int32
	children []int32
	cursors  []int32
	clamped  []selfTimeInterval
}

// computeSelfTimeBySpanID returns the self time in nanoseconds of every span in
// the batch, keyed by span ID. Traces that are not complete in the batch are omitted.
func computeSelfTimeBySpanID(traces ptrace.Traces) map[pcommon.SpanID]int64 {
	spansByTrace := groupSpansByTraceID(traces)
	out := make(map[pcommon.SpanID]int64, traces.SpanCount())
	var scratch selfTimeScratch
	for _, spans := range spansByTrace {
		scratch.computeForTrace(spans, out)
	}
	return out
}

func groupSpansByTraceID(traces ptrace.Traces) map[pcommon.TraceID][]ptrace.Span {
	grouped := make(map[pcommon.TraceID][]ptrace.Span)
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				tid := span.TraceID()
				if tid.IsEmpty() {
					continue
				}
				grouped[tid] = append(grouped[tid], span)
			}
		}
	}
	return grouped
}

// computeForTrace writes the self time of every span of one trace into out. It
// writes nothing when the trace is not complete in this batch.
func (s *selfTimeScratch) computeForTrace(spans []ptrace.Span, out map[pcommon.SpanID]int64) {
	if !s.buildNodes(spans) {
		return
	}
	if !s.linkParents() {
		return
	}
	s.fillChildren()

	for i := range s.nodes {
		node := &s.nodes[i]
		if selfTime, ok := s.selfTimeNS(node); ok {
			out[node.spanID] = selfTime
		}
	}
}

// buildNodes resets the scratch buffers and loads the spans. It reports false
// when the same span ID appears more than once, which makes the tree ambiguous.
func (s *selfTimeScratch) buildNodes(spans []ptrace.Span) bool {
	s.nodes = s.nodes[:0]
	if s.indexBy == nil {
		s.indexBy = make(map[pcommon.SpanID]int32, len(spans))
	} else {
		clear(s.indexBy)
	}

	for _, span := range spans {
		spanID := span.SpanID()
		if spanID.IsEmpty() {
			continue
		}
		if _, exists := s.indexBy[spanID]; exists {
			return false
		}
		s.indexBy[spanID] = int32(len(s.nodes))
		s.nodes = append(s.nodes, selfTimeSpanNode{
			spanID:    spanID,
			parentID:  span.ParentSpanID(),
			parentIdx: noParent,
			startNS:   int64(span.StartTimestamp()),
			endNS:     int64(span.EndTimestamp()),
		})
	}
	return true
}

// linkParents resolves each parent span ID to its index and counts the children
// of every node. It reports false when the batch holds a root span next to a
// span whose parent is missing, which means this trace arrived only in part.
func (s *selfTimeScratch) linkParents() bool {
	var hasExplicitRoot, hasMissingParent bool
	for i := range s.nodes {
		node := &s.nodes[i]
		if node.parentID.IsEmpty() {
			hasExplicitRoot = true
			continue
		}
		parentIdx, ok := s.indexBy[node.parentID]
		if !ok {
			hasMissingParent = true
			continue
		}
		node.parentIdx = parentIdx
		s.nodes[parentIdx].childLen++
	}
	return !hasExplicitRoot || !hasMissingParent
}

// fillChildren turns the per-node child counts into offsets into one shared
// slice, then places every child index at its parent's offset.
func (s *selfTimeScratch) fillChildren() {
	var total int32
	for i := range s.nodes {
		s.nodes[i].childOff = total
		total += s.nodes[i].childLen
	}

	s.children = slices.Grow(s.children[:0], int(total))[:total]
	s.cursors = slices.Grow(s.cursors[:0], len(s.nodes))[:len(s.nodes)]
	for i := range s.nodes {
		s.cursors[i] = s.nodes[i].childOff
	}
	for i := range s.nodes {
		parentIdx := s.nodes[i].parentIdx
		if parentIdx == noParent {
			continue
		}
		s.children[s.cursors[parentIdx]] = int32(i)
		s.cursors[parentIdx]++
	}
}

func (s *selfTimeScratch) childrenOf(node *selfTimeSpanNode) []int32 {
	return s.children[node.childOff : node.childOff+node.childLen]
}

func (s *selfTimeScratch) selfTimeNS(node *selfTimeSpanNode) (int64, bool) {
	if node.endNS < node.startNS {
		return 0, false
	}
	duration := node.endNS - node.startNS
	if duration == 0 || node.childLen == 0 {
		return duration, true
	}
	covered := s.coveredDurationNS(node)
	selfTime := max(duration-covered, 0)
	return selfTime, true
}

// coveredDurationNS returns the length of the union of the child intervals of
// node, each clamped to the interval of node. Children that run at the same
// time must only be counted once, so the intervals are merged before they are
// added up.
func (s *selfTimeScratch) coveredDurationNS(node *selfTimeSpanNode) int64 {
	parentStart, parentEnd := node.startNS, node.endNS
	children := s.childrenOf(node)

	clamped := s.clamped[:0]
	for _, childIdx := range children {
		child := &s.nodes[childIdx]
		if child.endNS <= child.startNS {
			continue
		}
		start := max(child.startNS, parentStart)
		end := min(child.endNS, parentEnd)
		if end > start {
			clamped = append(clamped, selfTimeInterval{start: start, end: end})
		}
	}
	s.clamped = clamped
	if len(clamped) == 0 {
		return 0
	}

	slices.SortFunc(clamped, func(a, b selfTimeInterval) int {
		if a.start != b.start {
			if a.start < b.start {
				return -1
			}
			return 1
		}
		if a.end < b.end {
			return -1
		}
		if a.end > b.end {
			return 1
		}
		return 0
	})

	mergedStart, mergedEnd := clamped[0].start, clamped[0].end
	var covered int64
	for _, iv := range clamped[1:] {
		if iv.start <= mergedEnd {
			if iv.end > mergedEnd {
				mergedEnd = iv.end
			}
			continue
		}
		covered += mergedEnd - mergedStart
		mergedStart, mergedEnd = iv.start, iv.end
	}
	covered += mergedEnd - mergedStart
	return covered
}
