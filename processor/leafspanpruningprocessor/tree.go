// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leafspanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// spanNode represents a span in the trace tree with parent/child relationships
// groupKey is computed once during tree construction to avoid repeated string allocations
type spanNode struct {
	span             ptrace.Span
	scopeSpans       ptrace.ScopeSpans
	parent           *spanNode
	children         []*spanNode
	groupKey         string // cached group key for leaf spans
	isLeaf           bool   // true if node has no children (updated during tree build)
	markedForRemoval bool   // true if node will be aggregated (avoids map lookups)
}

// traceTree represents a complete trace as a tree structure
type traceTree struct {
	nodeByID map[pcommon.SpanID]*spanNode
	leaves   []*spanNode // nodes with no children, populated during build
	orphans  []*spanNode // spans whose parent is not in the trace
}

// buildTraceTree constructs a tree structure from a list of spans
// Handles incomplete traces: orphans (missing parents), multiple roots, no root
// Also identifies leaf nodes during construction to avoid a separate pass
func (p *leafSpanPruningProcessor) buildTraceTree(spans []spanInfo) *traceTree {
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
	tree.leaves = make([]*spanNode, 0, len(spans)/4)
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

// getLeaves returns the pre-computed list of leaf nodes
func (t *traceTree) getLeaves() []*spanNode {
	return t.leaves
}

// findEligibleParentNodes finds parent nodes whose ALL children are marked for removal
// Uses the markedForRemoval field on nodes instead of map lookups for better performance
func (p *leafSpanPruningProcessor) findEligibleParentNodes(tree *traceTree) []*spanNode {
	eligibleParents := make([]*spanNode, 0, len(tree.nodeByID)/10)

	for _, node := range tree.nodeByID {
		if !p.isEligibleForParentAggregation(node) {
			continue
		}
		eligibleParents = append(eligibleParents, node)
	}

	return eligibleParents
}

// isEligibleForParentAggregation checks if a node can be aggregated as a parent
func (p *leafSpanPruningProcessor) isEligibleForParentAggregation(node *spanNode) bool {
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
