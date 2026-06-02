// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/criticalpath"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/transactions"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	assert.False(t, cfg.TransactionsConfig.Enabled)
	assert.False(t, cfg.CriticalPathConfig.Enabled)
}

func TestProcessTraces_CriticalPathDisabled(t *testing.T) {
	processor := &coralogixProcessor{
		config: &Config{},
		logger: zap.NewNop(),
	}
	traces := newProcessorTestTrace()

	_, err := processor.processTraces(t.Context(), traces)
	require.NoError(t, err)

	root := processorSpanByName(traces, "root")
	_, ok := root.Attributes().Get(criticalpath.AttributeCriticalPath)
	assert.False(t, ok)
}

func TestProcessTraces_CriticalPathEnabled(t *testing.T) {
	processor := &coralogixProcessor{
		config: &Config{
			CriticalPathConfig: CriticalPathConfig{Enabled: true},
		},
		logger: zap.NewNop(),
	}
	traces := newProcessorTestTrace()
	child := processorSpanByName(traces, "child")
	child.Attributes().PutBool(criticalpath.AttributeCriticalPath, true)
	child.Attributes().PutInt(criticalpath.AttributeCriticalPathExclusiveNS, 1)
	child.Attributes().PutInt(criticalpath.AttributeCriticalPathInclusiveNS, 1)

	_, err := processor.processTraces(t.Context(), traces)
	require.NoError(t, err)

	root := processorSpanByName(traces, "root")
	assertCriticalAttrsOnProcessorSpan(t, root, 60, 100)
	assertCriticalAttrsOnProcessorSpan(t, child, 40, 40)
}

func TestProcessTraces_EmptyTraces(t *testing.T) {
	processor := &coralogixProcessor{
		config: &Config{
			TransactionsConfig: TransactionsConfig{Enabled: true},
			CriticalPathConfig: CriticalPathConfig{Enabled: true},
		},
		logger: zap.NewNop(),
	}

	result, err := processor.processTraces(t.Context(), ptrace.NewTraces())
	require.NoError(t, err)
	assert.Equal(t, 0, result.SpanCount())
}

func TestProcessTraces_TransactionsEnabled(t *testing.T) {
	processor := &coralogixProcessor{
		config: &Config{
			TransactionsConfig: TransactionsConfig{Enabled: true},
		},
		logger: zap.NewNop(),
	}
	traces := newProcessorTestTrace()

	_, err := processor.processTraces(t.Context(), traces)
	require.NoError(t, err)

	root := processorSpanByName(traces, "root")
	transactionName, ok := root.Attributes().Get(transactions.TransactionIdentifier)
	require.True(t, ok)
	assert.Equal(t, "root", transactionName.Str())
}

func TestProcessTraces_TransactionsAndCriticalPathEnabled(t *testing.T) {
	processor := &coralogixProcessor{
		config: &Config{
			TransactionsConfig: TransactionsConfig{Enabled: true},
			CriticalPathConfig: CriticalPathConfig{Enabled: true},
		},
		logger: zap.NewNop(),
	}
	traces := newProcessorTestTrace()

	_, err := processor.processTraces(t.Context(), traces)
	require.NoError(t, err)

	root := processorSpanByName(traces, "root")
	child := processorSpanByName(traces, "child")

	transactionName, ok := root.Attributes().Get(transactions.TransactionIdentifier)
	require.True(t, ok)
	assert.Equal(t, "root", transactionName.Str())

	transactionRoot, ok := root.Attributes().Get(transactions.TransactionIdentifierRoot)
	require.True(t, ok)
	assert.True(t, transactionRoot.Bool())

	assertCriticalAttrsOnProcessorSpan(t, root, 60, 100)
	assertCriticalAttrsOnProcessorSpan(t, child, 40, 40)
}

func TestProcessTraces_BothFeaturesShareGroupingPass(t *testing.T) {
	processor := &coralogixProcessor{
		config: &Config{
			TransactionsConfig: TransactionsConfig{Enabled: true},
			CriticalPathConfig: CriticalPathConfig{Enabled: true},
		},
		logger: zap.NewNop(),
	}
	traces := newProcessorTestTrace()

	_, err := processor.processTraces(t.Context(), traces)
	require.NoError(t, err)

	root := processorSpanByName(traces, "root")
	transactionName, ok := root.Attributes().Get(transactions.TransactionIdentifier)
	require.True(t, ok)
	assert.Equal(t, "root", transactionName.Str())

	critical, ok := root.Attributes().Get(criticalpath.AttributeCriticalPath)
	require.True(t, ok)
	assert.True(t, critical.Bool())
}

func newProcessorTestTrace() ptrace.Traces {
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9})

	root := spans.AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(pcommon.SpanID([8]byte{1}))
	root.SetName("root")
	root.SetStartTimestamp(pcommon.Timestamp(0))
	root.SetEndTimestamp(pcommon.Timestamp(100))
	root.SetKind(ptrace.SpanKindServer)

	child := spans.AppendEmpty()
	child.SetTraceID(traceID)
	child.SetSpanID(pcommon.SpanID([8]byte{2}))
	child.SetParentSpanID(pcommon.SpanID([8]byte{1}))
	child.SetName("child")
	child.SetStartTimestamp(pcommon.Timestamp(20))
	child.SetEndTimestamp(pcommon.Timestamp(60))

	return traces
}

func processorSpanByName(traces ptrace.Traces, name string) ptrace.Span {
	spans := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	for i := 0; i < spans.Len(); i++ {
		span := spans.At(i)
		if span.Name() == name {
			return span
		}
	}

	panic("span not found: " + name)
}

func assertCriticalAttrsOnProcessorSpan(t *testing.T, span ptrace.Span, exclusiveNS, inclusiveNS int64) {
	t.Helper()

	critical, ok := span.Attributes().Get(criticalpath.AttributeCriticalPath)
	require.True(t, ok)
	assert.True(t, critical.Bool())

	exclusive, ok := span.Attributes().Get(criticalpath.AttributeCriticalPathExclusiveNS)
	require.True(t, ok)
	assert.Equal(t, exclusiveNS, exclusive.Int())

	inclusive, ok := span.Attributes().Get(criticalpath.AttributeCriticalPathInclusiveNS)
	require.True(t, ok)
	assert.Equal(t, inclusiveNS, inclusive.Int())
}
