// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/gobwas/glob"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
)

// spanInfo pairs a span with its ScopeSpans container for in-place edits.
type spanInfo struct {
	span       ptrace.Span
	scopeSpans ptrace.ScopeSpans
}

// attributePattern caches a compiled glob used for attribute key matching.
type attributePattern struct {
	glob glob.Glob
}

// spanPruningProcessor aggregates similar leaf spans (and eligible parents)
// according to configuration while emitting telemetry about pruning actions.
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

// shutdown releases processor resources, including telemetry providers.
func (p *spanPruningProcessor) shutdown(_ context.Context) error {
	p.telemetryBuilder.Shutdown()
	return nil
}

// shouldSampleAttributeLossExemplar decides whether to attach exemplars to
// attribute-loss metrics based on the configured sampling rate.
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

// processTraces runs aggregation for each trace batch and records processor
// telemetry about received, pruned, and aggregated spans.
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
		p.processTrace(ctx, spans)
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

// groupSpansByTraceID flattens incoming data into a TraceID-indexed map so
// each trace can be analyzed independently.
func (*spanPruningProcessor) groupSpansByTraceID(td ptrace.Traces) map[pcommon.TraceID][]spanInfo {
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

// processTrace applies the pruning algorithm to a single trace:
// 1) analyze aggregation candidates bottom-up, 2) build a top-down execution
// plan, and 3) create summary spans while removing originals.
func (p *spanPruningProcessor) processTrace(ctx context.Context, spans []spanInfo) {
	// Build trace tree
	tree := p.buildTraceTree(spans)
	if len(tree.nodeByID) == 0 {
		return
	}

	// Phase 1: Analyze aggregations (bottom-up)
	aggregationGroups := p.analyzeAggregationsWithTree(ctx, tree)
	if len(aggregationGroups) == 0 {
		return
	}

	// Phase 2: Build aggregation plan (order top-down)
	plan := p.buildAggregationPlan(aggregationGroups)

	// Phase 3: Execute aggregations (top-down) and record pruned spans
	prunedCount := p.executeAggregations(plan, tree)

	// Record telemetry after aggregation is complete
	p.telemetryBuilder.ProcessorSpanpruningSpansPruned.Add(ctx, int64(prunedCount))
	p.telemetryBuilder.ProcessorSpanpruningAggregationsCreated.Add(ctx, int64(len(plan.groups)))
	for i := range plan.groups {
		p.telemetryBuilder.ProcessorSpanpruningAggregationGroupSize.Record(ctx, int64(len(plan.groups[i].nodes)))
	}
}

// analyzeAggregationsWithTree plans the candidate aggregation groups, then
// detects and protects outliers within them, then aggregates the non-protected
// spans. Planning runs once and feeds both detection and execution.
func (p *spanPruningProcessor) analyzeAggregationsWithTree(ctx context.Context, tree *traceTree) map[string]aggregationGroup {
	// Step 1: plan the groups that would aggregate.
	groups := p.planCandidateGroups(tree)
	if len(groups) == 0 {
		return nil
	}

	// Step 2: detect outliers over those groups and protect their subtrees
	// before any aggregation runs.
	outlierResultByKey, protectedRootsByKey := p.detectAndProtectOutliers(ctx, groups)

	// Step 3: aggregate. planCandidateGroups returns groups bottom-up (leaves
	// first, then parents by increasing depth), so processing them in order
	// marks a group's children before the parent group is evaluated.
	aggregationGroups := make(map[string]aggregationGroup, len(groups))
	preserving := p.config.EnableOutlierAnalysis && p.config.OutlierAnalysis.PreserveOutliers

	for _, g := range groups {
		nodes := g.nodes
		if g.depth == 0 {
			// Leaf group: drop preserved-outlier subtrees, then re-check the floor.
			if preserving {
				nodes = excludeProtectedNodes(nodes)
			}
			if len(nodes) < p.config.MinSpansToAggregate {
				continue
			}
		} else {
			// Parent group: protection, or a child group that fell below the
			// floor, can leave a planned parent with an un-aggregated child, so
			// re-check eligibility against the marks set by the deeper groups.
			nodes = p.eligibleParents(nodes)
			if len(nodes) < 2 {
				continue
			}
		}

		templateNode := findLongestDurationNode(nodes)
		lossInfo := p.recordAttributeLoss(ctx, g.depth == 0, nodes, templateNode)

		preserved := protectedRootsByKey[g.key]
		aggregationGroups[g.key] = aggregationGroup{
			nodes:             nodes,
			depth:             g.depth,
			lossInfo:          lossInfo,
			templateNode:      templateNode,
			outlierAnalysis:   outlierResultByKey[g.key],
			preservedOutliers: preserved,
		}
		for _, n := range nodes {
			n.markedForRemoval = true
		}
		p.recordPreserved(ctx, preserved)
	}

	return aggregationGroups
}

// eligibleParents returns the parents of a planned parent group that are still
// eligible to aggregate, in a stable order so the summary's parent is chosen
// deterministically. Protection, or a child group that fell below
// MinSpansToAggregate, can make a planned parent ineligible by the time it is
// executed.
func (p *spanPruningProcessor) eligibleParents(nodes []*spanNode) []*spanNode {
	out := make([]*spanNode, 0, len(nodes))
	for _, n := range nodes {
		if p.isEligibleForParentAggregation(n) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return nodeOrderLess(out[i], out[j])
	})
	return out
}

