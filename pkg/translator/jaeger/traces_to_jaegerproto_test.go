// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaeger

import (
	"testing"

	"github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.9.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
)

func TestGetTagFromStatusCode(t *testing.T) {
	tests := []struct {
		name string
		code ptrace.StatusCode
		tag  model.KeyValue
	}{
		{
			name: "ok",
			code: ptrace.StatusCodeOk,
			tag: model.KeyValue{
				Key:   conventions.OtelStatusCode,
				VType: model.ValueType_STRING,
				VStr:  statusOk,
			},
		},

		{
			name: "error",
			code: ptrace.StatusCodeError,
			tag: model.KeyValue{
				Key:   conventions.OtelStatusCode,
				VType: model.ValueType_STRING,
				VStr:  statusError,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := getTagFromStatusCode(test.code)
			assert.True(t, ok)
			assert.EqualValues(t, test.tag, got)
		})
	}
}

func TestGetErrorTagFromStatusCode(t *testing.T) {
	errTag := model.KeyValue{
		Key:   tracetranslator.TagError,
		VBool: true,
		VType: model.ValueType_BOOL,
	}

	_, ok := getErrorTagFromStatusCode(ptrace.StatusCodeUnset)
	assert.False(t, ok)

	_, ok = getErrorTagFromStatusCode(ptrace.StatusCodeOk)
	assert.False(t, ok)

	got, ok := getErrorTagFromStatusCode(ptrace.StatusCodeError)
	assert.True(t, ok)
	assert.EqualValues(t, errTag, got)
}

func TestGetTagFromStatusMsg(t *testing.T) {
	_, ok := getTagFromStatusMsg("")
	assert.False(t, ok)

	got, ok := getTagFromStatusMsg("test-error")
	assert.True(t, ok)
	assert.EqualValues(t, model.KeyValue{
		Key:   conventions.OtelStatusDescription,
		VStr:  "test-error",
		VType: model.ValueType_STRING,
	}, got)
}

func TestGetTagFromSpanKind(t *testing.T) {
	tests := []struct {
		name string
		kind ptrace.SpanKind
		tag  model.KeyValue
		ok   bool
	}{
		{
			name: "unspecified",
			kind: ptrace.SpanKindUnspecified,
			tag:  model.KeyValue{},
			ok:   false,
		},

		{
			name: "client",
			kind: ptrace.SpanKindClient,
			tag: model.KeyValue{
				Key:   tracetranslator.TagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  string(tracetranslator.OpenTracingSpanKindClient),
			},
			ok: true,
		},

		{
			name: "server",
			kind: ptrace.SpanKindServer,
			tag: model.KeyValue{
				Key:   tracetranslator.TagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  string(tracetranslator.OpenTracingSpanKindServer),
			},
			ok: true,
		},

		{
			name: "producer",
			kind: ptrace.SpanKindProducer,
			tag: model.KeyValue{
				Key:   tracetranslator.TagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  string(tracetranslator.OpenTracingSpanKindProducer),
			},
			ok: true,
		},

		{
			name: "consumer",
			kind: ptrace.SpanKindConsumer,
			tag: model.KeyValue{
				Key:   tracetranslator.TagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  string(tracetranslator.OpenTracingSpanKindConsumer),
			},
			ok: true,
		},

		{
			name: "internal",
			kind: ptrace.SpanKindInternal,
			tag: model.KeyValue{
				Key:   tracetranslator.TagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  string(tracetranslator.OpenTracingSpanKindInternal),
			},
			ok: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := getTagFromSpanKind(test.kind)
			assert.Equal(t, test.ok, ok)
			assert.EqualValues(t, test.tag, got)
		})
	}
}

func TestAttributesToJaegerProtoTags(t *testing.T) {

	attributes := pcommon.NewMap()
	attributes.PutBool("bool-val", true)
	attributes.PutInt("int-val", 123)
	attributes.PutStr("string-val", "abc")
	attributes.PutDouble("double-val", 1.23)
	attributes.PutEmptyBytes("bytes-val").FromRaw([]byte{1, 2, 3, 4})
	attributes.PutStr(conventions.AttributeServiceName, "service-name")

	expected := []model.KeyValue{
		{
			Key:   "bool-val",
			VType: model.ValueType_BOOL,
			VBool: true,
		},
		{
			Key:    "int-val",
			VType:  model.ValueType_INT64,
			VInt64: 123,
		},
		{
			Key:   "string-val",
			VType: model.ValueType_STRING,
			VStr:  "abc",
		},
		{
			Key:      "double-val",
			VType:    model.ValueType_FLOAT64,
			VFloat64: 1.23,
		},
		{
			Key:   "bytes-val",
			VType: model.ValueType_STRING,
			VStr:  "AQIDBA==", // base64 encoding of the byte array [1,2,3,4]
		},
		{
			Key:   conventions.AttributeServiceName,
			VType: model.ValueType_STRING,
			VStr:  "service-name",
		},
	}

	got := appendTagsFromAttributes(make([]model.KeyValue, 0, len(expected)), attributes)
	require.EqualValues(t, expected, got)

	// The last item in expected ("service-name") must be skipped in resource tags translation
	got = appendTagsFromResourceAttributes(make([]model.KeyValue, 0, len(expected)-1), attributes)
	require.EqualValues(t, expected[:5], got)
}

