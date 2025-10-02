// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestSkipEvaluatorContinued(t *testing.T) {
	// Test case where no span matches all sub-policies - should return Continued
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "service", []string{"important-service"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip([]samplingpolicy.Evaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	// First span - has service but not error status
	span1 := ils.Spans().AppendEmpty()
	span1.Attributes().PutStr("service", "important-service")
	span1.Status().SetCode(ptrace.StatusCodeOk)
	span1.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span1.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	// Second span - has error status but not the right service
	span2 := ils.Spans().AppendEmpty()
	span2.Attributes().PutStr("service", "other-service")
	span2.Status().SetCode(ptrace.StatusCodeError)
	span2.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span2.SetSpanID([8]byte{2, 3, 4, 5, 6, 7, 8, 9})

	trace := &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, samplingpolicy.Continued, decision)
}

func TestSkipEvaluatorSkipped(t *testing.T) {
	// Test case where one span matches all sub-policies - should returnsamplingpolicy.Skipped
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "service", []string{"important-service"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip([]samplingpolicy.Evaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	// First span - has service but not error status
	span1 := ils.Spans().AppendEmpty()
	span1.Attributes().PutStr("service", "important-service")
	span1.Status().SetCode(ptrace.StatusCodeOk)
	span1.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span1.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	// Second span - matches both conditions (service AND error status)
	span2 := ils.Spans().AppendEmpty()
	span2.Attributes().PutStr("service", "important-service")
	span2.Status().SetCode(ptrace.StatusCodeError)
	span2.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span2.SetSpanID([8]byte{2, 3, 4, 5, 6, 7, 8, 9})

	trace := &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, samplingpolicy.Skipped, decision)
}

func TestSkipEvaluatorStringInvertMatch(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"no_match"}, false, 0, true)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip([]samplingpolicy.Evaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value")
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, samplingpolicy.Skipped, decision)
}

func TestSkipEvaluatorStringInvertNotMatch(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, true)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip([]samplingpolicy.Evaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value")
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, samplingpolicy.Continued, decision)
}

func TestSkipEvaluatorOneSubPolicyNotSampled(t *testing.T) {
	// Test case where first policy returns NotSampled and second returns Sampled
	// Should return Continued because any NotSampled/InvertNotSampled causes Continued
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "missing_attribute", []string{"value"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip([]samplingpolicy.Evaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, samplingpolicy.Continued, decision)
}

func TestSkipEvaluatorEmptySubPolicies(t *testing.T) {
	// Test case with no sub-policies - should return Continued (no conditions to match)
	skip := NewSkip([]samplingpolicy.Evaluator{})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, samplingpolicy.Continued, decision)
}

func TestSkipEvaluatorSpanLevelEvaluation(t *testing.T) {
	// Test case demonstrating span-level evaluation behavior
	// This is the key test that shows how the skip policy works at span level
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "service.name", []string{"critical-service"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	skip := NewSkip([]samplingpolicy.Evaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	// Span 1: service.name="web-frontend", status=OK (doesn't match all conditions)
	span1 := ils.Spans().AppendEmpty()
	span1.Attributes().PutStr("service.name", "web-frontend")
	span1.Status().SetCode(ptrace.StatusCodeOk)
	span1.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span1.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	// Span 2: service.name="critical-service", status=ERROR (matches all conditions!)
	span2 := ils.Spans().AppendEmpty()
	span2.Attributes().PutStr("service.name", "critical-service")
	span2.Status().SetCode(ptrace.StatusCodeError)
	span2.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span2.SetSpanID([8]byte{2, 3, 4, 5, 6, 7, 8, 9})

	// Span 3: service.name="database", status=OK (doesn't match all conditions)
	span3 := ils.Spans().AppendEmpty()
	span3.Attributes().PutStr("service.name", "database")
	span3.Status().SetCode(ptrace.StatusCodeOk)
	span3.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span3.SetSpanID([8]byte{3, 4, 5, 6, 7, 8, 9, 10})

	trace := &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	// Should returnsamplingpolicy.Skipped because span2 matches all conditions (service.name AND status)
	assert.Equal(t, samplingpolicy.Skipped, decision)
}

func TestSkipEvaluatorAllSubPoliciesSampled(t *testing.T) {
	// Test case where all sub-policies return Sampled - should returnsamplingpolicy.Skipped
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, false)
	n2 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "another_attribute", []string{"another_value"}, false, 0, false)

	skip := NewSkip([]samplingpolicy.Evaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value")
	span.Attributes().PutStr("another_attribute", "another_value")
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := skip.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate skip policy: %v", err)
	assert.Equal(t, samplingpolicy.Skipped, decision)
}
