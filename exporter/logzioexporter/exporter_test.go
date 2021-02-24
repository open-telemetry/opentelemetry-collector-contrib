// Copyright The OpenTelemetry Authors
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

package logzioexporter

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/jaegertracing/jaeger/model"
	"github.com/logzio/jaeger-logzio/store/objects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	testService   = "testService"
	testHost      = "testHost"
	testOperation = "testOperation"
)

var testSpans = []*tracepb.Span{
	{
		TraceId:                 []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		SpanId:                  []byte{0, 0, 0, 0, 0, 0, 0, 2},
		Name:                    &tracepb.TruncatableString{Value: testOperation},
		Kind:                    tracepb.Span_SERVER,
		SameProcessAsParentSpan: &wrapperspb.BoolValue{Value: true},
	},
}

func testTraceExporter(td pdata.Traces, t *testing.T, cfg *Config) {
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, err := createTraceExporter(context.Background(), params, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.ConsumeTraces(ctx, td)
	require.NoError(t, err)
	err = exporter.Shutdown(ctx)
	require.NoError(t, err)
}

func TestNullTraceExporterConfig(tester *testing.T) {
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := newLogzioTraceExporter(nil, params)
	assert.Error(tester, err, "Null exporter config should produce error")
}

func testMetricsExporter(md pdata.Metrics, t *testing.T, cfg *Config) {
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, err := createMetricsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	err = exporter.ConsumeMetrics(context.Background(), md)
	assert.NoError(t, err)
}

func TestNullExporterConfig(tester *testing.T) {
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := newLogzioExporter(nil, params)
	assert.Error(tester, err, "Null exporter config should produce error")
}

func TestNullTokenConfig(tester *testing.T) {
	cfg := Config{
		Region: "eu",
	}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := createTraceExporter(context.Background(), params, &cfg)
	assert.Error(tester, err, "Empty token should produce error")
}

func TestEmptyNode(tester *testing.T) {
	cfg := Config{
		TracesToken: "test",
		Region:      "eu",
	}
	testTraceExporter(internaldata.OCToTraces(nil, nil, nil), tester, &cfg)
}

func TestWriteSpanError(tester *testing.T) {
	cfg := Config{
		TracesToken: "test",
		Region:      "eu",
	}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, _ := newLogzioExporter(&cfg, params)
	oldFunc := exporter.WriteSpanFunc
	defer func() { exporter.WriteSpanFunc = oldFunc }()
	exporter.WriteSpanFunc = func(context.Context, *model.Span) error {
		return errors.New("fail")
	}
	droppedSpans, _ := exporter.pushTraceData(context.Background(), internaldata.OCToTraces(nil, nil, testSpans))
	assert.Equal(tester, 1, droppedSpans)
}

func TestConversionTraceError(tester *testing.T) {
	cfg := Config{
		TracesToken: "test",
		Region:      "eu",
	}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, _ := newLogzioExporter(&cfg, params)
	oldFunc := exporter.InternalTracesToJaegerTraces
	defer func() { exporter.InternalTracesToJaegerTraces = oldFunc }()
	exporter.InternalTracesToJaegerTraces = func(td pdata.Traces) ([]*model.Batch, error) {
		return nil, errors.New("fail")
	}
	_, err := exporter.pushTraceData(context.Background(), internaldata.OCToTraces(nil, nil, testSpans))
	assert.Error(tester, err)
}

func TestPushTraceData(tester *testing.T) {
	var recordedRequests []byte
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		recordedRequests, _ = ioutil.ReadAll(req.Body)
		rw.WriteHeader(http.StatusOK)
	}))
	cfg := Config{
		TracesToken:    "test",
		Region:         "eu",
		CustomEndpoint: server.URL,
	}
	defer server.Close()

	node := &commonpb.Node{
		ServiceInfo: &commonpb.ServiceInfo{
			Name: testService,
		},
		Identifier: &commonpb.ProcessIdentifier{
			HostName: testHost,
		},
	}
	testTraceExporter(internaldata.OCToTraces(node, nil, testSpans), tester, &cfg)
	requests := strings.Split(string(recordedRequests), "\n")
	var logzioSpan objects.LogzioSpan
	assert.NoError(tester, json.Unmarshal([]byte(requests[0]), &logzioSpan))
	assert.Equal(tester, testOperation, logzioSpan.OperationName)
	assert.Equal(tester, testService, logzioSpan.Process.ServiceName)

	var logzioService objects.LogzioService
	assert.NoError(tester, json.Unmarshal([]byte(requests[1]), &logzioService))

	assert.Equal(tester, testOperation, logzioService.OperationName)
	assert.Equal(tester, testService, logzioService.ServiceName)

}

func TestPushMetricsData(tester *testing.T) {
	cfg := Config{
		MetricsToken:   "test",
		Region:         "eu",
		CustomEndpoint: "url",
	}
	md := pdata.NewMetrics()

	testMetricsExporter(md, tester, &cfg)
}
