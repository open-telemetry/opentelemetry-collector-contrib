// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transactions // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/transactions"

import (
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/traceutil"
)

// buildSpanTree selects the transaction root from a shared trace tree.
func buildSpanTree(tree traceutil.TraceTree, logger *zap.Logger) *traceutil.TraceTreeNode {
	if len(tree.Roots) == 0 {
		return nil
	}

	rootSpan, explicitRootFound := selectTransactionRoot(tree.Roots)
	if len(tree.Roots) > 1 {
		for _, root := range tree.Roots[1:] {
			logger.Warn("Multiple root spans found in single trace",
				zap.String("existingRootSpanID", rootSpan.Span.SpanID().String()),
				zap.String("newRootSpanID", root.Span.SpanID().String()))
		}
		logger.Warn("orphaned spans found", zap.Int("orphanedSpans", len(tree.Roots)-1))
	}
	if !explicitRootFound {
		logger.Debug("No explicit root span found in trace, using earliest orphaned span as root",
			zap.String("selectedRootSpanID", rootSpan.Span.SpanID().String()))
	}

	return rootSpan
}

func selectTransactionRoot(roots []*traceutil.TraceTreeNode) (*traceutil.TraceTreeNode, bool) {
	var selectedExplicit *traceutil.TraceTreeNode
	var selectedFallback *traceutil.TraceTreeNode

	for _, root := range roots {
		if selectedFallback == nil || isBetterTransactionRoot(root, selectedFallback) {
			selectedFallback = root
		}
		if root.Span.ParentSpanID().IsEmpty() && (selectedExplicit == nil || isBetterTransactionRoot(root, selectedExplicit)) {
			selectedExplicit = root
		}
	}

	if selectedExplicit != nil {
		return selectedExplicit, true
	}

	return selectedFallback, false
}

func isBetterTransactionRoot(candidate, current *traceutil.TraceTreeNode) bool {
	if candidate.Span.StartTimestamp() != current.Span.StartTimestamp() {
		return candidate.Span.StartTimestamp() < current.Span.StartTimestamp()
	}

	return candidate.Span.SpanID().String() < current.Span.SpanID().String()
}
