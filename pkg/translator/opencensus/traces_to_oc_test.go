// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensus

import (
	"testing"

	occommon "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	ocresource "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	octrace "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/occonventions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
)

func TestInternalTraceStateToOC(t *testing.T) {
	assert.Equal(t, (*octrace.Span_Tracestate)(nil), traceStateToOC(""))

	ocTracestate := &octrace.Span_Tracestate{
		Entries: []*octrace.Span_Tracestate_Entry{
			{
				Key:   "abc",
				Value: "def",
			},
		},
	}
	assert.EqualValues(t, ocTracestate, traceStateToOC("abc=def"))

	ocTracestate.Entries = append(ocTracestate.Entries,
		&octrace.Span_Tracestate_Entry{
			Key:   "123",
			Value: "4567",
		})
	assert.EqualValues(t, ocTracestate, traceStateToOC("abc=def,123=4567"))
}

func TestAttributesMapToOC(t *testing.T) {
	assert.EqualValues(t, (*octrace.Span_Attributes)(nil), attributesMapToOCSpanAttributes(pcommon.NewMap(), 0))

	ocAttrs := &octrace.Span_Attributes{
		DroppedAttributesCount: 123,
	}
	assert.EqualValues(t, ocAttrs, attributesMapToOCSpanAttributes(pcommon.NewMap(), 123))

	ocAttrs = &octrace.Span_Attributes{
		AttributeMap: map[string]*octrace.AttributeValue{
			"abc": {
				Value: &octrace.AttributeValue_StringValue{StringValue: &octrace.TruncatableString{Value: "def"}},
			},
		},
		DroppedAttributesCount: 234,
	}
	attrs := pcommon.NewMap()
	attrs.PutStr("abc", "def")
	assert.EqualValues(t, ocAttrs, attributesMapToOCSpanAttributes(attrs, 234))

	ocAttrs.AttributeMap["intval"] = &octrace.AttributeValue{
		Value: &octrace.AttributeValue_IntValue{IntValue: 345},
	}
	ocAttrs.AttributeMap["boolval"] = &octrace.AttributeValue{
		Value: &octrace.AttributeValue_BoolValue{BoolValue: true},
	}
	ocAttrs.AttributeMap["doubleval"] = &octrace.AttributeValue{
		Value: &octrace.AttributeValue_DoubleValue{DoubleValue: 4.5},
	}
	assert.NoError(t, attrs.FromRaw(map[string]any{
		"abc":       "def",
		"intval":    345,
		"boolval":   true,
		"doubleval": 4.5,
	}))
	assert.EqualValues(t, ocAttrs, attributesMapToOCSpanAttributes(attrs, 234))
}

func TestSpanKindToOC(t *testing.T) {
	tests := []struct {
		kind   ptrace.SpanKind
		ocKind octrace.Span_SpanKind
	}{
		{
			kind:   ptrace.SpanKindClient,
			ocKind: octrace.Span_CLIENT,
		},
		{
			kind:   ptrace.SpanKindServer,
			ocKind: octrace.Span_SERVER,
		},
		{
			kind:   ptrace.SpanKindConsumer,
			ocKind: octrace.Span_SPAN_KIND_UNSPECIFIED,
		},
		{
			kind:   ptrace.SpanKindProducer,
			ocKind: octrace.Span_SPAN_KIND_UNSPECIFIED,
		},
		{
			kind:   ptrace.SpanKindUnspecified,
			ocKind: octrace.Span_SPAN_KIND_UNSPECIFIED,
		},
		{
			kind:   ptrace.SpanKindInternal,
			ocKind: octrace.Span_SPAN_KIND_UNSPECIFIED,
		},
	}

	for _, test := range tests {
		t.Run(test.kind.String(), func(t *testing.T) {
			got := spanKindToOC(test.kind)
			assert.EqualValues(t, test.ocKind, got, "Expected "+test.ocKind.String()+", got "+got.String())
		})
	}
}

