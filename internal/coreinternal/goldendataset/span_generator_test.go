// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestGenerateParentSpan(t *testing.T) {
	random := rand.Reader
	traceID := generateTraceID(random)
	spanInputs := &PICTSpanInputs{
		Parent:     SpanParentRoot,
		Tracestate: TraceStateEmpty,
		Kind:       SpanKindServer,
		Attributes: SpanAttrHTTPServer,
		Events:     SpanChildCountTwo,
		Links:      SpanChildCountOne,
		Status:     SpanStatusOk,
	}
	span := ptrace.NewSpan()
	fillSpan(traceID, pcommon.SpanID([8]byte{}), "/gotest-parent", spanInputs, random, span)
	assert.Equal(t, traceID, span.TraceID())
	assert.True(t, span.ParentSpanID().IsEmpty())
	assert.Equal(t, 11, span.Attributes().Len())
	assert.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
}

func TestGenerateChildSpan(t *testing.T) {
	random := rand.Reader
	traceID := generateTraceID(random)
	parentID := generateSpanID(random)
	spanInputs := &PICTSpanInputs{
		Parent:     SpanParentChild,
		Tracestate: TraceStateEmpty,
		Kind:       SpanKindClient,
		Attributes: SpanAttrDatabaseSQL,
		Events:     SpanChildCountEmpty,
		Links:      SpanChildCountEmpty,
		Status:     SpanStatusOk,
	}
	span := ptrace.NewSpan()
	fillSpan(traceID, parentID, "get_test_info", spanInputs, random, span)
	assert.Equal(t, traceID, span.TraceID())
	assert.Equal(t, parentID, span.ParentSpanID())
	assert.Equal(t, 12, span.Attributes().Len())
	assert.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
}

func TestGenerateSpans(t *testing.T) {
	random := rand.Reader
	count1 := 16
	spans := ptrace.NewSpanSlice()
	err := appendSpans(count1, "testdata/generated_pict_pairs_spans.txt", random, spans)
	assert.NoError(t, err)
	assert.Equal(t, count1, spans.Len())

	count2 := 256
	spans = ptrace.NewSpanSlice()
	err = appendSpans(count2, "testdata/generated_pict_pairs_spans.txt", random, spans)
	assert.NoError(t, err)
	assert.Equal(t, count2, spans.Len())

	count3 := 118
	spans = ptrace.NewSpanSlice()
	err = appendSpans(count3, "testdata/generated_pict_pairs_spans.txt", random, spans)
	assert.NoError(t, err)
	assert.Equal(t, count3, spans.Len())
}
