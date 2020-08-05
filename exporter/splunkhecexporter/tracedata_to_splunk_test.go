// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter

import (
	"testing"
	"time"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/openzipkin/zipkin-go/model"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

func Test_traceDataToSplunk(t *testing.T) {
	logger := zap.NewNop()
	ts := &timestamp.Timestamp{
		Nanos: 0,
	}

	tests := []struct {
		name                string
		traceDataFn         func() consumerdata.TraceData
		wantSplunkEvents    []*splunkEvent
		wantNumDroppedSpans int
	}{
		{
			name: "valid",
			traceDataFn: func() consumerdata.TraceData {
				return consumerdata.TraceData{
					Spans: []*v1.Span{
						makeSpan("myspan", ts),
					},
				}
			},
			wantSplunkEvents: []*splunkEvent{
				commonSplunkEvent("myspan", ts),
			},
			wantNumDroppedSpans: 0,
		},
		{
			name: "missing_start_ts",
			traceDataFn: func() consumerdata.TraceData {
				return consumerdata.TraceData{
					Spans: []*v1.Span{
						makeSpan("myspan", nil),
					},
				}
			},
			wantSplunkEvents:    []*splunkEvent{},
			wantNumDroppedSpans: 1,
		},
		{
			name: "bad_tag_value",
			traceDataFn: func() consumerdata.TraceData {
				span := makeSpan("myspan", ts)
				span.Attributes = &v1.Span_Attributes{
					AttributeMap: map[string]*v1.AttributeValue{
						"foo": {
							Value: nil,
						},
					},
				}
				return consumerdata.TraceData{
					Spans: []*v1.Span{
						span,
					},
				}
			},
			wantSplunkEvents:    []*splunkEvent{},
			wantNumDroppedSpans: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEvents, gotNumDroppedSpans := traceDataToSplunk(logger, tt.traceDataFn(), &Config{})
			assert.Equal(t, tt.wantNumDroppedSpans, gotNumDroppedSpans)
			assert.Equal(t, len(tt.wantSplunkEvents), len(gotEvents))
			for i, want := range tt.wantSplunkEvents {
				assert.EqualValues(t, want, gotEvents[i])
			}
			assert.Equal(t, tt.wantSplunkEvents, gotEvents)
		})
	}
}

func makeSpan(name string, ts *timestamp.Timestamp) *v1.Span {
	trunceableName := &v1.TruncatableString{
		Value: name,
	}
	return &v1.Span{
		TraceId:      []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 8, 7, 6, 5, 4, 3, 2},
		SpanId:       []byte{1, 2, 3, 4, 5, 6, 7, 8},
		ParentSpanId: []byte{2, 1, 3, 4, 5, 6, 7, 8},
		Name:         trunceableName,
		StartTime:    ts,
		Status:       &v1.Status{Code: 1, Message: "all good"},
	}
}

func commonSplunkEvent(
	name string,
	ts *timestamp.Timestamp,
) *splunkEvent {
	id := model.ID(578437695752307201)
	parentID := model.ID(578437695752306946)
	span := model.SpanModel{
		SpanContext: model.SpanContext{
			TraceID: model.TraceID{
				High: uint64(578437695752307201),
				Low:  uint64(144964032628459529),
			},
			ID:       id,
			ParentID: &parentID,
			Debug:    false,
			Sampled:  nil,
			Err:      nil,
		},
		Name:           name,
		Kind:           model.Undetermined,
		Timestamp:      time.Unix(ts.GetSeconds(), int64(ts.GetNanos())),
		Duration:       0,
		Shared:         false,
		LocalEndpoint:  nil,
		RemoteEndpoint: nil,
		Annotations:    nil,
		Tags:           map[string]string{"ot.status_code": "\x01", "ot.status_description": "all good"},
	}
	return &splunkEvent{
		Time:  timestampToEpochMilliseconds(ts),
		Host:  "unknown",
		Event: span,
	}
}

func Test_convertKind(t *testing.T) {
	assert.Equal(t, model.Undetermined, convertKind(56))
	assert.Equal(t, model.Undetermined, convertKind(v1.Span_SPAN_KIND_UNSPECIFIED))
	assert.Equal(t, model.Client, convertKind(v1.Span_CLIENT))
	assert.Equal(t, model.Server, convertKind(v1.Span_SERVER))
}

func Test_convertAnnotations(t *testing.T) {
	ts := &timestamp.Timestamp{
		Nanos: 0,
	}
	assert.Equal(t,
		[]model.Annotation{{Timestamp: time.Unix(0, 0), Value: "foo"}, {Timestamp: time.Unix(0, 0), Value: "42"}},
		convertAnnotations(&v1.Span_TimeEvents{
			TimeEvent: []*v1.Span_TimeEvent{
				{
					Time: ts,
					Value: &v1.Span_TimeEvent_Annotation_{Annotation: &v1.Span_TimeEvent_Annotation{
						Description: &v1.TruncatableString{
							Value: "foo",
						},
						Attributes: &v1.Span_Attributes{
							AttributeMap: map[string]*v1.AttributeValue{
								"foobar": {
									Value: &v1.AttributeValue_StringValue{StringValue: &v1.TruncatableString{
										Value: "foo",
									}},
								},
							},
						},
					}},
				},
				{
					Time: ts,
					Value: &v1.Span_TimeEvent_MessageEvent_{MessageEvent: &v1.Span_TimeEvent_MessageEvent{
						Type: 1,
						Id:   42,
					}},
				},
			},
		}))
}

func Test_convertTags(t *testing.T) {
	result, err := convertTags(&v1.Span_Attributes{
		AttributeMap: map[string]*v1.AttributeValue{
			"foo": {
				Value: &v1.AttributeValue_StringValue{StringValue: &v1.TruncatableString{
					Value: "bar",
				}},
			},
			"foo1": {
				Value: &v1.AttributeValue_IntValue{IntValue: 13},
			},
			"foo2": {
				Value: &v1.AttributeValue_DoubleValue{DoubleValue: 13.33},
			},
			"foo3": {
				Value: &v1.AttributeValue_BoolValue{BoolValue: false},
			},
			"foo4": {
				Value: &v1.AttributeValue_BoolValue{BoolValue: true},
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"foo": "bar", "foo1": "13", "foo2": "13.330000", "foo3": "false", "foo4": "true"}, result)
}

func Test_convertBadTag(t *testing.T) {
	_, err := convertTags(&v1.Span_Attributes{
		AttributeMap: map[string]*v1.AttributeValue{
			"foo": {
				Value: nil,
			},
		},
	})
	assert.EqualError(t, err, "cannot convert tag value")
}