func TestAttributesMapTOOcSameProcessAsParentSpan(t *testing.T) {
	attr := pcommon.NewMap()
	assert.Nil(t, attributesMapToOCSameProcessAsParentSpan(attr))

	attr.PutBool(occonventions.AttributeSameProcessAsParentSpan, true)
	assert.True(t, proto.Equal(wrapperspb.Bool(true), attributesMapToOCSameProcessAsParentSpan(attr)))

	attr.PutBool(occonventions.AttributeSameProcessAsParentSpan, false)
	assert.True(t, proto.Equal(wrapperspb.Bool(false), attributesMapToOCSameProcessAsParentSpan(attr)))

	attr.PutInt(occonventions.AttributeSameProcessAsParentSpan, 13)
	assert.Nil(t, attributesMapToOCSameProcessAsParentSpan(attr))
}

func TestSpanKindToOCAttribute(t *testing.T) {
	tests := []struct {
		kind        ptrace.SpanKind
		ocAttribute *octrace.AttributeValue
	}{
		{
			kind: ptrace.SpanKindConsumer,
			ocAttribute: &octrace.AttributeValue{
				Value: &octrace.AttributeValue_StringValue{
					StringValue: &octrace.TruncatableString{
						Value: string(tracetranslator.OpenTracingSpanKindConsumer),
					},
				},
			},
		},
		{
			kind: ptrace.SpanKindProducer,
			ocAttribute: &octrace.AttributeValue{
				Value: &octrace.AttributeValue_StringValue{
					StringValue: &octrace.TruncatableString{
						Value: string(tracetranslator.OpenTracingSpanKindProducer),
					},
				},
			},
		},
		{
			kind: ptrace.SpanKindInternal,
			ocAttribute: &octrace.AttributeValue{
				Value: &octrace.AttributeValue_StringValue{
					StringValue: &octrace.TruncatableString{
						Value: string(tracetranslator.OpenTracingSpanKindInternal),
					},
				},
			},
		},
		{
			kind:        ptrace.SpanKindUnspecified,
			ocAttribute: nil,
		},
		{
			kind:        ptrace.SpanKindServer,
			ocAttribute: nil,
		},
		{
			kind:        ptrace.SpanKindClient,
			ocAttribute: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.kind.String(), func(t *testing.T) {
			got := spanKindToOCAttribute(test.kind)
			assert.EqualValues(t, test.ocAttribute, got, "Expected "+test.ocAttribute.String()+", got "+got.String())
		})
	}
}

