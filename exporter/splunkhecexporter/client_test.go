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
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

type CapturingData struct {
	receivedRequest chan string
}

func (c *CapturingData) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	go func() {
		c.receivedRequest <- string(body)
	}()
	w.WriteHeader(200)

}

func TestReceiveTraces(t *testing.T) {
	receivedRequest := make(chan string)
	capture := CapturingData{receivedRequest: receivedRequest}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	s := &http.Server{
		Handler: &capture,
	}
	go func() {
		panic(s.Serve(listener))
	}()

	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	cfg.DisableCompression = true
	cfg.Token = "1234-1234"

	assert.Equal(t, configmodels.Type(typeStr), factory.Type())
	exporter, err := factory.CreateTraceExporter(zap.NewNop(), cfg)
	assert.NoError(t, err)

	td := consumerdata.TraceData{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-service"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"resource": "R1",
			},
		},
		Spans: []*tracepb.Span{
			{
				TraceId:   []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:    []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:      &tracepb.TruncatableString{Value: "root"},
				Status:    &tracepb.Status{},
				StartTime: &timestamp.Timestamp{Seconds: 36},
			},
			{
				TraceId:      []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:       []byte{0, 0, 0, 0, 0, 0, 0, 2},
				ParentSpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:         &tracepb.TruncatableString{Value: "client"},
				Status:       &tracepb.Status{},
				StartTime:    &timestamp.Timestamp{Seconds: 37},
			},
			{
				TraceId:      []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:       []byte{0, 0, 0, 0, 0, 0, 0, 3},
				ParentSpanId: []byte{0, 0, 0, 0, 0, 0, 0, 2},
				Name:         &tracepb.TruncatableString{Value: "server"},
				Status:       &tracepb.Status{},
				StartTime:    &timestamp.Timestamp{Seconds: 38},
			},
		},
	}

	expected := `{"time":36000,"host":"unknown","event":{"trace_id":"AQEBAQEBAQEBAQEBAQEBAQ==","span_id":"AAAAAAAAAAE=","name":{"value":"root"},"start_time":{"seconds":36},"status":{}}}`
	expected += "\n\r\n\r\n"
	expected += `{"time":37000,"host":"unknown","event":{"trace_id":"AQEBAQEBAQEBAQEBAQEBAQ==","span_id":"AAAAAAAAAAI=","parent_span_id":"AAAAAAAAAAE=","name":{"value":"client"},"start_time":{"seconds":37},"status":{}}}`
	expected += "\n\r\n\r\n"
	expected += `{"time":38000,"host":"unknown","event":{"trace_id":"AQEBAQEBAQEBAQEBAQEBAQ==","span_id":"AAAAAAAAAAM=","parent_span_id":"AAAAAAAAAAI=","name":{"value":"server"},"start_time":{"seconds":38},"status":{}}}`
	expected += "\n\r\n\r\n"

	err = exporter.ConsumeTraceData(context.Background(), td)
	assert.NoError(t, err)
	select {
	case request := <-receivedRequest:
		assert.Equal(t, expected, request)
	case <-time.After(5 * time.Second):
		t.Fatal("Should have received request")
	}
}
