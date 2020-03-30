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
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/go-cmp/cmp"
	"github.com/klauspost/compress/zstd"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type honeycombData struct {
	Data map[string]interface{} `json:"data"`
}

func TestExporter(t *testing.T) {
	var got []honeycombData

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		uncompressed, err := zstd.NewReader(req.Body)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			return
		}
		defer req.Body.Close()
		b, err := ioutil.ReadAll(uncompressed)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			return
		}

		var data []honeycombData
		err = json.Unmarshal(b, &data)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			return
		}

		got = append(got, data...)

		rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	cfg := Config{
		APIKey:  "test",
		Dataset: "test",
		APIURL:  server.URL,
	}

	logger := zap.NewNop()
	factory := Factory{}
	exporter, err := factory.CreateTraceExporter(logger, &cfg)
	require.NoError(t, err)

	td := consumerdata.TraceData{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test_service"},
			Attributes: map[string]string{
				"A": "B",
			},
		},
		Spans: []*tracepb.Span{
			{
				TraceId:                 []byte{0x01},
				SpanId:                  []byte{0x02},
				Name:                    &tracepb.TruncatableString{Value: "root"},
				Kind:                    tracepb.Span_SERVER,
				SameProcessAsParentSpan: &wrappers.BoolValue{Value: true},
			},
			{
				TraceId:                 []byte{0x01},
				SpanId:                  []byte{0x03},
				ParentSpanId:            []byte{0x02},
				Name:                    &tracepb.TruncatableString{Value: "client"},
				Kind:                    tracepb.Span_CLIENT,
				SameProcessAsParentSpan: &wrappers.BoolValue{Value: true},
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

	ctx := context.Background()
	err = exporter.ConsumeTraceData(ctx, td)
	require.NoError(t, err)
	exporter.Shutdown()

	want := []honeycombData{
		{
			Data: map[string]interface{}{
				"duration_ms":       float64(0),
				"has_remote_parent": false,
				"name":              "root",
				"service_name":      "test_service",
				"status.code":       float64(0),
				"status.message":    "OK",
				"trace.span_id":     "0200000000000000",
				"trace.trace_id":    "01000000-0000-0000-0000-000000000000",
				"A":                 "B",
			},
		},
		{
			Data: map[string]interface{}{
				"duration_ms":       float64(0),
				"has_remote_parent": false,
				"name":              "client",
				"service_name":      "test_service",
				"status.code":       float64(0),
				"status.message":    "OK",
				"trace.parent_id":   "0200000000000000",
				"trace.span_id":     "0300000000000000",
				"trace.trace_id":    "01000000-0000-0000-0000-000000000000",
			},
		},
		{
			Data: map[string]interface{}{
				"duration_ms":       float64(0),
				"has_remote_parent": true,
				"name":              "server",
				"service_name":      "test_service",
				"status.code":       float64(0),
				"status.message":    "OK",
				"trace.parent_id":   "0300000000000000",
				"trace.span_id":     "0400000000000000",
				"trace.trace_id":    "01000000-0000-0000-0000-000000000000",
				"A":                 "B",
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("otel span: (-want +got):\n%s", diff)
	}
}
