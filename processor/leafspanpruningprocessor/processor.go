// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leafspanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor"

import (
	"context"
	"fmt"

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

// attributePattern holds a compiled glob pattern for matching attribute keys
type attributePattern struct {
	glob glob.Glob
}

// leafSpanPruningProcessor is the leaf span pruning processor implementation
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
		if err := p.processTrace(ctx, spans); err != nil {
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

// processTrace processes a single trace using two-phase approach:
// Phase 1: Analyze aggregations bottom-up (identify leaf groups, then eligible parents)
// Phase 2: Execute aggregations top-down (create parent summaries first, then children)
func (p *leafSpanPruningProcessor) processTrace(ctx context.Context, spans []spanInfo) error {
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

// analyzeAggregationsWithTree performs Phase 1 using tree structure
// Uses markedForRemoval field on nodes instead of separate map for better performance
func (p *leafSpanPruningProcessor) analyzeAggregationsWithTree(tree *traceTree) map[string]aggregationGroup {
	// Step 1: Get pre-computed leaf nodes
	leafNodes := tree.getLeaves()
	if len(leafNodes) == 0 {
		return nil
	}

	// Step 2: Group similar leaf nodes
	leafGroups := p.groupLeafNodesByKey(leafNodes)

	// Step 3: Filter groups meeting minimum threshold and mark nodes
	// Pre-size based on expected number of groups
	aggregationGroups := make(map[string]aggregationGroup, len(leafGroups)/2)

	for groupKey, nodes := range leafGroups {
		if len(nodes) >= p.config.MinSpansToAggregate {
			aggregationGroups[groupKey] = aggregationGroup{
				nodes: nodes,
				level: 0,
			}
			// Mark nodes for removal using field instead of map
			for _, node := range nodes {
				node.markedForRemoval = true
			}
		}
	}

	if len(aggregationGroups) == 0 {
		return nil
	}

	// Step 4: Walk up the tree to find eligible parent spans recursively
	level := 1
	for {
		parentCandidates := p.findEligibleParentNodes(tree)
		if len(parentCandidates) == 0 {
			break
		}

		// Group parent candidates by name + status
		parentGroups := make(map[string][]*spanNode)
		for _, node := range parentCandidates {
			parentKey := p.buildParentGroupKey(node.span)
			parentGroups[parentKey] = append(parentGroups[parentKey], node)
		}

		// Add parent groups (at least 2 parents to aggregate)
		foundNewParents := false
		for parentKey, nodes := range parentGroups {
			if len(nodes) >= 2 {
				aggregationGroups[parentKey] = aggregationGroup{
					nodes: nodes,
					level: level,
				}
				// Mark parent nodes for removal
				for _, node := range nodes {
					node.markedForRemoval = true
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
