// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transactions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestApplyTransactionsAttributes_EmptyTrace(t *testing.T) {
	logger := zap.NewNop()
	traces := ptrace.NewTraces()

	result, err := ApplyTransactionsAttributes(traces, logger)
	assert.NoError(t, err)
	assert.Equal(t, int(0), result.SpanCount())
}

func TestApplyTransactionsAttributes_SingleSpan(t *testing.T) {
	logger := zap.NewNop()
	traces := createTestTraces(1, ptrace.SpanKindServer)

	result, err := ApplyTransactionsAttributes(traces, logger)
	assert.NoError(t, err)

	// Get the first span
	rspan := result.ResourceSpans().At(0)
	sspan := rspan.ScopeSpans().At(0)
	span := sspan.Spans().At(0)

	// Check if root attributes were set correctly
	val, ok := span.Attributes().Get(TransactionIdentifier)
	assert.True(t, ok)
	assert.Equal(t, "test-span-0", val.Str())

	val, ok = span.Attributes().Get(TransactionIdentifierRoot)
	assert.True(t, ok)
	assert.True(t, val.Bool())
}

func TestApplyTransactionsAttributes_MultipleSpans(t *testing.T) {
	logger := zap.NewNop()
	traces := createTestTraces(3, ptrace.SpanKindClient)

	// Set the first span as Server kind
	rspan := traces.ResourceSpans().At(0)
	sspan := rspan.ScopeSpans().At(0)
	span := sspan.Spans().At(0)
	span.SetKind(ptrace.SpanKindServer)

	result, err := ApplyTransactionsAttributes(traces, logger)
	assert.NoError(t, err)

	// Check first span (server)
	rspan = result.ResourceSpans().At(0)
	sspan = rspan.ScopeSpans().At(0)
	span = sspan.Spans().At(0)

	val, ok := span.Attributes().Get(TransactionIdentifier)
	assert.True(t, ok)
	assert.Equal(t, "test-span-0", val.Str())

	val, ok = span.Attributes().Get(TransactionIdentifierRoot)
	assert.True(t, ok)
	assert.True(t, val.Bool())

	// Check other spans (clients)
	for i := 1; i < 3; i++ {
		span := sspan.Spans().At(i)
		val, ok := span.Attributes().Get(TransactionIdentifier)
		assert.True(t, ok)
		assert.Equal(t, "test-span-0", val.Str())

		_, ok = span.Attributes().Get(TransactionIdentifierRoot)
		assert.False(t, ok)
	}
}

func TestApplyTransactionsAttributes_ConsumerSpan(t *testing.T) {
	logger := zap.NewNop()
	traces := createTestTraces(3, ptrace.SpanKindClient)

	// Set the first span as Consumer kind
	rspan := traces.ResourceSpans().At(0)
	sspan := rspan.ScopeSpans().At(0)
	span := sspan.Spans().At(0)
	span.SetKind(ptrace.SpanKindConsumer)

	result, err := ApplyTransactionsAttributes(traces, logger)
	assert.NoError(t, err)

	// Check first span (consumer)
	rspan = result.ResourceSpans().At(0)
	sspan = rspan.ScopeSpans().At(0)
	span = sspan.Spans().At(0)

	val, ok := span.Attributes().Get(TransactionIdentifier)
	assert.True(t, ok)
	assert.Equal(t, "test-span-0", val.Str())

	val, ok = span.Attributes().Get(TransactionIdentifierRoot)
	assert.True(t, ok)
	assert.True(t, val.Bool())

	// Check other spans (clients)
	for i := 1; i < 3; i++ {
		span := sspan.Spans().At(i)
		val, ok := span.Attributes().Get(TransactionIdentifier)
		assert.True(t, ok)
		assert.Equal(t, "test-span-0", val.Str())

		_, ok = span.Attributes().Get(TransactionIdentifierRoot)
		assert.False(t, ok)
	}
}

func TestApplyTransactionsAttributes_ServerAndConsumerSpans(t *testing.T) {
	logger := zap.NewNop()
	traces := createTestTraces(4, ptrace.SpanKindClient)

	// Set the first span as Server kind
	rspan := traces.ResourceSpans().At(0)
	sspan := rspan.ScopeSpans().At(0)
	span := sspan.Spans().At(0)
	span.SetKind(ptrace.SpanKindServer)
	span.SetName("server-span")

	// Set the second span as Consumer kind
	span = sspan.Spans().At(1)
	span.SetKind(ptrace.SpanKindConsumer)
	span.SetName("consumer-span")

	result, err := ApplyTransactionsAttributes(traces, logger)
	assert.NoError(t, err)

	// Check first span (server)
	rspan = result.ResourceSpans().At(0)
	sspan = rspan.ScopeSpans().At(0)
	span = sspan.Spans().At(0)

	val, ok := span.Attributes().Get(TransactionIdentifier)
	assert.True(t, ok)
	assert.Equal(t, "server-span", val.Str())

	val, ok = span.Attributes().Get(TransactionIdentifierRoot)
	assert.True(t, ok)
	assert.True(t, val.Bool())

	// Check second span (consumer)
	span = sspan.Spans().At(1)
	val, ok = span.Attributes().Get(TransactionIdentifier)
	assert.True(t, ok)
	assert.Equal(t, "consumer-span", val.Str())

	val, ok = span.Attributes().Get(TransactionIdentifierRoot)
	assert.True(t, ok)
	assert.True(t, val.Bool())

	// Check other spans (clients)
	for i := 2; i < 4; i++ {
		span := sspan.Spans().At(i)
		val, ok := span.Attributes().Get(TransactionIdentifier)
		assert.True(t, ok)
		assert.Equal(t, "server-span", val.Str())

		_, ok = span.Attributes().Get(TransactionIdentifierRoot)
		assert.False(t, ok)
	}
}

func TestGroupSpansByTraceID(t *testing.T) {
	traces := createTestTraces(3, ptrace.SpanKindClient)

	result := groupSpansByTraceID(traces)
	assert.Len(t, result, 1) // All spans should have the same trace ID

	for traceID, spans := range result {
		assert.Len(t, spans, 3)
		for _, span := range spans {
			assert.Equal(t, traceID, span.TraceID())
		}
	}
}

// Helper function to create test traces
func createTestTraces(numSpans int, kind ptrace.SpanKind) ptrace.Traces {
	traces := ptrace.NewTraces()

	// Create resource spans
	rspans := traces.ResourceSpans().AppendEmpty()

	// Create scope spans
	sspans := rspans.ScopeSpans().AppendEmpty()

	// Create spans
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	for i := range numSpans {
		span := sspans.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, byte(i)}))
		span.SetName("test-span-" + string(rune(i+'0')))
		span.SetKind(kind)

		if i > 0 {
			// Set parent ID for all spans except the first one
			span.SetParentSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 0}))
		}
	}

	return traces
}
