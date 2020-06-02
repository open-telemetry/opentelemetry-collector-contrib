// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package honeycombexporter

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/go-cmp/cmp"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

type honeycombData struct {
	Data map[string]interface{} `json:"data"`
}

func testingServer(callback func(data []honeycombData)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		uncompressed, err := zstd.NewReader(req.Body)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		defer req.Body.Close()
		b, err := ioutil.ReadAll(uncompressed)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		var data []honeycombData
		err = json.Unmarshal(b, &data)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		callback(data)
		rw.Write([]byte(`OK`))
	}))
}

func testTraceExporter(td consumerdata.TraceData, t *testing.T) []honeycombData {
	var got []honeycombData
	server := testingServer(func(data []honeycombData) {
		got = append(got, data...)
	})
	defer server.Close()
	cfg := Config{
		APIKey:     "test",
		Dataset:    "test",
		APIURL:     server.URL,
		Debug:      false,
		SampleRate: 1,
	}

	logger := zap.NewNop()
	factory := Factory{}
	exporter, err := factory.CreateTraceExporter(logger, &cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.ConsumeTraceData(ctx, td)
	require.NoError(t, err)
	exporter.Shutdown(context.Background())

	return got
}

func TestExporter(t *testing.T) {
	td := consumerdata.TraceData{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test_service"},
			Attributes: map[string]string{
				"A": "B",
			},
		},
		Resource: &resourcepb.Resource{
			Type: "foobar",
			Labels: map[string]string{
				"B": "C",
			},
		},
		Spans: []*tracepb.Span{
			{
				TraceId:                 []byte{0x01},
				SpanId:                  []byte{0x02},
				Name:                    &tracepb.TruncatableString{Value: "root"},
				Kind:                    tracepb.Span_SERVER,
				SameProcessAsParentSpan: &wrappers.BoolValue{Value: true},
				Attributes: &tracepb.Span_Attributes{
					AttributeMap: map[string]*tracepb.AttributeValue{
						"span_attr_name": {
							Value: &tracepb.AttributeValue_StringValue{
								StringValue: &tracepb.TruncatableString{Value: "Span Attribute"},
							},
						},
					},
				},
				Resource: &resourcepb.Resource{
					Type: "override",
					Labels: map[string]string{
						"B": "D",
					},
				},
				TimeEvents: &tracepb.Span_TimeEvents{
					TimeEvent: []*tracepb.Span_TimeEvent{
						{
							Time: &timestamp.Timestamp{
								Seconds: 0,
								Nanos:   0,
							},
							Value: &tracepb.Span_TimeEvent_Annotation_{
								Annotation: &tracepb.Span_TimeEvent_Annotation{
									Description: &tracepb.TruncatableString{Value: "Some Description"},
									Attributes: &tracepb.Span_Attributes{
										AttributeMap: map[string]*tracepb.AttributeValue{
											"attribute_name": {
												Value: &tracepb.AttributeValue_StringValue{
													StringValue: &tracepb.TruncatableString{Value: "Hello MessageEvent"},
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
				TraceId:                 []byte{0x01},
				SpanId:                  []byte{0x03},
				ParentSpanId:            []byte{0x02},
				Name:                    &tracepb.TruncatableString{Value: "client"},
				Kind:                    tracepb.Span_CLIENT,
				SameProcessAsParentSpan: &wrappers.BoolValue{Value: true},
				Links: &tracepb.Span_Links{
					Link: []*tracepb.Span_Link{
						{
							TraceId: []byte{0x04},
							SpanId:  []byte{0x05},
							Type:    tracepb.Span_Link_CHILD_LINKED_SPAN,
							Attributes: &tracepb.Span_Attributes{
								AttributeMap: map[string]*tracepb.AttributeValue{
									"span_link_attr": {
										Value: &tracepb.AttributeValue_IntValue{
											IntValue: 12345,
										},
									},
								},
							},
						},
					},
					DroppedLinksCount: 0,
				},
			},
			{
				TraceId:                 []byte{0x01},
				SpanId:                  []byte{0x04},
				ParentSpanId:            []byte{0x03},
				Name:                    &tracepb.TruncatableString{Value: "server"},
				Kind:                    tracepb.Span_SERVER,
				SameProcessAsParentSpan: &wrappers.BoolValue{Value: false},
			},
		},
	}
	got := testTraceExporter(td, t)
	want := []honeycombData{
		{
			Data: map[string]interface{}{
				"A":                       "B",
				"B":                       "D",
				"attribute_name":          "Hello MessageEvent",
				"meta.span_type":          "span_event",
				"name":                    "Some Description",
				"opencensus.resourcetype": "foobar",
				"resource_type":           "override",
				"service_name":            "test_service",
				"trace.parent_id":         "02",
				"trace.parent_name":       "root",
				"trace.trace_id":          "01",
			},
		},
		{
			Data: map[string]interface{}{
				"duration_ms":             float64(0),
				"has_remote_parent":       false,
				"name":                    "root",
				"resource_type":           "override",
				"service_name":            "test_service",
				"span_attr_name":          "Span Attribute",
				"status.code":             float64(0),
				"status.message":          "OK",
				"trace.span_id":           "02",
				"trace.trace_id":          "01",
				"A":                       "B",
				"B":                       "D",
				"opencensus.resourcetype": "foobar",
			},
		},
		{
			Data: map[string]interface{}{
				"meta.span_type":      "link",
				"ref_type":            float64(1),
				"span_link_attr":      float64(12345),
				"trace.trace_id":      "01",
				"trace.parent_id":     "03",
				"trace.link.span_id":  "05",
				"trace.link.trace_id": "04",
			},
		},
		{
			Data: map[string]interface{}{
				"duration_ms":             float64(0),
				"has_remote_parent":       false,
				"name":                    "client",
				"service_name":            "test_service",
				"status.code":             float64(0),
				"status.message":          "OK",
				"trace.parent_id":         "02",
				"trace.span_id":           "03",
				"trace.trace_id":          "01",
				"opencensus.resourcetype": "foobar",
				"B":                       "C",
			},
		},
		{
			Data: map[string]interface{}{
				"duration_ms":             float64(0),
				"has_remote_parent":       true,
				"name":                    "server",
				"service_name":            "test_service",
				"status.code":             float64(0),
				"status.message":          "OK",
				"trace.parent_id":         "03",
				"trace.span_id":           "04",
				"trace.trace_id":          "01",
				"A":                       "B",
				"B":                       "C",
				"opencensus.resourcetype": "foobar",
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("otel span: (-want +got):\n%s", diff)
	}
}

func TestEmptyNode(t *testing.T) {
	td := consumerdata.TraceData{
		Node: nil,
		Spans: []*tracepb.Span{
			{
				TraceId:                 []byte{0x01},
				SpanId:                  []byte{0x02},
				Name:                    &tracepb.TruncatableString{Value: "root"},
				Kind:                    tracepb.Span_SERVER,
				SameProcessAsParentSpan: &wrappers.BoolValue{Value: true},
			},
		},
	}

	got := testTraceExporter(td, t)

	want := []honeycombData{
		{
			Data: map[string]interface{}{
				"duration_ms":       float64(0),
				"has_remote_parent": false,
				"name":              "root",
				"status.code":       float64(0),
				"status.message":    "OK",
				"trace.span_id":     "02",
				"trace.trace_id":    "01",
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("otel span: (-want +got):\n%s", diff)
	}
}
