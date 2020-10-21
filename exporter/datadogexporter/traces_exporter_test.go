// Copyright The OpenTelemetry Authors
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

// +build !windows

package datadogexporter

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/DataDog/datadog-agent/pkg/trace/stats"
	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func testTraceExporterHelper(td pdata.Traces, t *testing.T) []string {
	var got []string
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", req.Header.Get("DD-Api-Key"))

		contentType := req.Header.Get("Content-Type")

		data := []string{contentType}
		got = append(got, data...)

		if contentType == "application/x-protobuf" {
			testProtobufTracePayload(t, rw, req)
		} else if contentType == "application/json" {
			testJSONTraceStatsPayload(t, rw, req)
		}
		rw.WriteHeader(http.StatusAccepted)
	}))

	defer server.Close()
	cfg := Config{
		API: APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: TagsConfig{
			Hostname: "test_host",
			Env:      "test_env",
			Tags:     []string{"key:val"},
		},
		Traces: TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
		},
	}

	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	exporter, err := createTraceExporter(context.Background(), params, &cfg)

	assert.NoError(t, err)

	defer exporter.Shutdown(context.Background())

	ctx := context.Background()
	errConsume := exporter.ConsumeTraces(ctx, td)
	assert.NoError(t, errConsume)

	return got
}

func testProtobufTracePayload(t *testing.T, rw http.ResponseWriter, req *http.Request) {
	var traceData pb.TracePayload
	b, err := ioutil.ReadAll(req.Body)

	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		assert.NoError(t, err, "http server received malformed trace payload")
		return
	}

	defer req.Body.Close()

	if marshallErr := proto.Unmarshal(b, &traceData); marshallErr != nil {
		http.Error(rw, marshallErr.Error(), http.StatusInternalServerError)
		assert.NoError(t, marshallErr, "http server received malformed trace payload")
		return
	}

	assert.NotNil(t, traceData.Env)
	assert.NotNil(t, traceData.HostName)
	assert.NotNil(t, traceData.Traces)
}

func testJSONTraceStatsPayload(t *testing.T, rw http.ResponseWriter, req *http.Request) {
	var statsData stats.Payload

	gz, err := gzip.NewReader(req.Body)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		require.NoError(t, err, "http server received malformed stats payload")
		return
	}

	defer req.Body.Close()
	defer gz.Close()

	statsBytes, err := ioutil.ReadAll(gz)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		require.NoError(t, err, "http server received malformed stats payload")
		return
	}

	if marshallErr := json.Unmarshal(statsBytes, &statsData); marshallErr != nil {
		http.Error(rw, marshallErr.Error(), http.StatusInternalServerError)
		require.NoError(t, marshallErr, "http server received malformed stats payload")
		return
	}

	assert.NotNil(t, statsData.Env)
	assert.NotNil(t, statsData.HostName)
	assert.NotNil(t, statsData.Stats)
}

func TestNewTraceExporter(t *testing.T) {
	cfg := &Config{}
	cfg.API.Key = "ddog_32_characters_long_api_key1"
	logger := zap.NewNop()

	// The client should have been created correctly
	exp, err := newTraceExporter(logger, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestPushTraceData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", req.Header.Get("DD-Api-Key"))
		rw.WriteHeader(http.StatusAccepted)
	}))

	defer server.Close()
	cfg := &Config{
		API: APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		TagsConfig: TagsConfig{
			Hostname: "test_host",
			Env:      "test_env",
			Tags:     []string{"key:val"},
		},
		Traces: TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
		},
	}
	logger := zap.NewNop()

	exp, err := newTraceExporter(logger, cfg)

	assert.NoError(t, err)

	tracesLength, err := exp.pushTraceData(context.Background(), func() pdata.Traces {
		traces := pdata.NewTraces()
		resourceSpans := traces.ResourceSpans()
		resourceSpans.Resize(1)
		resourceSpans.At(0).InitEmpty()
		resourceSpans.At(0).InstrumentationLibrarySpans().Resize(1)
		resourceSpans.At(0).InstrumentationLibrarySpans().At(0).Spans().Resize(1)
		return traces
	}())

	assert.Nil(t, err)
	assert.Equal(t, 1, tracesLength)

}

func TestTraceAndStatsExporter(t *testing.T) {
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
				SameProcessAsParentSpan: &wrapperspb.BoolValue{Value: true},
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
							Time: &timestamppb.Timestamp{
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
				SameProcessAsParentSpan: &wrapperspb.BoolValue{Value: true},
				Links: &tracepb.Span_Links{
					Link: []*tracepb.Span_Link{
						{
							TraceId: []byte{0x04},
							SpanId:  []byte{0x05},
							Type:    tracepb.Span_Link_PARENT_LINKED_SPAN,
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
				SameProcessAsParentSpan: &wrapperspb.BoolValue{Value: false},
			},
		},
	}

	// ensure that the protobuf serialized traces payload contains HostName Env and Traces
	// ensure that the json gzipped stats payload contains HostName Env and Stats
	got := testTraceExporterHelper(internaldata.OCToTraceData(td), t)

	// ensure a protobuf and json payload are sent
	assert.Equal(t, 2, len(got))
	assert.Equal(t, "application/json", got[1])
	assert.Equal(t, "application/x-protobuf", got[0])
}
