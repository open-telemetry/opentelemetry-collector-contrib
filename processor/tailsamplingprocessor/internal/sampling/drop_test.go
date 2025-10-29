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

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestDropEvaluatorNotSampled(t *testing.T) {
	n1, err := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "name", []string{"value"}, false, 0, false)
	require.NoError(t, err)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewDrop(zap.NewNop(), []samplingpolicy.Evaluator{n1, n2})

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
	decision, err := and.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)
}

func TestDropEvaluatorSampled(t *testing.T) {
	n1, err := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, false)
	require.NoError(t, err)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewDrop(zap.NewNop(), []samplingpolicy.Evaluator{n1, n2})

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
	decision, err := and.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)
	assert.Equal(t, samplingpolicy.Dropped, decision)
}

func TestDropEvaluatorStringInvertMatch(t *testing.T) {
	n1, err := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"no_match"}, false, 0, true)
	require.NoError(t, err)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewDrop(zap.NewNop(), []samplingpolicy.Evaluator{n1, n2})

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
	decision, err := and.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)
	assert.Equal(t, samplingpolicy.Dropped, decision)
}

func TestDropEvaluatorStringInvertNotMatch(t *testing.T) {
	n1, err := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, true)
	require.NoError(t, err)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewDrop(zap.NewNop(), []samplingpolicy.Evaluator{n1, n2})

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
	decision, err := and.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)
}
