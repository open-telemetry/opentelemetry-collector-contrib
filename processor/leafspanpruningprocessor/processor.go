// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leafspanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor"

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

// spanInfo holds a span and its location within the trace data structure
type spanInfo struct {
	span       ptrace.Span
	scopeSpans ptrace.ScopeSpans
}

// aggregationStats holds statistics for a group of spans
type aggregationStats struct {
	count        int64
	minDuration  time.Duration
	maxDuration  time.Duration
	sumDuration  time.Duration
	bucketCounts []int64
}

// aggregationGroup represents a group of spans to be aggregated
type aggregationGroup struct {
	spans         []spanInfo
	level         int            // tree level (0 = leaf, 1 = parent of leaf, etc.)
	summarySpanID pcommon.SpanID // SpanID of the summary span (assigned before creation)
}

// aggregationPlan holds all aggregations ordered for top-down execution
type aggregationPlan struct {
	groups []aggregationGroup
}

// attributePattern holds a compiled glob pattern for matching attribute keys
type attributePattern struct {
	glob glob.Glob
}

// spanNode represents a span in the trace tree with parent/child relationships
type spanNode struct {
	info     spanInfo
	parent   *spanNode
	children []*spanNode
}

// traceTree represents a complete trace as a tree structure
type traceTree struct {
	nodeByID map[pcommon.SpanID]*spanNode
	orphans  []*spanNode // spans whose parent is not in the trace
}

type leafSpanPruningProcessor struct {
	config            *Config
	logger            *zap.Logger
	attributePatterns []attributePattern
}

func newLeafSpanPruningProcessor(set processor.Settings, cfg *Config) (*leafSpanPruningProcessor, error) {
	// Compile glob patterns for group_by_attributes
	patterns := make([]attributePattern, 0, len(cfg.GroupByAttributes))
	for _, pattern := range cfg.GroupByAttributes {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		patterns = append(patterns, attributePattern{
			glob: g,
		})
	}

	return &leafSpanPruningProcessor{
		config:            cfg,
		logger:            set.Logger,
		attributePatterns: patterns,
	}, nil
}

func (p *leafSpanPruningProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	// Group spans by TraceID
	traceSpans := p.groupSpansByTraceID(td)

	// Process each trace independently
	for _, spans := range traceSpans {
		if err := p.processTrace(ctx, spans, td); err != nil {
			return td, err
		}
	}

	return td, nil
}

// groupSpansByTraceID collects all spans organized by trace ID
func (p *leafSpanPruningProcessor) groupSpansByTraceID(td ptrace.Traces) map[pcommon.TraceID][]spanInfo {
	traceSpans := make(map[pcommon.TraceID][]spanInfo)

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				traceID := span.TraceID()
				traceSpans[traceID] = append(traceSpans[traceID], spanInfo{
					span:       span,
					scopeSpans: ils,
				})
			}
		}
	}

	return traceSpans
}

// buildTraceTree constructs a tree structure from a list of spans
// Handles incomplete traces: orphans (missing parents), multiple roots, no root
func (p *leafSpanPruningProcessor) buildTraceTree(spans []spanInfo) *traceTree {
	tree := &traceTree{
		nodeByID: make(map[pcommon.SpanID]*spanNode),
	}

	if len(spans) == 0 {
		return tree
	}

	// First pass: create nodes for all spans
	for _, info := range spans {
		node := &spanNode{info: info}
		tree.nodeByID[info.span.SpanID()] = node
	}

	// Second pass: identify root(s) and link parent-child relationships
	var rootCount int
	for _, node := range tree.nodeByID {
		parentID := node.info.span.ParentSpanID()
		if parentID.IsEmpty() {
			// This is a root span (no parent)
			rootCount++
		} else if parent, exists := tree.nodeByID[parentID]; exists {
			// Link to parent
			node.parent = parent
			parent.children = append(parent.children, node)
		} else {
			// Parent not in trace - this is an orphan
			tree.orphans = append(tree.orphans, node)
		}
	}

	// Log warnings for incomplete traces
	if rootCount > 1 {
		p.logger.Debug("multiple root spans found",
			zap.Int("rootCount", rootCount))
	} else if rootCount == 0 && len(tree.orphans) > 0 {
		p.logger.Debug("no root span found, trace may be incomplete")
	}

	if len(tree.orphans) > 0 {
		p.logger.Debug("orphaned spans detected",
			zap.Int("orphanCount", len(tree.orphans)))
	}

	return tree
}

// findLeafNodes returns all nodes with no children (leaf spans)
func (t *traceTree) findLeafNodes() []*spanNode {
	var leaves []*spanNode
	for _, node := range t.nodeByID {
		if len(node.children) == 0 {
			leaves = append(leaves, node)
		}
	}
	return leaves
}

