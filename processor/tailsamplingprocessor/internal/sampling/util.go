// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// hasResourceOrSpanWithCondition iterates through all the resources and instrumentation library spans until any
// callback returns true.
func hasResourceOrSpanWithCondition(
	td ptrace.Traces,
	shouldSampleResource func(resource pcommon.Resource) bool,
	shouldSampleSpan func(span ptrace.Span) bool,
) Decision {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)

		resource := rs.Resource()
		if shouldSampleResource(resource) {
			return NewDecisionWithThreshold(sampling.AlwaysSampleThreshold)
		}

		if hasInstrumentationLibrarySpanWithCondition(rs.ScopeSpans(), shouldSampleSpan, false) {
			return NewDecisionWithThreshold(sampling.AlwaysSampleThreshold)
		}
	}
	return NewDecisionWithThreshold(sampling.NeverSampleThreshold)
}

// invertHasResourceOrSpanWithCondition mathematically inverts the result of hasResourceOrSpanWithCondition.
// This implements proper OTEP 250 threshold inversion instead of boolean-style logic.
func invertHasResourceOrSpanWithCondition(
	td ptrace.Traces,
	shouldSampleResource func(resource pcommon.Resource) bool,
	shouldSampleSpan func(span ptrace.Span) bool,
) Decision {
	// First get the normal (non-inverted) decision
	normalDecision := hasResourceOrSpanWithCondition(td, shouldSampleResource, shouldSampleSpan)

	// Apply mathematical inversion using OTEP 250 semantics
	return NewInvertedDecision(normalDecision.Threshold)
}

// hasSpanWithCondition iterates through all the instrumentation library spans until any callback returns true.
func hasSpanWithCondition(td ptrace.Traces, shouldSample func(span ptrace.Span) bool) Decision {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)

		if hasInstrumentationLibrarySpanWithCondition(rs.ScopeSpans(), shouldSample, false) {
			return NewDecisionWithThreshold(sampling.AlwaysSampleThreshold)
		}
	}
	return NewDecisionWithThreshold(sampling.NeverSampleThreshold)
}

func hasInstrumentationLibrarySpanWithCondition(ilss ptrace.ScopeSpansSlice, check func(span ptrace.Span) bool, invert bool) bool {
	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)

		for j := 0; j < ils.Spans().Len(); j++ {
			span := ils.Spans().At(j)

			if r := check(span); r != invert {
				return r
			}
		}
	}
	return invert
}

func SetAttrOnScopeSpans(data *TraceData, attrName string, attrKey string) {
	data.Lock()
	defer data.Unlock()

	rs := data.ReceivedBatches.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		rss := rs.At(i)
		for j := 0; j < rss.ScopeSpans().Len(); j++ {
			ss := rss.ScopeSpans().At(j)
			ss.Scope().Attributes().PutStr(attrName, attrKey)
		}
	}
}
