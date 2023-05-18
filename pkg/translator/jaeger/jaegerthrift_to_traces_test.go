// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaeger

import (
	"encoding/binary"
	"testing"

	"github.com/jaegertracing/jaeger/thrift-gen/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.9.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
)

func TestJThriftTagsToInternalAttributes(t *testing.T) {
	var intVal int64 = 123
	boolVal := true
	stringVal := "abc"
	doubleVal := 1.23
	tags := []*jaeger.Tag{
		{
			Key:   "bool-val",
			VType: jaeger.TagType_BOOL,
			VBool: &boolVal,
		},
		{
			Key:   "int-val",
			VType: jaeger.TagType_LONG,
			VLong: &intVal,
		},
		{
			Key:   "string-val",
			VType: jaeger.TagType_STRING,
			VStr:  &stringVal,
		},
		{
			Key:     "double-val",
			VType:   jaeger.TagType_DOUBLE,
			VDouble: &doubleVal,
		},
		{
			Key:     "binary-val",
			VType:   jaeger.TagType_BINARY,
			VBinary: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x7D, 0x98},
		},
	}

	expected := pcommon.NewMap()
	expected.PutBool("bool-val", true)
	expected.PutInt("int-val", 123)
	expected.PutStr("string-val", "abc")
	expected.PutDouble("double-val", 1.23)
	expected.PutStr("binary-val", "AAAAAABkfZg=")

	got := pcommon.NewMap()
	jThriftTagsToInternalAttributes(tags, got)

	require.EqualValues(t, expected, got)
}

func TestThriftBatchToInternalTraces(t *testing.T) {

	tests := []struct {
		name string
		jb   *jaeger.Batch
		td   ptrace.Traces
	}{
		{
			name: "empty",
			jb:   &jaeger.Batch{},
			td:   ptrace.NewTraces(),
		},

		{
			name: "no-spans",
			jb: &jaeger.Batch{
				Process: generateThriftProcess(),
			},
			td: testdata.GenerateTracesNoLibraries(),
		},

		{
			name: "one-span-no-resources",
			jb: &jaeger.Batch{
				Spans: []*jaeger.Span{
					generateThriftSpan(),
				},
			},
			td: generateTracesOneSpanNoResource(),
		},
		{
			name: "two-spans-child-parent",
			jb: &jaeger.Batch{
				Spans: []*jaeger.Span{
					generateThriftSpan(),
					generateThriftChildSpan(),
				},
			},
			td: generateTracesTwoSpansChildParent(),
		},

		{
			name: "a-spans-with-two-parent",
			jb: &jaeger.Batch{
				Spans: []*jaeger.Span{
					generateThriftSpan(),
					generateThriftFollowerSpan(),
					generateThriftTwoParentsSpan(),
				},
			},
			td: generateTracesSpanWithTwoParents(),
		},
		{
			name: "two-spans-with-follower",
			jb: &jaeger.Batch{
				Spans: []*jaeger.Span{
					generateThriftSpan(),
					generateThriftFollowerSpan(),
				},
			},
			td: generateTracesTwoSpansWithFollower(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			td, err := ThriftToTraces(test.jb)
			assert.NoError(t, err)
			assert.EqualValues(t, test.td, td)
		})
	}
}

func generateThriftProcess() *jaeger.Process {
	attrVal := "resource-attr-val-1"
	return &jaeger.Process{
		Tags: []*jaeger.Tag{
			{
				Key:   "resource-attr",
				VType: jaeger.TagType_STRING,
				VStr:  &attrVal,
			},
		},
	}
}

func generateThriftSpan() *jaeger.Span {
	spanStartTs := unixNanoToMicroseconds(testSpanStartTimestamp)
	spanEndTs := unixNanoToMicroseconds(testSpanEndTimestamp)
	eventTs := unixNanoToMicroseconds(testSpanEventTimestamp)
	intAttrVal := int64(123)
	eventName := "event-with-attr"
	eventStrAttrVal := "span-event-attr-val"
	statusCode := statusError
	statusMsg := "status-cancelled"
	kind := string(tracetranslator.OpenTracingSpanKindClient)

	return &jaeger.Span{
		TraceIdHigh:   int64(binary.BigEndian.Uint64([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8})),
		TraceIdLow:    int64(binary.BigEndian.Uint64([]byte{0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})),
		SpanId:        int64(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})),
		OperationName: "operationA",
		StartTime:     spanStartTs,
		Duration:      spanEndTs - spanStartTs,
		Logs: []*jaeger.Log{
			{
				Timestamp: eventTs,
				Fields: []*jaeger.Tag{
					{
						Key:   eventNameAttr,
						VType: jaeger.TagType_STRING,
						VStr:  &eventName,
					},
					{
						Key:   "span-event-attr",
						VType: jaeger.TagType_STRING,
						VStr:  &eventStrAttrVal,
					},
				},
			},
			{
				Timestamp: eventTs,
				Fields: []*jaeger.Tag{
					{
						Key:   "attr-int",
						VType: jaeger.TagType_LONG,
						VLong: &intAttrVal,
					},
				},
			},
		},
		Tags: []*jaeger.Tag{
			{
				Key:   conventions.OtelStatusCode,
				VType: jaeger.TagType_STRING,
				VStr:  &statusCode,
			},
			{
				Key:   conventions.OtelStatusDescription,
				VType: jaeger.TagType_STRING,
				VStr:  &statusMsg,
			},
			{
				Key:   tracetranslator.TagSpanKind,
				VType: jaeger.TagType_STRING,
				VStr:  &kind,
			},
		},
	}
}

