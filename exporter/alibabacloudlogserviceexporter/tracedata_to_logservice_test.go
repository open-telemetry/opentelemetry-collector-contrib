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

package alibabacloudlogserviceexporter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"sort"
	"strings"
	"testing"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
)

type logKeyValuePair struct {
	Key   string
	Value string
}

type logKeyValuePairs []logKeyValuePair

func (kv logKeyValuePairs) Len() int           { return len(kv) }
func (kv logKeyValuePairs) Swap(i, j int)      { kv[i], kv[j] = kv[j], kv[i] }
func (kv logKeyValuePairs) Less(i, j int) bool { return kv[i].Key < kv[j].Key }

func TestNilOCProtoNodeToLogServiceData(t *testing.T) {
	nilNodeBatch := consumerdata.TraceData{
		Spans: []*tracepb.Span{
			{
				TraceId: []byte("0123456789abcdef"),
				SpanId:  []byte("01234567"),
			},
		},
	}
	got := traceDataToLogServiceData(nilNodeBatch)
	if len(got) == 0 {
		t.Fatalf("Logs count must > 0")
	}
	for _, log := range got {
		if len(log.Contents) == 0 {
			t.Fatalf("Log contents count must > 0")
		}
	}
}

func TestOCProtoToLogServiceData(t *testing.T) {
	const numOfFiles = 2
	for i := 0; i < numOfFiles; i++ {
		td := tds[i]

		gotLogs := traceDataToLogServiceData(td)

		gotLogPairs := make([][]logKeyValuePair, 0, len(gotLogs))

		for _, log := range gotLogs {
			pairs := make([]logKeyValuePair, 0, len(log.Contents))
			for _, content := range log.Contents {
				pairs = append(pairs, logKeyValuePair{
					Key:   content.GetKey(),
					Value: content.GetValue(),
				})
				//fmt.Printf("%s : %s\n", content.GetKey(), content.GetValue())
			}
			gotLogPairs = append(gotLogPairs, pairs)

			//fmt.Println("#################")
		}
		//str, _ := json.Marshal(gotLogPairs)
		//fmt.Println(string(str))

		wantSpanCount, gotSpanCount := len(td.Spans), len(gotLogs)
		if wantSpanCount != gotSpanCount {
			t.Errorf("Different number of spans in the batches on pass #%d (want %d, got %d)", i, wantSpanCount, gotSpanCount)
			continue
		}

		resultLogFile := fmt.Sprintf("./testdata/logservice_trace_data_no_binary_tags_%02d.json", i+1)

		wantLogs := make([][]logKeyValuePair, 0, wantSpanCount)

		if err := loadFromJSON(resultLogFile, &wantLogs); err != nil {
			t.Errorf("Failed load log key value pairs from %q: %v", resultLogFile, err)
			continue
		}

		for j := 0; j < wantSpanCount; j++ {

			sort.Sort(logKeyValuePairs(gotLogPairs[j]))
			sort.Sort(logKeyValuePairs(wantLogs[j]))
			if !reflect.DeepEqual(gotLogPairs[j], wantLogs[j]) {
				t.Errorf("Unsuccessful conversion %d \nGot:\n\t%v\nWant:\n\t%v", i, gotLogPairs, wantLogs)
			}
		}
	}
}

