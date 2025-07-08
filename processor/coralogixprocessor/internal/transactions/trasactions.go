// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transactions // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/transactions"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const (
	TransactionIdentifier     = "cgx.transaction"
	TransactionIdentifierRoot = "cgx.transaction.root"
)

func ApplyTransactionsAttributes(td ptrace.Traces, logger *zap.Logger) (ptrace.Traces, error) {
	if td.SpanCount() == 0 {
		logger.Debug("no spans found in the trace")
		return td, nil
	}

	spansByTraceID := groupSpansByTraceID(td)

	for _, spans := range spansByTraceID {
		logger.Debug("processing trace", zap.String("traceID", spans[0].TraceID().String()), zap.Int("spans", len(spans)))
		root := buildSpanTree(spans, logger)
		if root != nil {
			markSpanAsRoot(root.span)
			applyTransactionToTrace(root, root.span.Name())
		}
	}

	return td, nil
}

func groupSpansByTraceID(td ptrace.Traces) map[pcommon.TraceID][]ptrace.Span {
	traceSpanMap := make(map[pcommon.TraceID][]ptrace.Span)
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			scopeSpans := rs.ScopeSpans().At(j)
			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				span := scopeSpans.Spans().At(k)
				traceID := span.TraceID()
				traceSpanMap[traceID] = append(traceSpanMap[traceID], span)
			}
		}
	}
	return traceSpanMap
}

func applyTransactionToTrace(currentSpan *spanNode, transactionName string) {
	for _, child := range currentSpan.children {
		if _, ok := child.span.Attributes().Get(TransactionIdentifierRoot); ok {
			applyTransactionToTrace(child, child.span.Name())
		} else if child.span.Kind() == ptrace.SpanKindServer || child.span.Kind() == ptrace.SpanKindConsumer {
			markSpanAsRoot(child.span)
			applyTransactionToTrace(child, child.span.Name())
		} else {
			child.span.Attributes().PutStr(TransactionIdentifier, transactionName)
			applyTransactionToTrace(child, transactionName)
		}
	}
}

func markSpanAsRoot(span ptrace.Span) {
	transactionName := span.Name()
	span.Attributes().PutStr(TransactionIdentifier, transactionName)
	span.Attributes().PutBool(TransactionIdentifierRoot, true)
}
