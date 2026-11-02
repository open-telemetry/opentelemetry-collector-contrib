// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traceutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestGroupSpansByTraceID(t *testing.T) {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans()

	firstScope := resourceSpans.AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	appendGroupedSpan(firstScope, pcommon.TraceID([16]byte{1}), 1)
	appendGroupedSpan(firstScope, pcommon.TraceID([16]byte{1}), 2)

	secondScope := resourceSpans.AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	appendGroupedSpan(secondScope, pcommon.TraceID([16]byte{2}), 3)

	grouped := GroupSpansByTraceID(traces)

	assert.Len(t, grouped, 2)
	assert.Len(t, grouped[pcommon.TraceID([16]byte{1})], 2)
	assert.Len(t, grouped[pcommon.TraceID([16]byte{2})], 1)
}

func appendGroupedSpan(spans ptrace.SpanSlice, traceID pcommon.TraceID, spanID byte) {
	span := spans.AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.SpanID([8]byte{spanID}))
}