func TestOCStatusToJaegerThriftTags(t *testing.T) {

	type test struct {
		haveAttributes *tracepb.Span_Attributes
		haveStatus     *tracepb.Status
		wantTags       []logKeyValuePair
	}

	cases := []test{
		// only status.code
		{
			haveAttributes: nil,
			haveStatus: &tracepb.Status{
				Code: 10,
			},
			wantTags: []logKeyValuePair{
				{
					Key:   tagsPrefix + tracetranslator.TagStatusCode,
					Value: "10",
				},
				{
					Key:   tagsPrefix + tracetranslator.TagStatusMsg,
					Value: "",
				},
			},
		},
		// only status.message
		{
			haveAttributes: nil,
			haveStatus: &tracepb.Status{
				Message: "Message",
			},
			wantTags: []logKeyValuePair{
				{
					Key:   tagsPrefix + tracetranslator.TagStatusCode,
					Value: "0",
				},
				{
					Key:   tagsPrefix + tracetranslator.TagStatusMsg,
					Value: "Message",
				},
			},
		},
		// both status.code and status.message
		{
			haveAttributes: nil,
			haveStatus: &tracepb.Status{
				Code:    12,
				Message: "Forbidden",
			},
			wantTags: []logKeyValuePair{
				{
					Key:   tagsPrefix + tracetranslator.TagStatusCode,
					Value: "12",
				},
				{
					Key:   tagsPrefix + tracetranslator.TagStatusMsg,
					Value: "Forbidden",
				},
			},
		},

		// status and existing tags
		{
			haveStatus: &tracepb.Status{
				Code:    404,
				Message: "NotFound",
			},
			haveAttributes: &tracepb.Span_Attributes{
				AttributeMap: map[string]*tracepb.AttributeValue{
					"status.code": {
						Value: &tracepb.AttributeValue_IntValue{
							IntValue: 13,
						},
					},
					"status.message": {
						Value: &tracepb.AttributeValue_StringValue{
							StringValue: &tracepb.TruncatableString{Value: "Error"},
						},
					},
				},
			},
			wantTags: []logKeyValuePair{
				{
					Key:   tagsPrefix + tracetranslator.TagStatusCode,
					Value: "13",
				},
				{
					Key:   tagsPrefix + tracetranslator.TagStatusMsg,
					Value: "Error",
				},
			},
		},

		// partial existing tag

		{
			haveStatus: &tracepb.Status{
				Code:    404,
				Message: "NotFound",
			},
			haveAttributes: &tracepb.Span_Attributes{
				AttributeMap: map[string]*tracepb.AttributeValue{
					"status.code": {
						Value: &tracepb.AttributeValue_IntValue{
							IntValue: 13,
						},
					},
				},
			},
			wantTags: []logKeyValuePair{
				{
					Key:   tagsPrefix + tracetranslator.TagStatusCode,
					Value: "13",
				},
			},
		},

		{
			haveStatus: &tracepb.Status{
				Code:    404,
				Message: "NotFound",
			},
			haveAttributes: &tracepb.Span_Attributes{
				AttributeMap: map[string]*tracepb.AttributeValue{
					"status.message": {
						Value: &tracepb.AttributeValue_StringValue{
							StringValue: &tracepb.TruncatableString{Value: "Error"},
						},
					},
				},
			},
			wantTags: []logKeyValuePair{
				{
					Key:   tagsPrefix + tracetranslator.TagStatusMsg,
					Value: "Error",
				},
			},
		},
		// both status and tags
		{
			haveStatus: &tracepb.Status{
				Code:    13,
				Message: "Forbidden",
			},
			haveAttributes: &tracepb.Span_Attributes{
				AttributeMap: map[string]*tracepb.AttributeValue{
					"http.status_code": {
						Value: &tracepb.AttributeValue_IntValue{
							IntValue: 404,
						},
					},
					"http.status_message": {
						Value: &tracepb.AttributeValue_StringValue{
							StringValue: &tracepb.TruncatableString{Value: "NotFound"},
						},
					},
				},
			},
			wantTags: []logKeyValuePair{
				{
					Key:   tagsPrefix + tracetranslator.TagHTTPStatusCode,
					Value: "404",
				},
				{
					Key:   tagsPrefix + tracetranslator.TagHTTPStatusMsg,
					Value: "NotFound",
				},
				{
					Key:   tagsPrefix + tracetranslator.TagStatusCode,
					Value: "13",
				},
				{
					Key:   tagsPrefix + tracetranslator.TagStatusMsg,
					Value: "Forbidden",
				},
			},
		},
	}

	fakeTraceID := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	fakeSpanID := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	for i, c := range cases {
		gotLogs := traceDataToLogServiceData(consumerdata.TraceData{
			Spans: []*tracepb.Span{{
				TraceId:    fakeTraceID,
				SpanId:     fakeSpanID,
				Status:     c.haveStatus,
				Attributes: c.haveAttributes,
			}},
		})
		gotLog := gotLogs[0]

		var gotPairs []logKeyValuePair
		for _, content := range gotLog.Contents {
			if strings.HasPrefix(content.GetKey(), tagsPrefix) && strings.Contains(content.GetKey(), "status") {
				gotPairs = append(gotPairs, logKeyValuePair{
					Key:   content.GetKey(),
					Value: content.GetValue(),
				})
			}
		}
		sort.Sort(logKeyValuePairs(gotPairs))
		sort.Sort(logKeyValuePairs(c.wantTags))
		if !reflect.DeepEqual(gotPairs, c.wantTags) {
			t.Errorf("Unsuccessful conversion : %d \nGot:\n\t%v\nWant:\n\t%v", i, gotPairs, c.wantTags)
		}
	}
}

