// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leafspanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor"

import (
	"sort"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// builderPool reduces allocations in hot path by reusing string builders
var builderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

// buildGroupKey creates a grouping key from span name, status, and matching attributes
// Attributes are matched using glob patterns from the configuration
// Uses a pooled string builder to reduce allocations in hot path
func (p *leafSpanPruningProcessor) buildGroupKey(span ptrace.Span) string {
	builder := builderPool.Get().(*strings.Builder)
	builder.Reset()
	defer builderPool.Put(builder)

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

// buildParentGroupKey creates a grouping key for parent spans using only name and status
// (attributes are not considered for parent aggregation)
func (p *leafSpanPruningProcessor) buildParentGroupKey(span ptrace.Span) string {
	builder := builderPool.Get().(*strings.Builder)
	builder.Reset()
	defer builderPool.Put(builder)

	builder.WriteString(span.Name())
	builder.WriteString("|status=")
	builder.WriteString(span.Status().Code().String())
	return builder.String()
}

// buildLeafGroupKey creates a grouping key for leaf spans using tree node parent pointer
// Renamed from buildLeafGroupKeyFromNode for clarity
func (p *leafSpanPruningProcessor) buildLeafGroupKey(node *spanNode) string {
	// Use cached group key if available
	if node.groupKey != "" {
		return node.groupKey
	}

	builder := builderPool.Get().(*strings.Builder)
	builder.Reset()
	defer builderPool.Put(builder)

	// Include parent span name to separate groups by parent
	if node.parent != nil {
		builder.WriteString("parent=")
		builder.WriteString(node.parent.span.Name())
		builder.WriteString("|")
	}

	// Include regular group key (name + status + attributes)
	builder.WriteString(p.buildGroupKey(node.span))

	// Cache the key for future use
	node.groupKey = builder.String()
	return node.groupKey
}

// groupLeafNodesByKey groups leaf nodes by their grouping key
func (p *leafSpanPruningProcessor) groupLeafNodesByKey(leafNodes []*spanNode) map[string][]*spanNode {
	// Pre-size map based on expected number of groups (assume ~1/4 unique groups)
	groups := make(map[string][]*spanNode, len(leafNodes)/4+1)
	for _, node := range leafNodes {
		key := p.buildLeafGroupKey(node)
		groups[key] = append(groups[key], node)
	}
	return groups
}