// processTrace processes a single trace using two-phase approach:
// Phase 1: Analyze aggregations bottom-up (identify leaf groups, then eligible parents)
// Phase 2: Execute aggregations top-down (create parent summaries first, then children)
func (p *leafSpanPruningProcessor) processTrace(ctx context.Context, spans []spanInfo, td ptrace.Traces) error {
	// Build trace tree
	tree := p.buildTraceTree(spans)
	if len(tree.nodeByID) == 0 {
		return nil
	}

	// Phase 1: Analyze aggregations (bottom-up)
	aggregationGroups := p.analyzeAggregationsWithTree(tree)
	if len(aggregationGroups) == 0 {
		return nil
	}

	// Phase 2: Build aggregation plan (order top-down)
	plan := p.buildAggregationPlan(aggregationGroups)

	// Phase 3: Execute aggregations (top-down)
	p.executeAggregations(plan)

	return nil
}

// buildGroupKey creates a grouping key from span name, status, and matching attributes
// Attributes are matched using glob patterns from the configuration
func (p *leafSpanPruningProcessor) buildGroupKey(span ptrace.Span) string {
	var builder strings.Builder
	builder.WriteString(span.Name())

	// Include status code in grouping key
	builder.WriteString("|status=")
	builder.WriteString(span.Status().Code().String())

	attrs := span.Attributes()

	// Collect all matching attribute key-value pairs
	matchedAttrs := make(map[string]string)
	attrs.Range(func(key string, value pcommon.Value) bool {
		for _, pattern := range p.attributePatterns {
			if pattern.glob.Match(key) {
				matchedAttrs[key] = value.AsString()
				break // Only match each key once
			}
		}
		return true
	})

	// Sort keys for consistent ordering in the group key
	keys := make([]string, 0, len(matchedAttrs))
	for k := range matchedAttrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build the group key with sorted attribute key-value pairs
	for _, key := range keys {
		builder.WriteString("|")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(matchedAttrs[key])
	}

	return builder.String()
}

// calculateStats computes aggregation statistics for a group of spans
func (p *leafSpanPruningProcessor) calculateStats(spans []spanInfo) aggregationStats {
	stats := aggregationStats{count: int64(len(spans))}

	// Initialize histogram bucket counts
	if len(p.config.AggregationHistogramBuckets) > 0 {
		stats.bucketCounts = make([]int64, len(p.config.AggregationHistogramBuckets)+1)
	}

	for i, info := range spans {
		startTime := info.span.StartTimestamp().AsTime()
		endTime := info.span.EndTimestamp().AsTime()
		duration := endTime.Sub(startTime)

		if i == 0 {
			stats.minDuration = duration
			stats.maxDuration = duration
		} else {
			if duration < stats.minDuration {
				stats.minDuration = duration
			}
			if duration > stats.maxDuration {
				stats.maxDuration = duration
			}
		}
		stats.sumDuration += duration

		// Update histogram bucket counts (cumulative)
		if len(p.config.AggregationHistogramBuckets) > 0 {
			// Find which bucket this duration belongs to
			bucketIndex := len(p.config.AggregationHistogramBuckets) // default to +Inf bucket
			for j, bucket := range p.config.AggregationHistogramBuckets {
				if duration <= bucket {
					bucketIndex = j
					break
				}
			}
			// Increment all buckets from bucketIndex to the end (cumulative histogram)
			for j := bucketIndex; j < len(stats.bucketCounts); j++ {
				stats.bucketCounts[j]++
			}
		}
	}

	return stats
}

// findTimeRange finds the earliest start and latest end timestamps in a group
func (p *leafSpanPruningProcessor) findTimeRange(spans []spanInfo) (pcommon.Timestamp, pcommon.Timestamp) {
	var earliestStart, latestEnd pcommon.Timestamp

	for i, info := range spans {
		start := info.span.StartTimestamp()
		end := info.span.EndTimestamp()

		if i == 0 {
			earliestStart = start
			latestEnd = end
		} else {
			if start < earliestStart {
				earliestStart = start
			}
			if end > latestEnd {
				latestEnd = end
			}
		}
	}

	return earliestStart, latestEnd
}

// generateSpanID creates a new random span ID
func generateSpanID() pcommon.SpanID {
	var id [8]byte
	_, _ = rand.Read(id[:])
	return pcommon.SpanID(id)
}

// buildParentGroupKey creates a grouping key for parent spans using only name and status
// (attributes are not considered for parent aggregation)
func (p *leafSpanPruningProcessor) buildParentGroupKey(span ptrace.Span) string {
	var builder strings.Builder
	builder.WriteString(span.Name())
	builder.WriteString("|status=")
	builder.WriteString(span.Status().Code().String())
	return builder.String()
}