// tds has the TraceData proto used in the test. They are hard coded because
// structs like tracepb.AttributeMap cannot be ready from JSON.
var tds = []consumerdata.TraceData{
	{
		Node: &commonpb.Node{
			Identifier: &commonpb.ProcessIdentifier{
				HostName:       "api246-sjc1",
				Pid:            13,
				StartTimestamp: &timestamp.Timestamp{Seconds: 1485467190, Nanos: 639875000},
			},
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "someVersion"},
			ServiceInfo: &commonpb.ServiceInfo{Name: "api"},
			Attributes: map[string]string{
				"a.binary": "AQIDBAMCAQ==",
				"a.bool":   "true",
				"a.double": "1234.56789",
				"a.long":   "123456789",
				"ip":       "10.53.69.61",
			},
		},
		Resource: &resourcepb.Resource{
			Type:   "k8s.io/container",
			Labels: map[string]string{"resource_key1": "resource_val1"},
		},
		Spans: []*tracepb.Span{
			{
				TraceId:      []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x52, 0x96, 0x9A, 0x89, 0x55, 0x57, 0x1A, 0x3F},
				SpanId:       []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x7D, 0x98},
				ParentSpanId: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x68, 0xC4, 0xE3},
				Name:         &tracepb.TruncatableString{Value: "get"},
				Kind:         tracepb.Span_CLIENT,
				StartTime:    &timestamp.Timestamp{Seconds: 1485467191, Nanos: 639875000},
				EndTime:      &timestamp.Timestamp{Seconds: 1485467191, Nanos: 662813000},
				Attributes: &tracepb.Span_Attributes{
					AttributeMap: map[string]*tracepb.AttributeValue{
						"http.url": {
							Value: &tracepb.AttributeValue_StringValue{StringValue: &tracepb.TruncatableString{Value: "http://localhost:15598/client_transactions"}},
						},
						"peer.ipv4": {
							Value: &tracepb.AttributeValue_IntValue{IntValue: 3224716605},
						},
						"peer.port": {
							Value: &tracepb.AttributeValue_IntValue{IntValue: 53931},
						},
						"peer.service": {
							Value: &tracepb.AttributeValue_StringValue{StringValue: &tracepb.TruncatableString{Value: "rtapi"}},
						},
						"someBool": {
							Value: &tracepb.AttributeValue_BoolValue{BoolValue: true},
						},
						"someDouble": {
							Value: &tracepb.AttributeValue_DoubleValue{DoubleValue: 129.8},
						},
						"span.kind": {
							Value: &tracepb.AttributeValue_StringValue{StringValue: &tracepb.TruncatableString{Value: "client"}},
						},
					},
				},
				TimeEvents: &tracepb.Span_TimeEvents{
					TimeEvent: []*tracepb.Span_TimeEvent{
						{
							Time: &timestamp.Timestamp{Seconds: 1485467191, Nanos: 639874000},
							Value: &tracepb.Span_TimeEvent_MessageEvent_{
								MessageEvent: &tracepb.Span_TimeEvent_MessageEvent{
									Type: tracepb.Span_TimeEvent_MessageEvent_SENT, UncompressedSize: 1024, CompressedSize: 512,
								},
							},
						},
						{
							Time: &timestamp.Timestamp{Seconds: 1485467191, Nanos: 639875000},
							Value: &tracepb.Span_TimeEvent_Annotation_{
								Annotation: &tracepb.Span_TimeEvent_Annotation{
									Attributes: &tracepb.Span_Attributes{
										AttributeMap: map[string]*tracepb.AttributeValue{
											"key1": {
												Value: &tracepb.AttributeValue_StringValue{StringValue: &tracepb.TruncatableString{Value: "value1"}},
											},
										},
									},
								},
							},
						},
						{
							Time: &timestamp.Timestamp{Seconds: 1485467191, Nanos: 639875000},
							Value: &tracepb.Span_TimeEvent_Annotation_{
								Annotation: &tracepb.Span_TimeEvent_Annotation{
									Description: &tracepb.TruncatableString{Value: "annotation description"},
									Attributes: &tracepb.Span_Attributes{
										AttributeMap: map[string]*tracepb.AttributeValue{
											"event": {
												Value: &tracepb.AttributeValue_StringValue{StringValue: &tracepb.TruncatableString{Value: "nothing"}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	},
	{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "api"},
		},
		Spans: []*tracepb.Span{
			{
				TraceId:      []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x52, 0x96, 0x9A, 0x89, 0x55, 0x57, 0x1A, 0x3F},
				SpanId:       []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x7D, 0x98},
				ParentSpanId: nil,
				Name:         &tracepb.TruncatableString{Value: "get"},
				Kind:         tracepb.Span_SERVER,
				StartTime:    &timestamp.Timestamp{Seconds: 1485467191, Nanos: 639875000},
				EndTime:      &timestamp.Timestamp{Seconds: 1485467191, Nanos: 662813000},
				Attributes: &tracepb.Span_Attributes{
					AttributeMap: map[string]*tracepb.AttributeValue{
						"peer.service": {
							Value: &tracepb.AttributeValue_StringValue{StringValue: &tracepb.TruncatableString{Value: "AAAAAAAAMDk="}},
						},
					},
				},
			},
			{
				TraceId:      []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x52, 0x96, 0x9A, 0x89, 0x55, 0x57, 0x1A, 0x3F},
				SpanId:       []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x7D, 0x99},
				ParentSpanId: []byte{},
				Name:         &tracepb.TruncatableString{Value: "get"},
				Kind:         tracepb.Span_SERVER,
				StartTime:    &timestamp.Timestamp{Seconds: 1485467191, Nanos: 639875000},
				EndTime:      &timestamp.Timestamp{Seconds: 1485467191, Nanos: 662813000},
				Links: &tracepb.Span_Links{
					Link: []*tracepb.Span_Link{
						{
							TraceId: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x52, 0x96, 0x9A, 0x89, 0x55, 0x57, 0x1A, 0x3F},
							SpanId:  []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x7D, 0x98},
							Type:    tracepb.Span_Link_PARENT_LINKED_SPAN,
						},
						{
							TraceId: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x52, 0x96, 0x9A, 0x89, 0x55, 0x57, 0x1A, 0x3F},
							SpanId:  []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x68, 0xC4, 0xE3},
						},
						{
							TraceId: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
							SpanId:  []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
						},
					},
				},
			},
			{
				TraceId:      []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x52, 0x96, 0x9A, 0x89, 0x55, 0x57, 0x1A, 0x3F},
				SpanId:       []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x7D, 0x98},
				ParentSpanId: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				Name:         &tracepb.TruncatableString{Value: "get2"},
				StartTime:    &timestamp.Timestamp{Seconds: 1485467192, Nanos: 639875000},
				EndTime:      &timestamp.Timestamp{Seconds: 1485467192, Nanos: 662813000},
			},
		},
	},
}

