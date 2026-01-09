// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

import (
	"encoding/binary"
	"math/rand/v2"
	"sort"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// aggregationGroup represents a group of spans to be aggregated
type aggregationGroup struct {
	nodes         []*spanNode          // nodes to aggregate (replaces []spanInfo for efficiency)
	depth         int                  // tree depth (0 = leaf, 1 = parent of leaf, etc.)
	summarySpanID pcommon.SpanID       // SpanID of the summary span (assigned before creation)
	lossInfo      attributeLossSummary // attribute loss info (diverse + missing)
}

// aggregationPlan holds all aggregations ordered for top-down execution
type aggregationPlan struct {
	groups []aggregationGroup
}

// generateSpanID creates a new random span ID using math/rand for performance.
// Span IDs don't require cryptographic randomness, just uniqueness.
func generateSpanID() pcommon.SpanID {
	var id [8]byte
	binary.BigEndian.PutUint64(id[:], rand.Uint64())
	return pcommon.SpanID(id)
}

// buildAggregationPlan orders aggregation groups for top-down execution
func (p *spanPruningProcessor) buildAggregationPlan(groups map[string]aggregationGroup) aggregationPlan {
	// Convert map to slice with pre-allocation
	groupSlice := make([]aggregationGroup, 0, len(groups))
	for _, group := range groups {
		groupSlice = append(groupSlice, group)
	}

	// Sort by depth descending (highest depth first = top-down)
	sort.Slice(groupSlice, func(i, j int) bool {
		return groupSlice[i].depth > groupSlice[j].depth
	})

	// Pre-assign SpanIDs for all summary spans
	for i := range groupSlice {
		groupSlice[i].summarySpanID = generateSpanID()
	}

	return aggregationPlan{groups: groupSlice}
}

// executeAggregations performs Phase 2: top-down creation of summary spans
// Optimized to batch span removals instead of calling RemoveIf repeatedly
// Returns the count of spans that were pruned (aggregated)
func (p *spanPruningProcessor) executeAggregations(plan aggregationPlan) int {
	// Track which parent SpanID should map to which summary SpanID
	// Pre-size based on expected number of nodes being aggregated
	parentReplacements := make(map[pcommon.SpanID]pcommon.SpanID, len(plan.groups)*4)

	// Track spans to remove per ScopeSpans for batch removal
	spansToRemove := make(map[ptrace.ScopeSpans]map[pcommon.SpanID]struct{})
	prunedCount := 0

	for _, group := range plan.groups {
		// Calculate statistics and time range in single pass
		data := p.calculateAggregationData(group.nodes)

		// Determine the parent SpanID for the summary span
		// Use the first node's parent as template
		originalParentID := group.nodes[0].span.ParentSpanID()

		// Check if the parent is being replaced by a summary span
		summaryParentID := originalParentID
		if replacementID, exists := parentReplacements[originalParentID]; exists {
			summaryParentID = replacementID
		}

		// Create summary span with correct parent
		p.createSummarySpanWithParent(group, data, summaryParentID)

		// Record that these original span IDs should be replaced by the summary span ID
		for _, node := range group.nodes {
			parentReplacements[node.span.SpanID()] = group.summarySpanID
		}

		// Mark original spans for removal (batch per ScopeSpans)
		for _, node := range group.nodes {
			scopeSpans := node.scopeSpans
			if spansToRemove[scopeSpans] == nil {
				spansToRemove[scopeSpans] = make(map[pcommon.SpanID]struct{})
			}
			spansToRemove[scopeSpans][node.span.SpanID()] = struct{}{}
		}
		prunedCount += len(group.nodes)
	}

	// Batch remove all marked spans in a single pass per ScopeSpans
	for scopeSpans, spanIDs := range spansToRemove {
		scopeSpans.Spans().RemoveIf(func(span ptrace.Span) bool {
			_, shouldRemove := spanIDs[span.SpanID()]
			return shouldRemove
		})
	}

	return prunedCount
}

// createSummarySpanWithParent creates a summary span with a specific parent SpanID
func (p *spanPruningProcessor) createSummarySpanWithParent(group aggregationGroup, data aggregationData, parentSpanID pcommon.SpanID) ptrace.Span {
	// Use the first node as a template
	templateNode := group.nodes[0]
	templateSpan := templateNode.span
	scopeSpans := templateNode.scopeSpans

	// Create new span in the same ScopeSpans as the first span
	newSpan := scopeSpans.Spans().AppendEmpty()

	// Copy basic properties from template
	newSpan.SetName(templateSpan.Name() + p.config.AggregationSpanNameSuffix)
	newSpan.SetTraceID(templateSpan.TraceID())
	newSpan.SetSpanID(group.summarySpanID)
	newSpan.SetParentSpanID(parentSpanID)
	newSpan.SetKind(templateSpan.Kind())

	// Set timestamps from aggregation data
	newSpan.SetStartTimestamp(data.earliestStart)
	newSpan.SetEndTimestamp(data.latestEnd)

	// Copy attributes from template
	templateSpan.Attributes().CopyTo(newSpan.Attributes())

	// Copy status from template
	templateSpan.Status().CopyTo(newSpan.Status())

	// Add aggregation statistics as attributes
	prefix := p.config.AggregationAttributePrefix
	newSpan.Attributes().PutInt(prefix+"span_count", data.count)
	newSpan.Attributes().PutInt(prefix+"duration_min_ns", int64(data.minDuration))
	newSpan.Attributes().PutInt(prefix+"duration_max_ns", int64(data.maxDuration))
	newSpan.Attributes().PutInt(prefix+"duration_total_ns", int64(data.sumDuration))
	if data.count > 0 {
		newSpan.Attributes().PutInt(prefix+"duration_avg_ns", int64(data.sumDuration)/data.count)
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
		for _, count := range data.bucketCounts {
			bucketCountsSlice.AppendEmpty().SetInt(count)
		}
	}

	// Add attribute loss info when detected
	if len(group.lossInfo.diverse) > 0 {
		newSpan.Attributes().PutStr(prefix+"diverse_attributes", formatAttributeCardinality(group.lossInfo.diverse))
	}
	if len(group.lossInfo.missing) > 0 {
		newSpan.Attributes().PutStr(prefix+"missing_attributes", formatAttributeCardinality(group.lossInfo.missing))
	}

	return newSpan
}
