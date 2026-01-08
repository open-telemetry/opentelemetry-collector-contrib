// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// spanNode represents a span in the trace tree with parent/child relationships.
type spanNode struct {
	span             ptrace.Span
	scopeSpans       ptrace.ScopeSpans
	parent           *spanNode
	children         []*spanNode
	groupKey         string // cached group key for leaf spans
	isLeaf           bool   // true if node has no children
	markedForRemoval bool   // true if node will be aggregated
}

// traceTree represents a complete trace as a tree structure.
type traceTree struct {
	nodeByID map[pcommon.SpanID]*spanNode
	leaves   []*spanNode // nodes with no children, populated during build
	orphans  []*spanNode // spans whose parent is not in the trace
}

// buildTraceTree constructs a tree structure from a list of spans.
// Handles incomplete traces: orphans (missing parents), multiple roots, no root.
func (p *spanPruningProcessor) buildTraceTree(spans []spanInfo) *traceTree {
	tree := &traceTree{
		nodeByID: make(map[pcommon.SpanID]*spanNode, len(spans)),
	}

	if len(spans) == 0 {
		return tree
	}

	// First pass: create nodes for all spans, initially mark all as leaves
	for _, info := range spans {
		node := &spanNode{
			span:       info.span,
			scopeSpans: info.scopeSpans,
			isLeaf:     true, // assume leaf until a child links to it
		}
		tree.nodeByID[info.span.SpanID()] = node
	}

	// Second pass: link parent-child relationships and update leaf status
	// Pre-allocate slices with reasonable capacity
	tree.orphans = make([]*spanNode, 0, len(spans)/10)
	var rootCount int

	for _, node := range tree.nodeByID {
		parentID := node.span.ParentSpanID()
		if parentID.IsEmpty() {
			// This is a root span (no parent)
			rootCount++
		} else if parent, exists := tree.nodeByID[parentID]; exists {
			// Link to parent and mark parent as non-leaf
			node.parent = parent
			parent.isLeaf = false
			if parent.children == nil {
				parent.children = make([]*spanNode, 0, 4)
			}
			parent.children = append(parent.children, node)
		} else {
			// Parent not in trace - this is an orphan
			tree.orphans = append(tree.orphans, node)
		}
	}

	// Third pass: collect leaves (nodes still marked as leaf)
	tree.leaves = make([]*spanNode, 0, len(spans)/4)
	for _, node := range tree.nodeByID {
		if node.isLeaf {
			tree.leaves = append(tree.leaves, node)
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

// getLeaves returns the pre-computed list of leaf nodes.
func (t *traceTree) getLeaves() []*spanNode {
	return t.leaves
}

// findEligibleParentNodesFromCandidates finds eligible parents from a set of candidates.
// More efficient than scanning all nodes when walking up from marked nodes.
func (p *spanPruningProcessor) findEligibleParentNodesFromCandidates(candidates []*spanNode) []*spanNode {
	if len(candidates) == 0 {
		return nil
	}

	eligibleParents := make([]*spanNode, 0, len(candidates)/4)
	for _, node := range candidates {
		if p.isEligibleForParentAggregation(node) {
			eligibleParents = append(eligibleParents, node)
		}
	}
	return eligibleParents
}

// collectParentCandidates returns unique parents of marked nodes for the next iteration.
func collectParentCandidates(markedNodes []*spanNode) []*spanNode {
	if len(markedNodes) == 0 {
		return nil
	}

	seen := make(map[*spanNode]struct{}, len(markedNodes)/2)
	candidates := make([]*spanNode, 0, len(markedNodes)/2)

	for _, node := range markedNodes {
		if node.parent != nil {
			if _, exists := seen[node.parent]; !exists {
				seen[node.parent] = struct{}{}
				candidates = append(candidates, node.parent)
			}
		}
	}

	return candidates
}

// isEligibleForParentAggregation checks if a node can be aggregated as a parent.
func (p *spanPruningProcessor) isEligibleForParentAggregation(node *spanNode) bool {
	// Must have children (not a leaf)
	if node.isLeaf {
		return false
	}

	// Must have a parent (not root)
	if node.parent == nil {
		return false
	}

	// Must not already be marked for removal
	if node.markedForRemoval {
		return false
	}

	// All children must be marked for removal
	for _, child := range node.children {
		if !child.markedForRemoval {
			return false
		}
	}

	return true
}