func loadFromJSON(file string, obj interface{}) error {
	blob, err := ioutil.ReadFile(file)
	if err == nil {
		err = json.Unmarshal(blob, obj)
	}

	return err
}

func TestTraceCornerCases(t *testing.T) {
	assert.Equal(t, truncableStringToStr(nil), "")
	assert.Equal(t, timestampToEpochMicroseconds(nil), int64(0))
	assert.Nil(t, ocMessageEventToMap(nil))
	msgEvent := &tracepb.Span_TimeEvent_MessageEvent{}
	assert.NotNil(t, ocMessageEventToMap(msgEvent))
	msgEvent.UncompressedSize = 1
	msgEvent.CompressedSize = 1
	assert.NotNil(t, ocMessageEventToMap(msgEvent))
	annotation := &tracepb.Span_TimeEvent_Annotation{}
	assert.Nil(t, ocAnnotationToMap(nil))
	assert.NotNil(t, ocAnnotationToMap(annotation))
	annotation.Description = &tracepb.TruncatableString{
		Value: "testDesc",
	}
	assert.NotNil(t, ocAnnotationToMap(annotation))

	assert.Nil(t, timeEventToLogContent(nil))

	assert.NotNil(t, timeEventToLogContent(&tracepb.Span_TimeEvents{
		TimeEvent: []*tracepb.Span_TimeEvent{
			{
				Value: nil,
			},
		},
	}))

	assert.Equal(t, spanKindToStr(tracepb.Span_SpanKind(99)), "")
	assert.Equal(t, spanKindToStr(tracepb.Span_CLIENT), string(tracetranslator.OpenTracingSpanKindClient))
	assert.Equal(t, spanKindToStr(tracepb.Span_SERVER), string(tracetranslator.OpenTracingSpanKindServer))
	assert.Equal(t, attributeValueToString(&tracepb.AttributeValue{}), "<Unknown OpenCensus Attribute>")
	assert.Equal(t, attributeValueToString(&tracepb.AttributeValue{
		Value: &tracepb.AttributeValue_StringValue{
			StringValue: annotation.Description,
		},
	}), "testDesc")
	assert.Equal(t, attributeValueToString(&tracepb.AttributeValue{
		Value: &tracepb.AttributeValue_BoolValue{
			BoolValue: true,
		},
	}), "true")
	assert.Equal(t, attributeValueToString(&tracepb.AttributeValue{
		Value: &tracepb.AttributeValue_BoolValue{
			BoolValue: false,
		},
	}), "false")
	assert.Equal(t, attributeValueToString(&tracepb.AttributeValue{
		Value: &tracepb.AttributeValue_IntValue{
			IntValue: 1,
		},
	}), "1")
	assert.Equal(t, attributeValueToString(&tracepb.AttributeValue{
		Value: &tracepb.AttributeValue_DoubleValue{
			DoubleValue: 1,
		},
	}), "1")
	rst := linksToLogContents(nil)
	assert.Nil(t, rst)
	rstLogs := spansToLogServiceData(nil)
	assert.Nil(t, rstLogs)
	rstLogContents := nodeAndResourceToLogContent(nil, nil)
	assert.Equal(t, len(rstLogContents), 0)

	rstLogContents = nodeAndResourceToLogContent(&commonpb.Node{}, &resourcepb.Resource{})
	assert.Equal(t, len(rstLogContents), 0)

	rstLogContents = nodeAndResourceToLogContent(&commonpb.Node{
		LibraryInfo: &commonpb.LibraryInfo{
			Language:           commonpb.LibraryInfo_GO_LANG,
			ExporterVersion:    "v_1.0",
			CoreLibraryVersion: "v0.1",
		},
	}, &resourcepb.Resource{})
	assert.Equal(t, len(rstLogContents), 4)
}