func TestInternalToOC(t *testing.T) {
	ocNode := &occommon.Node{}
	ocResource1 := &ocresource.Resource{Labels: map[string]string{"resource-attr": "resource-attr-val-1"}}

	startTime := timestamppb.New(testdata.TestSpanStartTime)
	eventTime := timestamppb.New(testdata.TestSpanEventTime)
	endTime := timestamppb.New(testdata.TestSpanEndTime)

	ocSpan1 := &octrace.Span{
		Name:      &octrace.TruncatableString{Value: "operationA"},
		StartTime: startTime,
		EndTime:   endTime,
		TimeEvents: &octrace.Span_TimeEvents{
			TimeEvent: []*octrace.Span_TimeEvent{
				{
					Time: eventTime,
					Value: &octrace.Span_TimeEvent_Annotation_{
						Annotation: &octrace.Span_TimeEvent_Annotation{
							Description: &octrace.TruncatableString{Value: "event-with-attr"},
							Attributes: &octrace.Span_Attributes{
								AttributeMap: map[string]*octrace.AttributeValue{
									"span-event-attr": {
										Value: &octrace.AttributeValue_StringValue{
											StringValue: &octrace.TruncatableString{Value: "span-event-attr-val"},
										},
									},
								},
								DroppedAttributesCount: 2,
							},
						},
					},
				},
				{
					Time: eventTime,
					Value: &octrace.Span_TimeEvent_Annotation_{
						Annotation: &octrace.Span_TimeEvent_Annotation{
							Description: &octrace.TruncatableString{Value: "event"},
							Attributes: &octrace.Span_Attributes{
								DroppedAttributesCount: 2,
							},
						},
					},
				},
			},
			DroppedAnnotationsCount: 1,
		},
		Attributes: &octrace.Span_Attributes{
			DroppedAttributesCount: 1,
		},
		Status: &octrace.Status{Message: "status-cancelled", Code: 2},
	}

	ocSpan2 := &octrace.Span{
		Name:      &octrace.TruncatableString{Value: "operationB"},
		StartTime: startTime,
		EndTime:   endTime,
		Links: &octrace.Span_Links{
			Link: []*octrace.Span_Link{
				{
					Attributes: &octrace.Span_Attributes{
						AttributeMap: map[string]*octrace.AttributeValue{
							"span-link-attr": {
								Value: &octrace.AttributeValue_StringValue{
									StringValue: &octrace.TruncatableString{Value: "span-link-attr-val"},
								},
							},
						},
						DroppedAttributesCount: 4,
					},
				},
				{
					Attributes: &octrace.Span_Attributes{
						DroppedAttributesCount: 4,
					},
				},
			},
			DroppedLinksCount: 3,
		},
		Status: &octrace.Status{},
	}

	tests := []struct {
		name     string
		td       ptrace.Traces
		Node     *occommon.Node
		Resource *ocresource.Resource
		Spans    []*octrace.Span
	}{
		{
			name:     "one-empty-resource-spans",
			td:       testdata.GenerateTracesOneEmptyResourceSpans(),
			Node:     nil,
			Resource: nil,
			Spans:    []*octrace.Span(nil),
		},

		{
			name:     "no-libraries",
			td:       testdata.GenerateTracesNoLibraries(),
			Node:     ocNode,
			Resource: ocResource1,
			Spans:    []*octrace.Span(nil),
		},

		{
			name:     "one-empty-instrumentation-library",
			td:       testdata.GenerateTracesOneEmptyInstrumentationLibrary(),
			Node:     ocNode,
			Resource: ocResource1,
			Spans:    []*octrace.Span{},
		},

		{
			name:     "one-span-no-resource",
			td:       testdata.GenerateTracesOneSpanNoResource(),
			Node:     nil,
			Resource: nil,
			Spans:    []*octrace.Span{ocSpan1},
		},

		{
			name:     "one-span",
			td:       testdata.GenerateTracesOneSpan(),
			Node:     ocNode,
			Resource: ocResource1,
			Spans:    []*octrace.Span{ocSpan1},
		},

		{
			name:     "two-spans-same-resource",
			td:       testdata.GenerateTracesTwoSpansSameResource(),
			Node:     ocNode,
			Resource: ocResource1,
			Spans:    []*octrace.Span{ocSpan1, ocSpan2},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotNode, gotResource, gotSpans := ResourceSpansToOC(test.td.ResourceSpans().At(0))
			assert.EqualValues(t, test.Node, gotNode)
			assert.EqualValues(t, test.Resource, gotResource)
			assert.EqualValues(t, test.Spans, gotSpans)
		})
	}
}

func TestInternalTracesToOCTracesAndBack(t *testing.T) {
	tds, err := goldendataset.GenerateTraces(
		"../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_traces.txt",
		"../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_spans.txt")
	assert.NoError(t, err)
	for _, td := range tds {
		ocNode, ocResource, ocSpans := ResourceSpansToOC(td.ResourceSpans().At(0))
		assert.Equal(t, td.SpanCount(), len(ocSpans))
		tdFromOC := OCToTraces(ocNode, ocResource, ocSpans)
		assert.NotNil(t, tdFromOC)
		assert.Equal(t, td.SpanCount(), tdFromOC.SpanCount())
	}
}
