// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

// hasResourceOrSpanWithCondition iterates through all the resources and instrumentation library spans until any
// callback returns true.
func hasResourceOrSpanWithCondition(
	td ptrace.Traces,
	shouldSampleResource func(resource pcommon.Resource) bool,
	shouldSampleSpan func(span ptrace.Span) bool,
) samplingpolicy.Decision {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)

		resource := rs.Resource()
		if shouldSampleResource(resource) {
			return samplingpolicy.Sampled
		}

		if hasInstrumentationLibrarySpanWithCondition(rs.ScopeSpans(), shouldSampleSpan, false) {
			return samplingpolicy.Sampled
		}
	}
	return samplingpolicy.NotSampled
}

// invertHasResourceOrSpanWithCondition iterates through all the resources and instrumentation library spans until any
// callback returns false.
func invertHasResourceOrSpanWithCondition(
	td ptrace.Traces,
	shouldSampleResource func(resource pcommon.Resource) bool,
	shouldSampleSpan func(span ptrace.Span) bool,
) samplingpolicy.Decision {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)

		resource := rs.Resource()
		if !shouldSampleResource(resource) {
			return samplingpolicy.NotSampled
		}

		if !hasInstrumentationLibrarySpanWithCondition(rs.ScopeSpans(), shouldSampleSpan, true) {
			return samplingpolicy.NotSampled
		}
	}

	return samplingpolicy.Sampled
}

// hasSpanWithCondition iterates through all the instrumentation library spans until any callback returns true.
func hasSpanWithCondition(td ptrace.Traces, shouldSample func(span ptrace.Span) bool) samplingpolicy.Decision {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)

		if hasInstrumentationLibrarySpanWithCondition(rs.ScopeSpans(), shouldSample, false) {
			return samplingpolicy.Sampled
		}
	}
	return samplingpolicy.NotSampled
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

func SetAttrOnScopeSpans(data ptrace.Traces, attrName, attrKey string) {
	rs := data.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		rss := rs.At(i)
		for j := 0; j < rss.ScopeSpans().Len(); j++ {
			ss := rss.ScopeSpans().At(j)
			ss.Scope().Attributes().PutStr(attrName, attrKey)
		}
	}
}

func SetBoolAttrOnScopeSpans(data ptrace.Traces, attrName string, attrValue bool) {
	rs := data.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		rss := rs.At(i)
		for j := 0; j < rss.ScopeSpans().Len(); j++ {
			ss := rss.ScopeSpans().At(j)
			ss.Scope().Attributes().PutBool(attrName, attrValue)
		}
	}
}

// WriteEffectiveThreshold rewrites the OpenTelemetry tracestate `th`
// field on every span in td to encode the given effective sampling
// threshold. Spans whose existing th is already stricter are left
// unchanged (UpdateTValueWithSampling refuses to lower probability).
// Spans with unparseable tracestate are counted via
// unparseableTracestate and skipped.
func WriteEffectiveThreshold(ctx context.Context, td ptrace.Traces, th sampling.Threshold, unparseableTracestate metric.Int64Counter) {
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				ts, err := sampling.NewW3CTraceState(span.TraceState().AsRaw())
				if err != nil {
					unparseableTracestate.Add(ctx, 1)
					continue
				}
				if err := ts.OTelValue().UpdateTValueWithSampling(th); err != nil {
					// UpdateTValueWithSampling only returns
					// ErrInconsistentSampling: the existing
					// threshold is stricter and the spec forbids
					// lowering it. Leave the span as-is.
					continue
				}
				var w strings.Builder
				// Serialize writes to a strings.Builder, which never
				// returns an error.
				_ = ts.Serialize(&w)
				span.TraceState().FromRaw(w.String())
			}
		}
	}
}
