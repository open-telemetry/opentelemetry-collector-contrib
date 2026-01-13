// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/gobwas/glob"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
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

// spanPruningProcessor is the leaf span pruning processor implementation
type spanPruningProcessor struct {
	config                      *Config
	logger                      *zap.Logger
	attributePatterns           []attributePattern
	telemetryBuilder            *metadata.TelemetryBuilder
	enableAttributeLossAnalysis bool
}

func newSpanPruningProcessor(set processor.Settings, cfg *Config, telemetryBuilder *metadata.TelemetryBuilder) (*spanPruningProcessor, error) {
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

	return &spanPruningProcessor{
		config:                      cfg,
		logger:                      set.Logger,
		attributePatterns:           patterns,
		telemetryBuilder:            telemetryBuilder,
		enableAttributeLossAnalysis: cfg.EnableAttributeLossAnalysis,
	}, nil
}

// shutdown is called when the processor is being stopped
func (p *spanPruningProcessor) shutdown(_ context.Context) error {
	p.telemetryBuilder.Shutdown()
	return nil
}

// shouldSampleAttributeLossExemplar returns true if this recording should include an exemplar
func (p *spanPruningProcessor) shouldSampleAttributeLossExemplar() bool {
	rate := p.config.AttributeLossExemplarSampleRate
	if rate <= 0 {
		return false
	}
	if rate >= 1 {
		return true
	}
	return rand.Float64() < rate
}

// createExemplarContext creates a context with span context for exemplar attachment.
// Uses direct type casting since pcommon and trace ID types are identical byte arrays.
func createExemplarContext(ctx context.Context, traceID pcommon.TraceID, spanID pcommon.SpanID) context.Context {
	return trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID(traceID),
		SpanID:     trace.SpanID(spanID),
		TraceFlags: trace.FlagsSampled,
	}))
}

func (p *spanPruningProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	start := time.Now()

	// Count incoming spans
	totalSpans := int64(0)
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		for j := 0; j < td.ResourceSpans().At(i).ScopeSpans().Len(); j++ {
			totalSpans += int64(td.ResourceSpans().At(i).ScopeSpans().At(j).Spans().Len())
		}
	}
	p.telemetryBuilder.ProcessorSpanpruningSpansReceived.Add(ctx, totalSpans)

	// Group spans by TraceID
	traceSpans := p.groupSpansByTraceID(td)

	// Process each trace independently
	tracesProcessed := int64(0)
	for _, spans := range traceSpans {
		if err := p.processTrace(ctx, spans); err != nil {
			return td, err
		}
		tracesProcessed++
	}

	// Record telemetry only when actual work was done
	if tracesProcessed > 0 {
		p.telemetryBuilder.ProcessorSpanpruningTracesProcessed.Add(ctx, tracesProcessed)
		p.telemetryBuilder.ProcessorSpanpruningProcessingDuration.Record(ctx,
			time.Since(start).Seconds())
	}

	return td, nil
}

