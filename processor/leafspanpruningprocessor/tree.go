// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leafspanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

// spanNode represents a span in the trace tree with parent/child relationships
// groupKey is computed once during tree construction to avoid repeated string allocations
type spanNode struct {
	info     spanInfo
	parent   *spanNode
	children []*spanNode
	groupKey string // cached group key for leaf spans
}

// traceTree represents a complete trace as a tree structure
type traceTree struct {
	nodeByID map[pcommon.SpanID]*spanNode
	orphans  []*spanNode // spans whose parent is not in the trace
}

// buildTraceTree constructs a tree structure from a list of spans
// Handles incomplete traces: orphans (missing parents), multiple roots, no root
func (p *leafSpanPruningProcessor) buildTraceTree(spans []spanInfo) *traceTree {
	tree := &traceTree{
		nodeByID: make(map[pcommon.SpanID]*spanNode, len(spans)),
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
	// Pre-allocate orphans slice with reasonable capacity
	tree.orphans = make([]*spanNode, 0, len(spans)/10)
	var rootCount int

	for _, node := range tree.nodeByID {
		parentID := node.info.span.ParentSpanID()
		if parentID.IsEmpty() {
			// This is a root span (no parent)
			rootCount++
		} else if parent, exists := tree.nodeByID[parentID]; exists {
			// Link to parent
			node.parent = parent
			if parent.children == nil {
				parent.children = make([]*spanNode, 0, 4) // pre-allocate with reasonable capacity
			}
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
	leaves := make([]*spanNode, 0, len(t.nodeByID)/4) // pre-allocate assuming ~25% are leaves
	for _, node := range t.nodeByID {
		if len(node.children) == 0 {
			leaves = append(leaves, node)
		}
	}
	return leaves
}

// findEligibleParentNodes finds parent nodes whose ALL children are marked for removal
func (p *leafSpanPruningProcessor) findEligibleParentNodes(tree *traceTree, nodesToRemove map[pcommon.SpanID]struct{}) []*spanNode {
	eligibleParents := make([]*spanNode, 0, len(tree.nodeByID)/10)

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
