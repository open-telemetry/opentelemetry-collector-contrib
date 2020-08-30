package logzioexporter

import (
	"context"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"testing"
)

func testTraceExporter(td pdata.Traces, t *testing.T) {
	//responseLock := sync.Mutex{}
	//
	//response := []byte{}
	//server := testingServer(func(data []byte) {
	//	responseLock.Lock()
	//	response = append(response, data...)
	//	responseLock.Unlock()
	//})
	//
	//u, _ := url.Parse(server.URL)
	//port, _ := strconv.Atoi(u.Port())
	cfg := Config{
		Token: "test",
		Region: "eu",
	}

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, err := createTraceExporter(context.Background(), params, &cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.ConsumeTraces(ctx, td)
	require.NoError(t, err)
	exporter.Shutdown(ctx)
}


func TestEmptyNode(t *testing.T) {
	td := consumerdata.TraceData{
		Node: nil,
		Spans: []*tracepb.Span{
			{
				TraceId:                 []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				SpanId:                  []byte{0, 0, 0, 0, 0, 0, 0, 2},
				Name:                    &tracepb.TruncatableString{Value: "root"},
				Kind:                    tracepb.Span_SERVER,
				SameProcessAsParentSpan: &wrapperspb.BoolValue{Value: true},
			},
		},
	}

	testTraceExporter(internaldata.OCToTraceData(td), t)
}