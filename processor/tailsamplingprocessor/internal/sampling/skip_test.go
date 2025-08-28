// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestSkipEvaluatorContinued(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "name", []string{"value"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, Continued, decision)
}

func TestSkipEvaluatorSkipped(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value")
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, Skipped, decision)
}

func TestSkipEvaluatorStringInvertMatch(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"no_match"}, false, 0, true)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value")
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, Skipped, decision)
}

func TestSkipEvaluatorStringInvertNotMatch(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, true)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value")
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, Continued, decision)
}

func TestSkipEvaluatorOneSubPolicyNotSampled(t *testing.T) {
	// Test case where first policy returns NotSampled and second returns Sampled
	// Should return Continued because any NotSampled/InvertNotSampled causes Continued
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "missing_attribute", []string{"value"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, Continued, decision)
}

func TestSkipEvaluatorEmptySubPolicies(t *testing.T) {
	// Test case with no sub-policies - should return Skipped
	skip := NewSkip(zap.NewNop(), []PolicyEvaluator{})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, Skipped, decision)
}

func TestSkipEvaluatorAllSubPoliciesSampled(t *testing.T) {
	// Test case where all sub-policies return Sampled - should return Skipped
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, false)
	n2 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "another_attribute", []string{"another_value"}, false, 0, false)

	skip := NewSkip(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value")
	span.Attributes().PutStr("another_attribute", "another_value")
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, Skipped, decision)
}