// recordAttributeLoss computes attribute loss for a group and records the
// matching leaf or parent telemetry, returning the loss summary.
func (p *spanPruningProcessor) recordAttributeLoss(ctx context.Context, isLeaf bool, nodes []*spanNode, templateNode *spanNode) attributeLossSummary {
	if !p.enableAttributeLossAnalysis {
		return attributeLossSummary{}
	}
	lossInfo := analyzeAttributeLoss(nodes, templateNode)
	if lossInfo.isEmpty() {
		return lossInfo
	}
	recordCtx := ctx
	if p.shouldSampleAttributeLossExemplar() {
		recordCtx = createExemplarContext(ctx, templateNode.span.TraceID(), templateNode.span.SpanID())
	}
	if isLeaf {
		p.telemetryBuilder.ProcessorSpanpruningLeafAttributeDiversityLoss.Record(recordCtx, int64(len(lossInfo.diverse)))
		p.telemetryBuilder.ProcessorSpanpruningLeafAttributeLoss.Record(recordCtx, int64(len(lossInfo.missing)))
	} else {
		p.telemetryBuilder.ProcessorSpanpruningParentAttributeDiversityLoss.Record(recordCtx, int64(len(lossInfo.diverse)))
		p.telemetryBuilder.ProcessorSpanpruningParentAttributeLoss.Record(recordCtx, int64(len(lossInfo.missing)))
	}
	return lossInfo
}

// detectAndProtectOutliers runs outlier detection over the planned groups and,
// when preservation is enabled, protects each preserved outlier's whole subtree
// so it is kept instead of aggregated. It returns per-group detection results (to
// annotate the matching summary) and the preserved-outlier roots grouped by group
// key (to link them to their summary during execution).
func (p *spanPruningProcessor) detectAndProtectOutliers(ctx context.Context, groups []candidateGroup) (map[string]*outlierAnalysisResult, map[string][]*spanNode) {
	if !p.config.EnableOutlierAnalysis {
		return nil, nil
	}

	outlierResultByKey := make(map[string]*outlierAnalysisResult)
	protectedRootsByKey := make(map[string][]*spanNode)
	preserve := p.config.OutlierAnalysis.PreserveOutliers

	// Process groups shallowest-first (by tree depth) so an interior outlier's
	// subtree protection lands before any group nested beneath it; a leaf outlier
	// inside an already-protected subtree is then skipped rather than preserved
	// (and counted) twice. Sort a copy so the caller keeps its bottom-up order.
	ordered := append([]candidateGroup(nil), groups...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].nodes[0].depth() < ordered[j].nodes[0].depth()
	})

	for _, group := range ordered {
		res := analyzeOutliers(group.nodes, p.config.OutlierAnalysis)
		if res == nil {
			continue
		}
		outlierResultByKey[group.key] = res

		if res.hasOutliers {
			p.telemetryBuilder.ProcessorSpanpruningOutliersDetected.Add(ctx, int64(len(res.outlierIndices)))
			if len(res.correlations) > 0 {
				p.telemetryBuilder.ProcessorSpanpruningOutliersCorrelationsDetected.Add(ctx, 1)
			}
		}

		if !preserve {
			continue
		}
		// filterOutlierNodes applies correlation gating and orders outliers most
		// extreme first. Ask it for all of them (cap 0) and apply
		// MaxPreservedOutliers here, after dropping outliers already kept by an
		// enclosing protected subtree. Letting filterOutlierNodes cap first would
		// spend a budget slot on an already-covered outlier and starve a
		// not-yet-preserved one further down the order.
		unlimited := p.config.OutlierAnalysis
		unlimited.MaxPreservedOutliers = 0
		_, outliers := filterOutlierNodes(group.nodes, res, unlimited)
		limit := p.config.OutlierAnalysis.MaxPreservedOutliers
		for _, o := range outliers {
			// Skip outliers already kept by an enclosing protected subtree.
			if o.protected {
				continue
			}
			if limit > 0 && len(protectedRootsByKey[group.key]) >= limit {
				break
			}
			markOutlierSubtree(o)
			protectedRootsByKey[group.key] = append(protectedRootsByKey[group.key], o)
		}
	}
	return outlierResultByKey, protectedRootsByKey
}

