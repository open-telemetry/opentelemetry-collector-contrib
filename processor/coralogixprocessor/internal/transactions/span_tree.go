// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transactions // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/transactions"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type spanNode struct {
	span     ptrace.Span
	children []*spanNode
}

// buildSpanTree constructs a hierarchical tree of spans
func buildSpanTree(spans []ptrace.Span, logger *zap.Logger) *spanNode {
	spanMap := make(map[pcommon.SpanID]*spanNode)
	var rootSpan *spanNode
	var orphanedSpans []*spanNode

	for _, span := range spans {
		node := &spanNode{span: span}
		spanMap[span.SpanID()] = node

		if span.ParentSpanID().IsEmpty() {
			if rootSpan != nil {
				logger.Warn("Multiple root spans found in single trace",
					zap.String("existingRootSpanID", rootSpan.span.SpanID().String()),
					zap.String("newRootSpanID", span.SpanID().String()))
				// We'll keep the earliest span as root
				if span.StartTimestamp() < rootSpan.span.StartTimestamp() {
					orphanedSpans = append(orphanedSpans, rootSpan)
					rootSpan = node
				} else {
					orphanedSpans = append(orphanedSpans, node)
				}
			} else {
				rootSpan = node
			}
		}
	}

	if len(orphanedSpans) > 0 {
		logger.Warn("orphaned spans found", zap.Int("orphanedSpans", len(orphanedSpans)))
	}

	// If no root span was found, use the earliest span as root
	if rootSpan == nil && len(spans) > 0 {
		earliestSpan := spanMap[spans[0].SpanID()]
		earliestTime := spans[0].StartTimestamp()

		for _, node := range spanMap {
			if node.span.StartTimestamp() < earliestTime {
				earliestTime = node.span.StartTimestamp()
				earliestSpan = node
			}
		}

		rootSpan = earliestSpan
		logger.Debug("No root span found in trace, using earliest span as root",
			zap.String("selectedRootSpanID", rootSpan.span.SpanID().String()))
	}

	for _, node := range spanMap {
		if node == rootSpan {
			continue
		}

		parentID := node.span.ParentSpanID()
		if parent, exists := spanMap[parentID]; exists {
			parent.children = append(parent.children, node)
		}
	}

	return rootSpan
}
