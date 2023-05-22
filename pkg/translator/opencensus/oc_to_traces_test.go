// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensus

import (
	"strconv"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/occonventions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestOcTraceStateToInternal(t *testing.T) {
	assert.EqualValues(t, "", ocTraceStateToInternal(nil))

	tracestate := &octrace.Span_Tracestate{
		Entries: []*octrace.Span_Tracestate_Entry{
			{
				Key:   "abc",
				Value: "def",
			},
		},
	}
	assert.EqualValues(t, "abc=def", ocTraceStateToInternal(tracestate))

	tracestate.Entries = append(tracestate.Entries,
		&octrace.Span_Tracestate_Entry{
			Key:   "123",
			Value: "4567",
		})
	assert.EqualValues(t, "abc=def,123=4567", ocTraceStateToInternal(tracestate))
}

func TestInitAttributeMapFromOC(t *testing.T) {
	attrs := pcommon.NewMap()
	initAttributeMapFromOC(nil, attrs)
	assert.EqualValues(t, pcommon.NewMap(), attrs)
	assert.EqualValues(t, 0, ocAttrsToDroppedAttributes(nil))

	ocAttrs := &octrace.Span_Attributes{}
	attrs = pcommon.NewMap()
	initAttributeMapFromOC(ocAttrs, attrs)
	assert.EqualValues(t, pcommon.NewMap(), attrs)
	assert.EqualValues(t, 0, ocAttrsToDroppedAttributes(ocAttrs))

	ocAttrs = &octrace.Span_Attributes{
		DroppedAttributesCount: 123,
	}
	attrs = pcommon.NewMap()
	initAttributeMapFromOC(ocAttrs, attrs)
	assert.EqualValues(t, pcommon.NewMap(), attrs)
	assert.EqualValues(t, 123, ocAttrsToDroppedAttributes(ocAttrs))

	ocAttrs = &octrace.Span_Attributes{
		AttributeMap:           map[string]*octrace.AttributeValue{},
		DroppedAttributesCount: 234,
	}
	attrs = pcommon.NewMap()
	initAttributeMapFromOC(ocAttrs, attrs)
	assert.EqualValues(t, pcommon.NewMap(), attrs)
	assert.EqualValues(t, 234, ocAttrsToDroppedAttributes(ocAttrs))

	ocAttrs = &octrace.Span_Attributes{
		AttributeMap: map[string]*octrace.AttributeValue{
			"abc": {
				Value: &octrace.AttributeValue_StringValue{StringValue: &octrace.TruncatableString{Value: "def"}},
			},
		},
		DroppedAttributesCount: 234,
	}
	attrs = pcommon.NewMap()
	initAttributeMapFromOC(ocAttrs, attrs)
	assert.Equal(t, map[string]interface{}{"abc": "def"}, attrs.AsRaw())
	assert.EqualValues(t, 234, ocAttrsToDroppedAttributes(ocAttrs))

	ocAttrs.AttributeMap["intval"] = &octrace.AttributeValue{
		Value: &octrace.AttributeValue_IntValue{IntValue: 345},
	}
	ocAttrs.AttributeMap["boolval"] = &octrace.AttributeValue{
		Value: &octrace.AttributeValue_BoolValue{BoolValue: true},
	}
	ocAttrs.AttributeMap["doubleval"] = &octrace.AttributeValue{
		Value: &octrace.AttributeValue_DoubleValue{DoubleValue: 4.5},
	}
	attrs = pcommon.NewMap()
	initAttributeMapFromOC(ocAttrs, attrs)

	assert.EqualValues(t, map[string]interface{}{
		"abc":       "def",
		"intval":    int64(345),
		"boolval":   true,
		"doubleval": 4.5,
	}, attrs.AsRaw())
	assert.EqualValues(t, 234, ocAttrsToDroppedAttributes(ocAttrs))
}

func TestOcSpanKindToInternal(t *testing.T) {
	tests := []struct {
		ocAttrs  *octrace.Span_Attributes
		ocKind   octrace.Span_SpanKind
		otlpKind ptrace.SpanKind
	}{
		{
			ocKind:   octrace.Span_CLIENT,
			otlpKind: ptrace.SpanKindClient,
		},
		{
			ocKind:   octrace.Span_SERVER,
			otlpKind: ptrace.SpanKindServer,
		},
		{
			ocKind:   octrace.Span_SPAN_KIND_UNSPECIFIED,
			otlpKind: ptrace.SpanKindUnspecified,
		},
		{
			ocKind: octrace.Span_SPAN_KIND_UNSPECIFIED,
			ocAttrs: &octrace.Span_Attributes{
				AttributeMap: map[string]*octrace.AttributeValue{
					"span.kind": {Value: &octrace.AttributeValue_StringValue{
						StringValue: &octrace.TruncatableString{Value: "consumer"}}},
				},
			},
			otlpKind: ptrace.SpanKindConsumer,
		},
		{
			ocKind: octrace.Span_SPAN_KIND_UNSPECIFIED,
			ocAttrs: &octrace.Span_Attributes{
				AttributeMap: map[string]*octrace.AttributeValue{
					"span.kind": {Value: &octrace.AttributeValue_StringValue{
						StringValue: &octrace.TruncatableString{Value: "producer"}}},
				},
			},
			otlpKind: ptrace.SpanKindProducer,
		},
		{
			ocKind: octrace.Span_SPAN_KIND_UNSPECIFIED,
			ocAttrs: &octrace.Span_Attributes{
				AttributeMap: map[string]*octrace.AttributeValue{
					"span.kind": {Value: &octrace.AttributeValue_IntValue{
						IntValue: 123}},
				},
			},
			otlpKind: ptrace.SpanKindUnspecified,
		},
		{
			ocKind: octrace.Span_CLIENT,
			ocAttrs: &octrace.Span_Attributes{
				AttributeMap: map[string]*octrace.AttributeValue{
					"span.kind": {Value: &octrace.AttributeValue_StringValue{
						StringValue: &octrace.TruncatableString{Value: "consumer"}}},
				},
			},
			otlpKind: ptrace.SpanKindClient,
		},
		{
			ocKind: octrace.Span_SPAN_KIND_UNSPECIFIED,
			ocAttrs: &octrace.Span_Attributes{
				AttributeMap: map[string]*octrace.AttributeValue{
					"span.kind": {Value: &octrace.AttributeValue_StringValue{
						StringValue: &octrace.TruncatableString{Value: "internal"}}},
				},
			},
			otlpKind: ptrace.SpanKindInternal,
		},
	}

	for _, test := range tests {
		t.Run(test.otlpKind.String(), func(t *testing.T) {
			got := ocSpanKindToInternal(test.ocKind, test.ocAttrs)
			assert.EqualValues(t, test.otlpKind, got, "Expected "+test.otlpKind.String()+", got "+got.String())
		})
	}
}

func TestOcToInternal(t *testing.T) {
	ocNode := &occommon.Node{}
	ocResource1 := &ocresource.Resource{Labels: map[string]string{"resource-attr": "resource-attr-val-1"}}
	ocResource2 := &ocresource.Resource{Labels: map[string]string{"resource-attr": "resource-attr-val-2"}}

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
		Status: &octrace.Status{Message: "status-cancelled", Code: 1},
	}

	// TODO: Create another unit test fully covering ocSpanToInternal
	ocSpanZeroedParentID := proto.Clone(ocSpan1).(*octrace.Span)
	ocSpanZeroedParentID.ParentSpanId = []byte{0, 0, 0, 0, 0, 0, 0, 0}

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
	}

	ocSpan3 := &octrace.Span{
		Name:      &octrace.TruncatableString{Value: "operationC"},
		StartTime: startTime,
		EndTime:   endTime,
		Resource:  ocResource2,
		Attributes: &octrace.Span_Attributes{
			AttributeMap: map[string]*octrace.AttributeValue{
				"span-attr": {
					Value: &octrace.AttributeValue_StringValue{
						StringValue: &octrace.TruncatableString{Value: "span-attr-val"},
					},
				},
			},
			DroppedAttributesCount: 5,
		},
	}

	tests := []struct {
		name     string
		td       ptrace.Traces
		node     *occommon.Node
		resource *ocresource.Resource
		spans    []*octrace.Span
	}{
		{
			name: "empty",
			td:   ptrace.NewTraces(),
		},

		{
			name: "one-empty-resource-spans",
			td:   testdata.GenerateTracesOneEmptyResourceSpans(),
			node: ocNode,
		},

		{
			name:     "no-libraries",
			td:       testdata.GenerateTracesNoLibraries(),
			resource: ocResource1,
		},

		{
			name:     "one-span-no-resource",
			td:       testdata.GenerateTracesOneSpanNoResource(),
			node:     ocNode,
			resource: &ocresource.Resource{},
			spans:    []*octrace.Span{ocSpan1},
		},

		{
			name:     "one-span",
			td:       testdata.GenerateTracesOneSpan(),
			node:     ocNode,
			resource: ocResource1,
			spans:    []*octrace.Span{ocSpan1},
		},

		{
			name:     "one-span-zeroed-parent-id",
			td:       testdata.GenerateTracesOneSpan(),
			node:     ocNode,
			resource: ocResource1,
			spans:    []*octrace.Span{ocSpanZeroedParentID},
		},

		{
			name:     "one-span-one-nil",
			td:       testdata.GenerateTracesOneSpan(),
			node:     ocNode,
			resource: ocResource1,
			spans:    []*octrace.Span{ocSpan1, nil},
		},

		{
			name:     "two-spans-same-resource",
			td:       testdata.GenerateTracesTwoSpansSameResource(),
			node:     ocNode,
			resource: ocResource1,
			spans:    []*octrace.Span{ocSpan1, nil, ocSpan2},
		},

		{
			name:     "two-spans-same-resource-one-different",
			td:       testdata.GenerateTracesTwoSpansSameResourceOneDifferent(),
			node:     ocNode,
			resource: ocResource1,
			spans:    []*octrace.Span{ocSpan1, ocSpan2, ocSpan3},
		},

		{
			name:     "two-spans-and-separate-in-the-middle",
			td:       testdata.GenerateTracesTwoSpansSameResourceOneDifferent(),
			node:     ocNode,
			resource: ocResource1,
			spans:    []*octrace.Span{ocSpan1, ocSpan3, ocSpan2},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.EqualValues(t, test.td, OCToTraces(test.node, test.resource, test.spans))
		})
	}
}

