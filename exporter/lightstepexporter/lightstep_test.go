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

package lightstepexporter

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func testingServer(callback func(data []byte)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		defer req.Body.Close()
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		callback(b)
		rw.Write([]byte(`OK`))
	}))
}

func testTraceExporter(td pdata.Traces, t *testing.T) []byte {
	responseLock := sync.Mutex{}

	response := []byte{}
	server := testingServer(func(data []byte) {
		responseLock.Lock()
		response = append(response, data...)
		responseLock.Unlock()
	})

	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())

	defer server.Close()
	cfg := Config{
		AccessToken:   "test",
		SatelliteHost: u.Hostname(),
		SatellitePort: port,
		ServiceName:   "test",
		PlainText:     true,
	}

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, err := createTraceExporter(context.Background(), params, &cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.ConsumeTraces(ctx, td)
	require.NoError(t, err)
	exporter.Shutdown(ctx)

	responseLock.Lock()
	defer responseLock.Unlock()
	return response
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
				SameProcessAsParentSpan: &wrapperspb.BoolValue{Value: true},
			},
		},
	}

	testTraceExporter(internaldata.OCToTraceData(td), t)
}

func TestPushTraceData(t *testing.T) {
	td := consumerdata.TraceData{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{
				Name: "serviceABC",
			},
			Identifier: &commonpb.ProcessIdentifier{
				HostName: "hostname123",
			},
		},
		Spans: []*tracepb.Span{
			{
				TraceId:                 []byte{0x01},
				SpanId:                  []byte{0x02},
				Name:                    &tracepb.TruncatableString{Value: "rootSpan"},
				Kind:                    tracepb.Span_SERVER,
				SameProcessAsParentSpan: &wrapperspb.BoolValue{Value: true},
			},
		},
	}
	response := testTraceExporter(internaldata.OCToTraceData(td), t)
	assert.Contains(t, string(response), "serviceABC")
	assert.Contains(t, string(response), "hostname123")
	assert.Contains(t, string(response), "rootSpan")
}