func TestInternalTracesToJaegerProto(t *testing.T) {

	tests := []struct {
		name string
		td   ptrace.Traces
		jb   *model.Batch
		err  error
	}{
		{
			name: "empty",
			td:   ptrace.NewTraces(),
			err:  nil,
		},

		{
			name: "no-spans",
			td:   generateTracesResourceOnly(),
			jb: &model.Batch{
				Process: generateProtoProcess(),
			},
			err: nil,
		},

		{
			name: "no-resource-attrs",
			td:   generateTracesResourceOnlyWithNoAttrs(),
			err:  nil,
		},

		{
			name: "one-span-no-resources",
			td:   generateTracesOneSpanNoResourceWithTraceState(),
			jb: &model.Batch{
				Process: &model.Process{
					ServiceName: tracetranslator.ResourceNoServiceName,
				},
				Spans: []*model.Span{
					generateProtoSpanWithTraceState(),
				},
			},
			err: nil,
		},
		{
			name: "library-info",
			td:   generateTracesWithLibraryInfo(),
			jb: &model.Batch{
				Process: &model.Process{
					ServiceName: tracetranslator.ResourceNoServiceName,
				},
				Spans: []*model.Span{
					generateProtoSpanWithLibraryInfo("io.opentelemetry.test"),
				},
			},
			err: nil,
		},
		{
			name: "two-spans-child-parent",
			td:   generateTracesTwoSpansChildParent(),
			jb: &model.Batch{
				Process: &model.Process{
					ServiceName: tracetranslator.ResourceNoServiceName,
				},
				Spans: []*model.Span{
					generateProtoSpan(),
					generateProtoChildSpan(),
				},
			},
			err: nil,
		},

		{
			name: "two-spans-with-follower",
			td:   generateTracesTwoSpansWithFollower(),
			jb: &model.Batch{
				Process: &model.Process{
					ServiceName: tracetranslator.ResourceNoServiceName,
				},
				Spans: []*model.Span{
					generateProtoSpan(),
					generateProtoFollowerSpan(),
				},
			},
			err: nil,
		},

		{
			name: "span-with-span-event-attribute",
			td:   generateTracesOneSpanNoResourceWithEventAttribute(),
			jb: &model.Batch{
				Process: &model.Process{
					ServiceName: tracetranslator.ResourceNoServiceName,
				},
				Spans: []*model.Span{
					generateJProtoSpanWithEventAttribute(),
				},
			},
			err: nil,
		},
		{
			name: "a-spans-with-two-parent",
			jb: &model.Batch{
				Process: &model.Process{
					ServiceName: tracetranslator.ResourceNoServiceName,
				},
				Spans: []*model.Span{
					generateProtoSpan(),
					generateProtoFollowerSpan(),
					generateProtoTwoParentsSpan(),
				},
			},
			td: generateTracesSpanWithTwoParents(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			jbs, err := ProtoFromTraces(test.td)
			assert.EqualValues(t, test.err, err)
			if test.jb == nil {
				assert.Len(t, jbs, 0)
			} else {
				require.Equal(t, 1, len(jbs))
				assert.EqualValues(t, test.jb, jbs[0])
			}
		})
	}
}

func TestInternalTracesToJaegerProtoBatchesAndBack(t *testing.T) {
	tds, err := goldendataset.GenerateTraces(
		"../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_traces.txt",
		"../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_spans.txt")
	assert.NoError(t, err)
	for _, td := range tds {
		protoBatches, err := ProtoFromTraces(td)
		assert.NoError(t, err)
		tdFromPB, err := ProtoToTraces(protoBatches)
		assert.NoError(t, err)
		assert.Equal(t, td.SpanCount(), tdFromPB.SpanCount())
	}
}

func generateTracesOneSpanNoResourceWithEventAttribute() ptrace.Traces {
	td := generateTracesOneSpanNoResource()
	event := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().At(0)
	event.SetName("must-be-ignorred")
	event.Attributes().PutStr("event", "must-be-used-instead-of-event-name")
	return td
}

func generateJProtoSpanWithEventAttribute() *model.Span {
	span := generateProtoSpan()
	span.Logs[0].Fields = []model.KeyValue{
		{
			Key:   "span-event-attr",
			VType: model.ValueType_STRING,
			VStr:  "span-event-attr-val",
		},
		{
			Key:   eventNameAttr,
			VType: model.ValueType_STRING,
			VStr:  "must-be-used-instead-of-event-name",
		},
	}
	return span
}

func BenchmarkInternalTracesToJaegerProto(b *testing.B) {
	td := generateTracesTwoSpansChildParent()
	resource := generateTracesResourceOnly().ResourceSpans().At(0).Resource()
	resource.CopyTo(td.ResourceSpans().At(0).Resource())

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := ProtoFromTraces(td)
		assert.NoError(b, err)
	}
}
