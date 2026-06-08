// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package criticalpath // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/criticalpath"

import (
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/traceutil"
)

const (
	AttributeCriticalPathIsOnPath            = "cgx.critical_path.is_on_path"
	AttributeCriticalPathExclusiveDurationNS = "cgx.critical_path.exclusive_duration_ns"
	AttributeCriticalPathInclusiveDurationNS = "cgx.critical_path.inclusive_duration_ns"
)

type criticalPathSection struct {
	spanID         pcommon.SpanID
	sectionStartNS int64
	sectionEndNS   int64
}

type contribution struct {
	exclusiveNS int64
	inclusiveNS int64
}

type sanitizeStats struct {
	droppedChildren   int
	truncatedChildren int
}

func ApplyCriticalPathAttributesByTraceID(spansByTraceID map[pcommon.TraceID][]ptrace.Span, logger *zap.Logger) {
	for traceID, spans := range spansByTraceID {
		logger.Debug("processing trace", zap.String("traceID", traceID.String()), zap.Int("spans", len(spans)))
		removeCriticalPathAttributes(spans)

		tree := traceutil.BuildTraceTree(spans)
		stats := sanitizeOverflowingChildren(tree)
		if stats.droppedChildren > 0 || stats.truncatedChildren > 0 {
			logger.Warn(
				"adjusted malformed spans while computing critical path",
				zap.String("traceID", traceID.String()),
				zap.Int("droppedChildren", stats.droppedChildren),
				zap.Int("truncatedChildren", stats.truncatedChildren),
			)
		}
		sections := computeCriticalPath(tree)
		contributions := accumulateContributions(tree, sections)
		annotateSpans(spans, contributions)
	}
}

func ApplyCriticalPathAttributesToTree(traceID pcommon.TraceID, tree traceutil.TraceTree, logger *zap.Logger) {
	spans := make([]ptrace.Span, 0, len(tree.Nodes))
	for _, node := range tree.Nodes {
		spans = append(spans, node.Span)
	}
	removeCriticalPathAttributes(spans)

	stats := sanitizeOverflowingChildren(tree)
	if stats.droppedChildren > 0 || stats.truncatedChildren > 0 {
		logger.Warn(
			"adjusted malformed spans while computing critical path",
			zap.String("traceID", traceID.String()),
			zap.Int("droppedChildren", stats.droppedChildren),
			zap.Int("truncatedChildren", stats.truncatedChildren),
		)
	}
	sections := computeCriticalPath(tree)
	contributions := accumulateContributions(tree, sections)
	annotateSpans(spans, contributions)
}

func computeCriticalPath(graph traceutil.TraceTree) []criticalPathSection {
	var sections []criticalPathSection
	sortNodesByEndDesc(graph.Roots)
	for _, root := range graph.Roots {
		sortChildrenRecursive(root)
		sections = computeCriticalPathSections(root, sections, nil)
	}
	return sections
}

func computeCriticalPathSections(
	current *traceutil.TraceTreeNode,
	sections []criticalPathSection,
	returningChildStartNS *int64,
) []criticalPathSection {
	if current == nil || current.EndNS <= current.StartNS {
		return sections
	}

	lastFinishingChild := findLastFinishingChild(current, returningChildStartNS)
	sectionEndNS := current.EndNS
	if returningChildStartNS != nil {
		sectionEndNS = *returningChildStartNS
	}

	if lastFinishingChild != nil {
		if lastFinishingChild.EndNS < sectionEndNS {
			sections = append(sections, criticalPathSection{
				spanID:         current.Span.SpanID(),
				sectionStartNS: lastFinishingChild.EndNS,
				sectionEndNS:   sectionEndNS,
			})
		}
		return computeCriticalPathSections(lastFinishingChild, sections, nil)
	}

	if current.StartNS < sectionEndNS {
		sections = append(sections, criticalPathSection{
			spanID:         current.Span.SpanID(),
			sectionStartNS: current.StartNS,
			sectionEndNS:   sectionEndNS,
		})
	}

	if current.Parent != nil {
		startNS := current.StartNS
		return computeCriticalPathSections(current.Parent, sections, &startNS)
	}

	return sections
}

func findLastFinishingChild(current *traceutil.TraceTreeNode, returningChildStartNS *int64) *traceutil.TraceTreeNode {
	var lastFinishingChild *traceutil.TraceTreeNode
	maxEndNS := int64(-1)

	for _, child := range current.Children {
		if child.EndNS <= child.StartNS {
			continue
		}

		if returningChildStartNS != nil {
			if child.EndNS >= *returningChildStartNS {
				continue
			}
		}

		if child.EndNS > maxEndNS {
			maxEndNS = child.EndNS
			lastFinishingChild = child
		}
	}

	return lastFinishingChild
}

