// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transactions // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/transactions"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/traceutil"
)

const (
	TransactionIdentifier     = "cgx.transaction"
	TransactionIdentifierRoot = "cgx.transaction.root"
)

func ApplyTransactionsAttributesByTraceID(spansByTraceID map[pcommon.TraceID][]ptrace.Span, logger *zap.Logger) {
	applyTransactionsAttributesByTraceID(spansByTraceID, logger)
}

func applyTransactionsAttributesByTraceID(spansByTraceID map[pcommon.TraceID][]ptrace.Span, logger *zap.Logger) {
	for traceID, spans := range spansByTraceID {
		if len(spans) == 0 {
			logger.Debug("skipping empty trace span group", zap.String("traceID", traceID.String()))
			continue
		}
		logger.Debug("processing trace", zap.String("traceID", traceID.String()), zap.Int("spans", len(spans)))
		root := selectSpanRoot(traceutil.BuildTraceTree(spans), logger)
		if root != nil {
			markSpanAsRoot(root.Span)
			applyTransactionToTrace(root, root.Span.Name())
		}
	}
}

func ApplyTransactionAttributesToTree(tree traceutil.TraceTree, logger *zap.Logger) {
	root := selectSpanRoot(tree, logger)
	if root != nil {
		markSpanAsRoot(root.Span)
		applyTransactionToTrace(root, root.Span.Name())
	}
}

func applyTransactionToTrace(currentSpan *traceutil.TraceTreeNode, transactionName string) {
	for _, child := range currentSpan.Children {
		if _, ok := child.Span.Attributes().Get(TransactionIdentifierRoot); ok {
			applyTransactionToTrace(child, child.Span.Name())
		} else if child.Span.Kind() == ptrace.SpanKindServer || child.Span.Kind() == ptrace.SpanKindConsumer {
			markSpanAsRoot(child.Span)
			applyTransactionToTrace(child, child.Span.Name())
		} else {
			child.Span.Attributes().PutStr(TransactionIdentifier, transactionName)
			applyTransactionToTrace(child, transactionName)
		}
	}
}

func markSpanAsRoot(span ptrace.Span) {
	transactionName := span.Name()
	span.Attributes().PutStr(TransactionIdentifier, transactionName)
	span.Attributes().PutBool(TransactionIdentifierRoot, true)
}
