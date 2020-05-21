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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

func testingServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte(`OK`))
	}))
}

func testTraceExporter(td consumerdata.TraceData, t *testing.T) {
	server := testingServer()

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

	logger := zap.NewNop()
	factory := Factory{}
	exporter, err := factory.CreateTraceExporter(logger, &cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.ConsumeTraceData(ctx, td)
	require.NoError(t, err)
	exporter.Shutdown(ctx)

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

	testTraceExporter(td, t)
}