func TestOcSameProcessAsParentSpanToInternal(t *testing.T) {
	span := ptrace.NewSpan()
	ocSameProcessAsParentSpanToInternal(nil, span)
	assert.Equal(t, 0, span.Attributes().Len())

	ocSameProcessAsParentSpanToInternal(wrapperspb.Bool(false), span)
	assert.Equal(t, 1, span.Attributes().Len())
	v, ok := span.Attributes().Get(occonventions.AttributeSameProcessAsParentSpan)
	assert.True(t, ok)
	assert.EqualValues(t, pcommon.ValueTypeBool, v.Type())
	assert.False(t, v.Bool())

	ocSameProcessAsParentSpanToInternal(wrapperspb.Bool(true), span)
	assert.Equal(t, 1, span.Attributes().Len())
	v, ok = span.Attributes().Get(occonventions.AttributeSameProcessAsParentSpan)
	assert.True(t, ok)
	assert.EqualValues(t, pcommon.ValueTypeBool, v.Type())
	assert.True(t, v.Bool())
}

func BenchmarkSpansWithAttributesOCToInternal(b *testing.B) {
	resource := generateOCTestResource()
	spans := []*octrace.Span{generateSpanWithAttributes(15)}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		OCToTraces(nil, resource, spans)
	}
}

