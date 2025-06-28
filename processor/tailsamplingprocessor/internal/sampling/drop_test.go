// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestDropEvaluatorNotSampled(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "name", []string{"value"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewDrop(zap.NewNop(), []PolicyEvaluator{n1, n2})

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
	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)
	assert.Equal(t, NotSampled, decision)
}

func TestDropEvaluatorSampled(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewDrop(zap.NewNop(), []PolicyEvaluator{n1, n2})

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
	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)
	assert.Equal(t, Dropped, decision)
}

func TestDropEvaluatorStringInvertMatch(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"no_match"}, false, 0, true)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewDrop(zap.NewNop(), []PolicyEvaluator{n1, n2})

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
	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)
	assert.Equal(t, Dropped, decision)
}

func TestDropEvaluatorStringInvertNotMatch(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, true)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewDrop(zap.NewNop(), []PolicyEvaluator{n1, n2})

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
	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)
	assert.Equal(t, NotSampled, decision)
}