// groupSpansByTraceID collects all spans organized by trace ID
func (p *spanPruningProcessor) groupSpansByTraceID(td ptrace.Traces) map[pcommon.TraceID][]spanInfo {
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
func (p *spanPruningProcessor) processTrace(ctx context.Context, spans []spanInfo) error {
	// Build trace tree
	tree := p.buildTraceTree(spans)
	if len(tree.nodeByID) == 0 {
		return nil
	}

	// Phase 1: Analyze aggregations (bottom-up)
	aggregationGroups := p.analyzeAggregationsWithTree(ctx, tree)
	if len(aggregationGroups) == 0 {
		return nil
	}

	// Phase 2: Build aggregation plan (order top-down)
	plan := p.buildAggregationPlan(aggregationGroups)

	// Phase 3: Execute aggregations (top-down) and record pruned spans
	prunedCount := p.executeAggregations(plan)

	// Record telemetry after aggregation is complete
	p.telemetryBuilder.ProcessorSpanpruningSpansPruned.Add(ctx, int64(prunedCount))
	p.telemetryBuilder.ProcessorSpanpruningAggregationsCreated.Add(ctx, int64(len(plan.groups)))
	for _, group := range plan.groups {
		p.telemetryBuilder.ProcessorSpanpruningAggregationGroupSize.Record(ctx, int64(len(group.nodes)))
	}

	return nil
}

// analyzeAggregationsWithTree performs Phase 1 using tree structure
// Uses markedForRemoval field on nodes instead of separate map for better performance
// Optimized to walk up from marked nodes instead of scanning all nodes
func (p *spanPruningProcessor) analyzeAggregationsWithTree(ctx context.Context, tree *traceTree) map[string]aggregationGroup {
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

	// Track nodes marked in this round for candidate collection
	var markedNodes []*spanNode

	for groupKey, nodes := range leafGroups {
		if len(nodes) >= p.config.MinSpansToAggregate {
			// Analyze attribute loss for leaf aggregation (only when enabled)
			var lossInfo attributeLossSummary
			if p.enableAttributeLossAnalysis {
				lossInfo = analyzeAttributeLoss(nodes)

				// Determine context for recording (with or without exemplar)
				recordCtx := ctx
				if p.shouldSampleAttributeLossExemplar() {
					exemplarSpan := nodes[0].span
					recordCtx = createExemplarContext(ctx, exemplarSpan.TraceID(), exemplarSpan.SpanID())
				}

				if !lossInfo.isEmpty() {
					p.telemetryBuilder.ProcessorSpanpruningLeafAttributeDiversityLoss.Record(
						recordCtx,
						int64(len(lossInfo.diverse)),
					)
					p.telemetryBuilder.ProcessorSpanpruningLeafAttributeLoss.Record(
						recordCtx,
						int64(len(lossInfo.missing)),
					)
				}
			}

			aggregationGroups[groupKey] = aggregationGroup{
				nodes:    nodes,
				depth:    0,
				lossInfo: lossInfo,
			}
			// Mark nodes for removal using field instead of map
			for _, node := range nodes {
				node.markedForRemoval = true
			}
			markedNodes = append(markedNodes, nodes...)
		}
	}

	if len(aggregationGroups) == 0 {
		return nil
	}

	// Step 4: Walk up the tree to find eligible parent spans recursively
	// Respect MaxParentDepth: 0 = no parent aggregation, -1 = unlimited, >0 = limit
	if p.config.MaxParentDepth == 0 {
		return aggregationGroups
	}

	// Collect initial parent candidates from marked leaf nodes
	candidates := collectParentCandidates(markedNodes)

	depth := 1
	for len(candidates) > 0 {
		// Check if we've reached the maximum parent depth limit
		if p.config.MaxParentDepth > 0 && depth > p.config.MaxParentDepth {
			break
		}

		// Find eligible parents from candidates (walks up from marked nodes)
		eligibleParents := p.findEligibleParentNodesFromCandidates(candidates)
		if len(eligibleParents) == 0 {
			break
		}

		// Group parent candidates by name + status
		parentGroups := make(map[string][]*spanNode)
		for _, node := range eligibleParents {
			parentKey := p.buildParentGroupKey(node.span)
			parentGroups[parentKey] = append(parentGroups[parentKey], node)
		}

		// Add parent groups (at least 2 parents to aggregate)
		markedNodes = markedNodes[:0] // reset for this round
		for parentKey, nodes := range parentGroups {
			if len(nodes) >= 2 {
				// Analyze attribute loss for parent aggregation (only when enabled)
				var lossInfo attributeLossSummary
				if p.enableAttributeLossAnalysis {
					lossInfo = analyzeAttributeLoss(nodes)

					// Determine context for recording (with or without exemplar)
					recordCtx := ctx
					if p.shouldSampleAttributeLossExemplar() {
						exemplarSpan := nodes[0].span
						recordCtx = createExemplarContext(ctx, exemplarSpan.TraceID(), exemplarSpan.SpanID())
					}

					if !lossInfo.isEmpty() {
						p.telemetryBuilder.ProcessorSpanpruningParentAttributeDiversityLoss.Record(
							recordCtx,
							int64(len(lossInfo.diverse)),
						)
						p.telemetryBuilder.ProcessorSpanpruningParentAttributeLoss.Record(
							recordCtx,
							int64(len(lossInfo.missing)),
						)
					}
				}

				aggregationGroups[parentKey] = aggregationGroup{
					nodes:    nodes,
					depth:    depth,
					lossInfo: lossInfo,
				}
				// Mark parent nodes for removal
				for _, node := range nodes {
					node.markedForRemoval = true
				}
				markedNodes = append(markedNodes, nodes...)
			}
		}

		if len(markedNodes) == 0 {
			break
		}

		// Collect next round of candidates from newly marked nodes
		candidates = collectParentCandidates(markedNodes)
		depth++
	}

	return aggregationGroups
}