// buildLeafGroupKeyFromNode creates a grouping key for leaf spans using tree node parent pointer
func (p *leafSpanPruningProcessor) buildLeafGroupKeyFromNode(node *spanNode) string {
	var builder strings.Builder

	// Include parent span name to separate groups by parent
	if node.parent != nil {
		builder.WriteString("parent=")
		builder.WriteString(node.parent.info.span.Name())
		builder.WriteString("|")
	}

	// Include regular group key (name + status + attributes)
	builder.WriteString(p.buildGroupKey(node.info.span))

	return builder.String()
}

// groupLeafNodesByKey groups leaf nodes by their grouping key
func (p *leafSpanPruningProcessor) groupLeafNodesByKey(leafNodes []*spanNode) map[string][]*spanNode {
	groups := make(map[string][]*spanNode)
	for _, node := range leafNodes {
		key := p.buildLeafGroupKeyFromNode(node)
		groups[key] = append(groups[key], node)
	}
	return groups
}

// analyzeAggregationsWithTree performs Phase 1 using tree structure
func (p *leafSpanPruningProcessor) analyzeAggregationsWithTree(tree *traceTree) map[string]aggregationGroup {
	// Step 1: Find leaf nodes (nodes with no children)
	leafNodes := tree.findLeafNodes()
	if len(leafNodes) == 0 {
		return nil
	}

	// Step 2: Group similar leaf nodes
	leafGroups := p.groupLeafNodesByKey(leafNodes)

	// Step 3: Filter groups meeting minimum threshold
	aggregationGroups := make(map[string]aggregationGroup)
	nodesToRemove := make(map[pcommon.SpanID]string) // spanID -> groupKey

	for groupKey, nodes := range leafGroups {
		if len(nodes) >= p.config.MinSpansToAggregate {
			// Convert nodes to spanInfo slice
			spans := make([]spanInfo, len(nodes))
			for i, node := range nodes {
				spans[i] = node.info
			}

			aggregationGroups[groupKey] = aggregationGroup{
				spans: spans,
				level: 0,
			}
			// Mark these spans for removal
			for _, node := range nodes {
				nodesToRemove[node.info.span.SpanID()] = groupKey
			}
		}
	}

	if len(aggregationGroups) == 0 {
		return nil
	}

	// Step 4: Walk up the tree to find eligible parent spans recursively
	level := 1
	for {
		parentCandidates := p.findEligibleParentNodes(tree, nodesToRemove)
		if len(parentCandidates) == 0 {
			break
		}

		// Group parent candidates by name + status
		parentGroups := make(map[string][]*spanNode)
		for _, node := range parentCandidates {
			parentKey := p.buildParentGroupKey(node.info.span)
			parentGroups[parentKey] = append(parentGroups[parentKey], node)
		}

		// Add parent groups (at least 2 parents to aggregate)
		foundNewParents := false
		for parentKey, nodes := range parentGroups {
			if len(nodes) >= 2 {
				// Convert nodes to spanInfo slice
				spans := make([]spanInfo, len(nodes))
				for i, node := range nodes {
					spans[i] = node.info
				}

				aggregationGroups[parentKey] = aggregationGroup{
					spans: spans,
					level: level,
				}
				// Mark these parent spans for removal
				for _, node := range nodes {
					nodesToRemove[node.info.span.SpanID()] = parentKey
				}
				foundNewParents = true
			}
		}

		if !foundNewParents {
			break
		}
		level++
	}

	return aggregationGroups
}

// findEligibleParentNodes finds parent nodes whose ALL children are marked for removal
func (p *leafSpanPruningProcessor) findEligibleParentNodes(tree *traceTree, nodesToRemove map[pcommon.SpanID]string) []*spanNode {
	var eligibleParents []*spanNode

	for _, node := range tree.nodeByID {
		// Skip if node has no children (it's a leaf)
		if len(node.children) == 0 {
			continue
		}

		// Skip if node is root (no parent)
		if node.parent == nil {
			continue
		}

		// Skip if already marked for removal
		if _, alreadyMarked := nodesToRemove[node.info.span.SpanID()]; alreadyMarked {
			continue
		}

		// Check if ALL children are marked for removal
		allChildrenRemoved := true
		for _, child := range node.children {
			if _, willRemove := nodesToRemove[child.info.span.SpanID()]; !willRemove {
				allChildrenRemoved = false
				break
			}
		}

		if allChildrenRemoved {
			eligibleParents = append(eligibleParents, node)
		}
	}

	return eligibleParents
}