// candidateGroup is a group of sibling spans that would aggregate, paired with
// the key and aggregation depth the executor will use for it (depth 0 for a leaf
// group, >=1 for a parent group). Carrying these from planning means detection
// and execution agree without recomputing them.
type candidateGroup struct {
	key   string
	depth int
	nodes []*spanNode
}

// planCandidateGroups computes the groups that would aggregate (leaf groups at
// or above MinSpansToAggregate and eligible parent groups), ignoring outlier
// protection. It mutates no node state, tracking would-aggregate membership in a
// local set, so detection runs over the same groups the executor later forms.
func (p *spanPruningProcessor) planCandidateGroups(tree *traceTree) []candidateGroup {
	leafNodes := tree.getLeaves()
	if len(leafNodes) == 0 {
		return nil
	}

	var groups []candidateGroup
	would := make(map[*spanNode]bool)
	var marked []*spanNode

	for key, nodes := range p.groupLeafNodesByKey(leafNodes) {
		if len(nodes) < p.config.MinSpansToAggregate {
			continue
		}
		groups = append(groups, candidateGroup{key: key, depth: 0, nodes: nodes})
		for _, n := range nodes {
			would[n] = true
			marked = append(marked, n)
		}
	}

	if p.config.MaxParentDepth == 0 {
		return groups
	}

	candidates := collectParentCandidates(marked)
	depth := 1
	for len(candidates) > 0 {
		if p.config.MaxParentDepth > 0 && depth > p.config.MaxParentDepth {
			break
		}

		var eligible []*spanNode
		for _, n := range candidates {
			if planEligibleForParentAggregation(n, would) {
				eligible = append(eligible, n)
			}
		}
		if len(eligible) == 0 {
			break
		}

		parentGroups := make(map[string][]*spanNode)
		for _, n := range eligible {
			key := p.buildParentGroupKey(n.span, n.depth())
			parentGroups[key] = append(parentGroups[key], n)
		}

		marked = marked[:0]
		for key, nodes := range parentGroups {
			if len(nodes) < 2 {
				continue
			}
			groups = append(groups, candidateGroup{key: key, depth: depth, nodes: nodes})
			for _, n := range nodes {
				would[n] = true
				marked = append(marked, n)
			}
		}
		if len(marked) == 0 {
			break
		}
		candidates = collectParentCandidates(marked)
		depth++
	}
	return groups
}

// recordPreserved emits preserved-outlier telemetry: every span kept across the
// preserved subtrees rooted at the given outlier roots, not just the roots.
func (p *spanPruningProcessor) recordPreserved(ctx context.Context, roots []*spanNode) {
	if len(roots) == 0 {
		return
	}
	p.telemetryBuilder.ProcessorSpanpruningOutliersPreserved.Add(ctx, int64(preservedSpanCount(roots)))
}

// planEligibleForParentAggregation mirrors isEligibleForParentAggregation for
// the planning phase, using the local would-aggregate set rather than node
// flags and ignoring protection (not yet decided during planning).
func planEligibleForParentAggregation(node *spanNode, would map[*spanNode]bool) bool {
	if node.isLeaf || node.parent == nil || would[node] {
		return false
	}
	for _, child := range node.children {
		if !would[child] {
			return false
		}
	}
	return true
}

// excludeProtectedNodes returns the subset of nodes that are not protected,
// allocating only when at least one node is protected.
func excludeProtectedNodes(nodes []*spanNode) []*spanNode {
	protectedCount := 0
	for _, n := range nodes {
		if n.protected {
			protectedCount++
		}
	}
	if protectedCount == 0 {
		return nodes
	}
	out := make([]*spanNode, 0, len(nodes)-protectedCount)
	for _, n := range nodes {
		if !n.protected {
			out = append(out, n)
		}
	}
	return out
}