func accumulateContributions(graph traceutil.TraceTree, sections []criticalPathSection) map[pcommon.SpanID]contribution {
	contributions := make(map[pcommon.SpanID]contribution, len(graph.Nodes))

	for _, section := range sections {
		durationNS := section.sectionEndNS - section.sectionStartNS
		if durationNS <= 0 {
			continue
		}

		node := graph.Nodes[section.spanID]
		if node == nil {
			continue
		}

		contrib := contributions[section.spanID]
		contrib.exclusiveNS += durationNS
		contributions[section.spanID] = contrib
	}

	for _, root := range graph.Roots {
		accumulateInclusiveContributions(root, contributions)
	}

	return contributions
}

func accumulateInclusiveContributions(
	node *traceutil.TraceTreeNode,
	contributions map[pcommon.SpanID]contribution,
) int64 {
	if node == nil {
		return 0
	}

	contrib := contributions[node.Span.SpanID()]
	inclusiveNS := contrib.exclusiveNS
	for _, child := range node.Children {
		inclusiveNS += accumulateInclusiveContributions(child, contributions)
	}
	contrib.inclusiveNS = inclusiveNS
	contributions[node.Span.SpanID()] = contrib
	return inclusiveNS
}

func sanitizeOverflowingChildren(graph traceutil.TraceTree) sanitizeStats {
	stats := sanitizeStats{}
	for _, root := range graph.Roots {
		sanitizeSubtree(root, graph.Nodes, &stats)
		sortChildrenRecursive(root)
	}

	return stats
}

func sanitizeSubtree(
	parent *traceutil.TraceTreeNode,
	nodes map[pcommon.SpanID]*traceutil.TraceTreeNode,
	stats *sanitizeStats,
) {
	if parent == nil {
		return
	}

	children := append([]*traceutil.TraceTreeNode(nil), parent.Children...)
	for _, child := range children {
		switch {
		case child.StartNS >= parent.EndNS:
			removeChild(parent, child)
			delete(nodes, child.Span.SpanID())
			stats.droppedChildren++
			continue
		case child.EndNS <= parent.StartNS:
			removeChild(parent, child)
			delete(nodes, child.Span.SpanID())
			stats.droppedChildren++
			continue
		case child.StartNS < parent.StartNS && child.EndNS > parent.EndNS:
			child.StartNS = parent.StartNS
			child.EndNS = parent.EndNS
			stats.truncatedChildren++
		case child.StartNS < parent.StartNS:
			child.StartNS = parent.StartNS
			stats.truncatedChildren++
		case child.EndNS > parent.EndNS:
			child.EndNS = parent.EndNS
			stats.truncatedChildren++
		}

		sanitizeSubtree(child, nodes, stats)
	}
}

func removeChild(parent, target *traceutil.TraceTreeNode) {
	filtered := parent.Children[:0]
	for _, child := range parent.Children {
		if child != target {
			filtered = append(filtered, child)
		}
	}
	parent.Children = filtered
	target.Parent = nil
}

func annotateSpans(spans []ptrace.Span, contributions map[pcommon.SpanID]contribution) {
	for _, span := range spans {
		contrib, ok := contributions[span.SpanID()]
		if !ok || contrib.inclusiveNS <= 0 {
			continue
		}

		span.Attributes().PutBool(AttributeCriticalPathIsOnPath, true)
		span.Attributes().PutInt(AttributeCriticalPathExclusiveDurationNS, contrib.exclusiveNS)
		span.Attributes().PutInt(AttributeCriticalPathInclusiveDurationNS, contrib.inclusiveNS)
	}
}

func removeCriticalPathAttributes(spans []ptrace.Span) {
	for _, span := range spans {
		attrs := span.Attributes()
		attrs.Remove(AttributeCriticalPathIsOnPath)
		attrs.Remove(AttributeCriticalPathExclusiveDurationNS)
		attrs.Remove(AttributeCriticalPathInclusiveDurationNS)
	}
}

func sortChildrenRecursive(node *traceutil.TraceTreeNode) {
	sortNodesByEndDesc(node.Children)
	for _, child := range node.Children {
		sortChildrenRecursive(child)
	}
}

func sortNodesByEndDesc(nodes []*traceutil.TraceTreeNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].EndNS != nodes[j].EndNS {
			return nodes[i].EndNS > nodes[j].EndNS
		}
		if nodes[i].StartNS != nodes[j].StartNS {
			return nodes[i].StartNS > nodes[j].StartNS
		}
		return nodes[i].Span.SpanID().String() > nodes[j].Span.SpanID().String()
	})
}
