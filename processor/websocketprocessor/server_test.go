// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package websocketprocessor

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"golang.org/x/net/websocket"
)

func TestSocketConnectionLogs(t *testing.T) {
	cfg := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "localhost:12001",
		},
	}
	logSink := &consumertest.LogsSink{}
	processor, err := NewFactory().CreateLogsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg,
		logSink)
	require.NoError(t, err)
	err = processor.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	rawConn, err := net.Dial("tcp", "localhost:12001")
	require.NoError(t, err)
	wsConfig, err := websocket.NewConfig("http://localhost:12001", "http://localhost:12001")
	require.NoError(t, err)
	wsConn, err := websocket.NewClient(wsConfig, rawConn)
	require.NoError(t, err)
	log := plog.NewLogs()
	log.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("foo")
	err = processor.ConsumeLogs(context.Background(), log)
	require.NoError(t, err)
	buf := make([]byte, 1024)
	require.Eventuallyf(t, func() bool {
		err = processor.ConsumeLogs(context.Background(), log)
		require.NoError(t, err)
		n, _ := wsConn.Read(buf)
		return n == 132
	}, 1*time.Second, 100*time.Millisecond, "received message")
	require.Equal(t, `{"resourceLogs":[{"resource":{},"scopeLogs":[{"scope":{},"logRecords":[{"body":{"stringValue":"foo"},"traceId":"","spanId":""}]}]}]}`, string(buf[0:132]))

	err = processor.Shutdown(context.Background())
	require.NoError(t, err)
	err = rawConn.Close()
	require.NoError(t, err)
}

func TestSocketConnectionMetrics(t *testing.T) {
	cfg := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "localhost:12002",
		},
	}
	metricsSink := &consumertest.MetricsSink{}
	processor, err := NewFactory().CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg,
		metricsSink)
	require.NoError(t, err)
	err = processor.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	rawConn, err := net.Dial("tcp", "localhost:12002")
	require.NoError(t, err)
	wsConfig, err := websocket.NewConfig("http://localhost:12001", "http://localhost:12001")
	require.NoError(t, err)
	wsConn, err := websocket.NewClient(wsConfig, rawConn)
	require.NoError(t, err)
	metric := pmetric.NewMetrics()
	metric.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("foo")
	buf := make([]byte, 1024)
	require.Eventuallyf(t, func() bool {
		err = processor.ConsumeMetrics(context.Background(), metric)
		require.NoError(t, err)
		n, _ := wsConn.Read(buf)
		return n == 94
	}, 1*time.Second, 100*time.Millisecond, "received message")
	require.Equal(t, `{"resourceMetrics":[{"resource":{},"scopeMetrics":[{"scope":{},"metrics":[{"name":"foo"}]}]}]}`, string(buf[0:94]))

	err = processor.Shutdown(context.Background())
	require.NoError(t, err)
	err = rawConn.Close()
	require.NoError(t, err)
}

func TestSocketConnectionTraces(t *testing.T) {
	cfg := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "localhost:12003",
		},
	}
	tracesSink := &consumertest.TracesSink{}
	processor, err := NewFactory().CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg,
		tracesSink)
	require.NoError(t, err)
	err = processor.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	rawConn, err := net.Dial("tcp", "localhost:12003")
	require.NoError(t, err)
	wsConfig, err := websocket.NewConfig("http://localhost:12001", "http://localhost:12001")
	require.NoError(t, err)
	wsConn, err := websocket.NewClient(wsConfig, rawConn)
	require.NoError(t, err)
	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("foo")
	buf := make([]byte, 1024)
	require.Eventuallyf(t, func() bool {
		err = processor.ConsumeTraces(context.Background(), trace)
		require.NoError(t, err)
		n, _ := wsConn.Read(buf)
		return n == 143
	}, 1*time.Second, 100*time.Millisecond, "received message")
	require.Equal(t, `{"resourceSpans":[{"resource":{},"scopeSpans":[{"scope":{},"spans":[{"traceId":"","spanId":"","parentSpanId":"","name":"foo","status":{}}]}]}]}`, string(buf[0:143]))

	err = processor.Shutdown(context.Background())
	require.NoError(t, err)
	err = rawConn.Close()
	require.NoError(t, err)
}
