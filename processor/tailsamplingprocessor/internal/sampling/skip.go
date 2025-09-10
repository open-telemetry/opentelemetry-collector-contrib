// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type Skip struct {
	// the subpolicy evaluators
	subpolicies []PolicyEvaluator
}

func NewSkip(
	subpolicies []PolicyEvaluator,
) PolicyEvaluator {
	return &Skip{
		subpolicies: subpolicies,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
// Evaluates each span individually with the sub-policies.
// If ANY span matches ALL sub-policies (all return Sampled/InvertSampled), it returns Skipped.
// If NO span matches all sub-policies, it returns Continued.
func (c *Skip) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	// If no sub-policies are defined, return Continued (no conditions to match)
	if len(c.subpolicies) == 0 {
		return Continued, nil
	}

	trace.Lock()
	defer trace.Unlock()
	batches := trace.ReceivedBatches

	// Create a reusable single-span trace structure for efficiency
	singleSpanTrace := ptrace.NewTraces()
	spanTraceData := &TraceData{
		ReceivedBatches: singleSpanTrace,
	}

	// Iterate through all spans in the trace
	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)
		resource := rs.Resource()

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			scope := ss.Scope()

			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)

				// Check if this span matches all sub-policies using reusable structures
				spanMatches, err := c.evaluateSpanAgainstSubPolicies(ctx, traceID, span, scope, resource, singleSpanTrace, spanTraceData)
				if err != nil {
					return Unspecified, err
				}

				// If any span matches all sub-policies, return Skipped
				if spanMatches {
					return Skipped, nil
				}
			}
		}
	}

	// No span matched all sub-policies, return Continued
	return Continued, nil
}

// evaluateSpanAgainstSubPolicies efficiently evaluates a single span using reusable trace structures
func (c *Skip) evaluateSpanAgainstSubPolicies(
	ctx context.Context,
	traceID pcommon.TraceID,
	span ptrace.Span,
	scope pcommon.InstrumentationScope,
	resource pcommon.Resource,
	singleSpanTrace ptrace.Traces,
	spanTraceData *TraceData,
) (bool, error) {
	// Clear and reuse the trace structure for efficiency
	singleSpanTrace.ResourceSpans().RemoveIf(func(ptrace.ResourceSpans) bool { return true })

	// Rebuild the single-span trace with current span data
	singleRS := singleSpanTrace.ResourceSpans().AppendEmpty()
	resource.CopyTo(singleRS.Resource())

	singleSS := singleRS.ScopeSpans().AppendEmpty()
	scope.CopyTo(singleSS.Scope())

	singleSpan := singleSS.Spans().AppendEmpty()
	span.CopyTo(singleSpan)

	// Evaluate all sub-policies against this single span
	for _, sub := range c.subpolicies {
		decision, err := sub.Evaluate(ctx, traceID, spanTraceData)
		if err != nil {
			return false, err
		}
		// If any sub-policy returns NotSampled or InvertNotSampled, this span doesn't match
		if decision == NotSampled || decision == InvertNotSampled {
			return false, nil
		}
	}

	// All sub-policies returned Sampled or InvertSampled for this span
	return true, nil
}
