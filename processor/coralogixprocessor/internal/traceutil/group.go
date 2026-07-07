// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traceutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/traceutil"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// GroupSpansByTraceID collects spans from a traces payload by trace ID.
func GroupSpansByTraceID(td ptrace.Traces) map[pcommon.TraceID][]ptrace.Span {
	traceSpanMap := make(map[pcommon.TraceID][]ptrace.Span)
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			scopeSpans := rs.ScopeSpans().At(j)
			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				span := scopeSpans.Spans().At(k)
				traceID := span.TraceID()
				traceSpanMap[traceID] = append(traceSpanMap[traceID], span)
			}
		}
	}
	return traceSpanMap
}
