// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestAndEvaluatorNotSampled(t *testing.T) {
	n1, err := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "name", []string{"value"}, false, 0, false)
	require.NoError(t, err)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewAnd(zap.NewNop(), []samplingpolicy.Evaluator{n1, n2})

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

func TestAndEvaluatorSampled(t *testing.T) {
	n1, err := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, false)
	require.NoError(t, err)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewAnd(zap.NewNop(), []samplingpolicy.Evaluator{n1, n2})

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
	assert.Equal(t, samplingpolicy.Sampled, decision)
}

func TestAndEvaluatorStringInvertSampled(t *testing.T) {
	n1, err := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"no_match"}, false, 0, true)
	require.NoError(t, err)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewAnd(zap.NewNop(), []samplingpolicy.Evaluator{n1, n2})

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
	assert.Equal(t, samplingpolicy.Sampled, decision)
}

func TestAndEvaluatorStringInvertNotSampled(t *testing.T) {
	n1, err := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, true)
	require.NoError(t, err)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewAnd(zap.NewNop(), []samplingpolicy.Evaluator{n1, n2})

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

func TestAndIsStatefulIfAnySubpolicyIsStateful(t *testing.T) {
	stateless := NewAlwaysSample(componenttest.NewNopTelemetrySettings())
	stateful := NewRateLimiting(componenttest.NewNopTelemetrySettings(), 10)

	and := NewAnd(zap.NewNop(), []samplingpolicy.Evaluator{stateless, stateful})
	assert.True(t, and.IsStateful())
}

func TestAndBatchRateLimitingWithFilter(t *testing.T) {
	enableTracestateFeatureGate(t)

	// d fails the filter but has the highest randomness. If it wrongly
	// competed for the budget it would be kept and push out b; asserting b is
	// kept proves filtered-out traces do not spend budget.
	d := andSvcTrace(4, "other", 200)
	a := andSvcTrace(1, "keep", 150)
	b := andSvcTrace(2, "keep", 100)
	c := andSvcTrace(3, "keep", 50)
	batch := []*samplingpolicy.TraceData{d, a, b, c}

	want := map[pcommon.TraceID]samplingpolicy.Decision{
		traceIDOf(a): samplingpolicy.Sampled,    // matches filter, within budget of 2
		traceIDOf(b): samplingpolicy.Sampled,    // matches filter, within budget of 2
		traceIDOf(c): samplingpolicy.NotSampled, // matches filter, over budget
		traceIDOf(d): samplingpolicy.NotSampled, // fails filter, no budget spent
	}

	// Run with the filter first and with the rate limiter first to confirm
	// the outcome does not depend on sub-policy order.
	for _, order := range []string{"filter-first", "ratelimiter-first"} {
		t.Run(order, func(t *testing.T) {
			settings := componenttest.NewNopTelemetrySettings()
			filter, err := NewStringAttributeFilter(settings, "svc", []string{"keep"}, false, 0, false)
			require.NoError(t, err)
			rl := NewRateLimitingWithBurstCapacity(settings, 2, 2)

			subs := []samplingpolicy.Evaluator{filter, rl}
			if order == "ratelimiter-first" {
				subs = []samplingpolicy.Evaluator{rl, filter}
			}
			and := NewAnd(zap.NewNop(), subs)
			ba, ok := and.(*batchAnd)
			require.True(t, ok, "and containing a rate_limiting sub-policy should be batch-aware")

			ba.CalculateThreshold(t.Context(), batch)

			te := and.(samplingpolicy.ThresholdEvaluator)
			for _, td := range batch {
				decision, _, err := te.EvaluateWithThreshold(t.Context(), traceIDOf(td), td)
				require.NoError(t, err)
				assert.Equalf(t, want[traceIDOf(td)], decision, "trace %x", traceIDOf(td))
			}
		})
	}
}

// andSvcTrace builds single-span trace data carrying a "svc" attribute,
// with a trace ID that encodes the given randomness in its low bytes and a
// unique tag in its first byte.
func andSvcTrace(tag byte, svc string, randomness uint64) *samplingpolicy.TraceData {
	var id [16]byte
	id[0] = tag
	binary.BigEndian.PutUint64(id[8:], randomness)
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID(id))
	span.Attributes().PutStr("svc", svc)
	return &samplingpolicy.TraceData{SpanCount: 1, ReceivedBatches: traces}
}