func generateThriftChildSpan() *jaeger.Span {
	spanStartTs := unixNanoToMicroseconds(testSpanStartTimestamp)
	spanEndTs := unixNanoToMicroseconds(testSpanEndTimestamp)
	notFoundAttrVal := int64(404)
	kind := string(tracetranslator.OpenTracingSpanKindServer)

	return &jaeger.Span{
		TraceIdHigh:   int64(binary.BigEndian.Uint64([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8})),
		TraceIdLow:    int64(binary.BigEndian.Uint64([]byte{0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})),
		SpanId:        int64(binary.BigEndian.Uint64([]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18})),
		ParentSpanId:  int64(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})),
		OperationName: "operationB",
		StartTime:     spanStartTs,
		Duration:      spanEndTs - spanStartTs,
		Tags: []*jaeger.Tag{
			{
				Key:   conventions.AttributeHTTPStatusCode,
				VType: jaeger.TagType_LONG,
				VLong: &notFoundAttrVal,
			},
			{
				Key:   tracetranslator.TagSpanKind,
				VType: jaeger.TagType_STRING,
				VStr:  &kind,
			},
		},
	}
}

func generateThriftFollowerSpan() *jaeger.Span {
	statusCode := statusOk
	statusMsg := "status-ok"
	kind := string(tracetranslator.OpenTracingSpanKindConsumer)

	return &jaeger.Span{
		TraceIdHigh:   int64(binary.BigEndian.Uint64([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8})),
		TraceIdLow:    int64(binary.BigEndian.Uint64([]byte{0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})),
		SpanId:        int64(binary.BigEndian.Uint64([]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18})),
		OperationName: "operationC",
		StartTime:     unixNanoToMicroseconds(testSpanEndTimestamp),
		Duration:      1000,
		Tags: []*jaeger.Tag{
			{
				Key:   conventions.OtelStatusCode,
				VType: jaeger.TagType_STRING,
				VStr:  &statusCode,
			},
			{
				Key:   conventions.OtelStatusDescription,
				VType: jaeger.TagType_STRING,
				VStr:  &statusMsg,
			},
			{
				Key:   tracetranslator.TagSpanKind,
				VType: jaeger.TagType_STRING,
				VStr:  &kind,
			},
		},
		References: []*jaeger.SpanRef{
			{
				TraceIdHigh: int64(binary.BigEndian.Uint64([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8})),
				TraceIdLow:  int64(binary.BigEndian.Uint64([]byte{0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})),
				SpanId:      int64(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})),
				RefType:     jaeger.SpanRefType_FOLLOWS_FROM,
			},
		},
	}
}

func generateThriftTwoParentsSpan() *jaeger.Span {
	spanStartTs := unixNanoToMicroseconds(testSpanStartTimestamp)
	spanEndTs := unixNanoToMicroseconds(testSpanEndTimestamp)
	statusCode := statusOk
	statusMsg := "status-ok"
	kind := string(tracetranslator.OpenTracingSpanKindConsumer)

	return &jaeger.Span{
		TraceIdHigh:   int64(binary.BigEndian.Uint64([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8})),
		TraceIdLow:    int64(binary.BigEndian.Uint64([]byte{0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})),
		SpanId:        int64(binary.BigEndian.Uint64([]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x20})),
		OperationName: "operationD",
		StartTime:     spanStartTs,
		Duration:      spanEndTs - spanStartTs,
		ParentSpanId:  int64(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})),
		Tags: []*jaeger.Tag{
			{
				Key:   conventions.OtelStatusCode,
				VType: jaeger.TagType_STRING,
				VStr:  &statusCode,
			},
			{
				Key:   conventions.OtelStatusDescription,
				VType: jaeger.TagType_STRING,
				VStr:  &statusMsg,
			},
			{
				Key:   tracetranslator.TagSpanKind,
				VType: jaeger.TagType_STRING,
				VStr:  &kind,
			},
		},
		References: []*jaeger.SpanRef{
			{
				TraceIdHigh: int64(binary.BigEndian.Uint64([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8})),
				TraceIdLow:  int64(binary.BigEndian.Uint64([]byte{0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})),
				SpanId:      int64(binary.BigEndian.Uint64([]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18})),
				RefType:     jaeger.SpanRefType_CHILD_OF,
			},
		},
	}
}
func unixNanoToMicroseconds(ns pcommon.Timestamp) int64 {
	return int64(ns / 1000)
}

func BenchmarkThriftBatchToInternalTraces(b *testing.B) {
	jb := &jaeger.Batch{
		Process: generateThriftProcess(),
		Spans: []*jaeger.Span{
			generateThriftSpan(),
			generateThriftChildSpan(),
		},
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := ThriftToTraces(jb)
		assert.NoError(b, err)
	}
}
