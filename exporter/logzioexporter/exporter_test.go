package logzioexporter

import (
	"context"
	"encoding/json"
	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/logzio/jaeger-logzio/store/objects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

func testTraceExporter(td pdata.Traces, t *testing.T, cfg Config) {
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, err := createTraceExporter(context.Background(), params, &cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.ConsumeTraces(ctx, td)
	require.NoError(t, err)
	exporter.Shutdown(ctx)
}


func TestEmptyNode(tester *testing.T) {
	cfg := Config{
		Token: "test",
		Region: "eu",
	}
	td := consumerdata.TraceData{
		Node: nil,
		Spans: testSpans,
	}
	testTraceExporter(internaldata.OCToTraceData(td), tester, cfg)
}

func TestPushTraceData(tester *testing.T) {
	var recordedRequests []byte
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		recordedRequests, _ = ioutil.ReadAll(req.Body)
		rw.WriteHeader(http.StatusOK)
	}))
	cfg := Config{
		Token: "test",
		Region: "eu",
		CustomListenerAddress:	server.URL,
	}
	defer server.Close()

	td := consumerdata.TraceData{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{
				Name: testService,
			},
			Identifier: &commonpb.ProcessIdentifier{
				HostName: testHost,
			},
		},
		Spans: testSpans,
	}
	testTraceExporter(internaldata.OCToTraceData(td), tester, cfg)
	//time.Sleep(time.Second * 10)
	requests := strings.Split(string(recordedRequests), "\n")
	var logzioSpan objects.LogzioSpan
	assert.NoError(tester, json.Unmarshal([]byte(requests[0]), &logzioSpan))
	assert.Equal(tester, logzioSpan.OperationName, testOperation)
	assert.Equal(tester, logzioSpan.Process.ServiceName, testService)

	var logzioService objects.LogzioService
	assert.NoError(tester, json.Unmarshal([]byte(requests[1]), &logzioService))

	assert.Equal(tester, logzioService.OperationName, testOperation)
	assert.Equal(tester, logzioService.ServiceName, testService)

}