func BenchmarkSpansWithAttributesUnmarshal(b *testing.B) {
	ocSpan := generateSpanWithAttributes(15)

	bytes, err := proto.Marshal(ocSpan)
	if err != nil {
		b.Fail()
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		unmarshalOc := &octrace.Span{}
		if err := proto.Unmarshal(bytes, unmarshalOc); err != nil {
			b.Fail()
		}
		if len(unmarshalOc.Attributes.AttributeMap) != 15 {
			b.Fail()
		}
	}
}

func generateSpanWithAttributes(len int) *octrace.Span {
	startTime := timestamppb.New(testdata.TestSpanStartTime)
	endTime := timestamppb.New(testdata.TestSpanEndTime)
	ocSpan2 := &octrace.Span{
		Name:      &octrace.TruncatableString{Value: "operationB"},
		StartTime: startTime,
		EndTime:   endTime,
		Attributes: &octrace.Span_Attributes{
			DroppedAttributesCount: 3,
		},
	}

	ocSpan2.Attributes.AttributeMap = make(map[string]*octrace.AttributeValue, len)
	ocAttr := ocSpan2.Attributes.AttributeMap
	for i := 0; i < len; i++ {
		ocAttr["span-link-attr_"+strconv.Itoa(i)] = &octrace.AttributeValue{
			Value: &octrace.AttributeValue_StringValue{
				StringValue: &octrace.TruncatableString{Value: "span-link-attr-val"},
			},
		}
	}
	return ocSpan2
}