// buildAggregationPlan orders aggregation groups for top-down execution
func (p *leafSpanPruningProcessor) buildAggregationPlan(groups map[string]aggregationGroup) aggregationPlan {
	// Convert map to slice
	groupSlice := make([]aggregationGroup, 0, len(groups))
	for _, group := range groups {
		groupSlice = append(groupSlice, group)
	}

	// Sort by level descending (highest level first = top-down)
	sort.Slice(groupSlice, func(i, j int) bool {
		return groupSlice[i].level > groupSlice[j].level
	})

	// Pre-assign SpanIDs for all summary spans
	for i := range groupSlice {
		groupSlice[i].summarySpanID = generateSpanID()
	}

	return aggregationPlan{groups: groupSlice}
}

// executeAggregations performs Phase 2: top-down creation of summary spans
func (p *leafSpanPruningProcessor) executeAggregations(plan aggregationPlan) {
	// Track which parent SpanID should map to which summary SpanID
	parentReplacements := make(map[pcommon.SpanID]pcommon.SpanID)

	for _, group := range plan.groups {
		// Calculate statistics
		stats := p.calculateStats(group.spans)

		// Determine the parent SpanID for the summary span
		// Use the first span's parent as template
		originalParentID := group.spans[0].span.ParentSpanID()

		// Check if the parent is being replaced by a summary span
		summaryParentID := originalParentID
		if replacementID, exists := parentReplacements[originalParentID]; exists {
			summaryParentID = replacementID
		}

		// Create summary span with correct parent
		p.createSummarySpanWithParent(group, stats, summaryParentID)

		// Record that these original span IDs should be replaced by the summary span ID
		for _, info := range group.spans {
			parentReplacements[info.span.SpanID()] = group.summarySpanID
		}

		// Mark original spans for removal
		for _, info := range group.spans {
			info.span.SetName("")
		}
	}

	// Remove marked spans from their scope spans
	for _, group := range plan.groups {
		for _, info := range group.spans {
			info.scopeSpans.Spans().RemoveIf(func(span ptrace.Span) bool {
				return span.Name() == ""
			})
		}
	}
}

// createSummarySpanWithParent creates a summary span with a specific parent SpanID
func (p *leafSpanPruningProcessor) createSummarySpanWithParent(group aggregationGroup, stats aggregationStats, parentSpanID pcommon.SpanID) ptrace.Span {
	// Use the first span as a template
	templateSpan := group.spans[0].span
	scopeSpans := group.spans[0].scopeSpans

	// Create new span in the same ScopeSpans as the first span
	newSpan := scopeSpans.Spans().AppendEmpty()

	// Copy basic properties from template
	newSpan.SetName(templateSpan.Name() + p.config.SummarySpanNameSuffix)
	newSpan.SetTraceID(templateSpan.TraceID())
	newSpan.SetSpanID(group.summarySpanID)
	newSpan.SetParentSpanID(parentSpanID)
	newSpan.SetKind(templateSpan.Kind())

	// Set timestamps: earliest start, latest end
	earliestStart, latestEnd := p.findTimeRange(group.spans)
	newSpan.SetStartTimestamp(earliestStart)
	newSpan.SetEndTimestamp(latestEnd)

	// Copy attributes from template
	templateSpan.Attributes().CopyTo(newSpan.Attributes())

	// Copy status from template
	templateSpan.Status().CopyTo(newSpan.Status())

	// Add aggregation statistics as attributes
	prefix := p.config.AggregationAttributePrefix
	newSpan.Attributes().PutInt(prefix+"span_count", stats.count)
	newSpan.Attributes().PutInt(prefix+"duration_min_ns", int64(stats.minDuration))
	newSpan.Attributes().PutInt(prefix+"duration_max_ns", int64(stats.maxDuration))
	newSpan.Attributes().PutInt(prefix+"duration_total_ns", int64(stats.sumDuration))
	if stats.count > 0 {
		newSpan.Attributes().PutInt(prefix+"duration_avg_ns", int64(stats.sumDuration)/stats.count)
	}

	// Add histogram attributes if enabled
	if len(p.config.AggregationHistogramBuckets) > 0 {
		// Add bucket bounds (in seconds)
		bucketBoundsSlice := newSpan.Attributes().PutEmptySlice(prefix + "histogram_bucket_bounds_s")
		for _, bucket := range p.config.AggregationHistogramBuckets {
			bucketBoundsSlice.AppendEmpty().SetDouble(float64(bucket) / float64(time.Second))
		}

		// Add bucket counts
		bucketCountsSlice := newSpan.Attributes().PutEmptySlice(prefix + "histogram_bucket_counts")
		for _, count := range stats.bucketCounts {
			bucketCountsSlice.AppendEmpty().SetInt(count)
		}
	}

	return newSpan
}
