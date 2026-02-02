// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapprocessor

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"golang.org/x/net/websocket"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor/internal/metadata"
)

func TestSocketConnectionLogs(t *testing.T) {
	cfg := &Config{
		ServerConfig: confighttp.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Transport: "tcp",
				Endpoint:  "localhost:12001",
			},
		},
		Limit: 1,
	}
	logSink := &consumertest.LogsSink{}
	processor, err := NewFactory().CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg,
		logSink)
	require.NoError(t, err)
	t.Cleanup(func() {
		errProcessorShutdown := processor.Shutdown(t.Context())
		require.NoError(t, errProcessorShutdown)
	})
	err = processor.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	rawConn, err := net.Dial("tcp", "localhost:12001")
	require.NoError(t, err)
	wsConfig, err := websocket.NewConfig("http://localhost:12001", "http://localhost:12001")
	require.NoError(t, err)
	wsConn, err := websocket.NewClient(wsConfig, rawConn)
	require.NoError(t, err)
	t.Cleanup(func() {
		errWsClose := wsConn.Close()
		require.NoError(t, errWsClose)
	})

	requireClientWaitingForData(t, cfg)
	log := plog.NewLogs()
	log.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("foo")
	err = processor.ConsumeLogs(t.Context(), log)
	require.NoError(t, err)
	buf := make([]byte, 1024)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		n, _ := wsConn.Read(buf)
		assert.Equal(tt, 107, n)
	}, 1*time.Second, 100*time.Millisecond)
	require.JSONEq(t, `{"resourceLogs":[{"resource":{},"scopeLogs":[{"scope":{},"logRecords":[{"body":{"stringValue":"foo"}}]}]}]}`, string(buf[0:107]))
}

func TestSocketConnectionMetrics(t *testing.T) {
	cfg := &Config{
		ServerConfig: confighttp.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Transport: "tcp",
				Endpoint:  "localhost:12002",
			},
		},
		Limit: 1,
	}
	metricsSink := &consumertest.MetricsSink{}
	processor, err := NewFactory().CreateMetrics(t.Context(), processortest.NewNopSettings(metadata.Type), cfg,
		metricsSink)
	require.NoError(t, err)
	t.Cleanup(func() {
		errProcessorShutdown := processor.Shutdown(t.Context())
		require.NoError(t, errProcessorShutdown)
	})
	err = processor.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	rawConn, err := net.Dial("tcp", "localhost:12002")
	require.NoError(t, err)
	wsConfig, err := websocket.NewConfig("http://localhost:12001", "http://localhost:12001")
	require.NoError(t, err)
	wsConn, err := websocket.NewClient(wsConfig, rawConn)
	require.NoError(t, err)
	t.Cleanup(func() {
		errWsClose := wsConn.Close()
		require.NoError(t, errWsClose)
	})

	requireClientWaitingForData(t, cfg)
	metric := pmetric.NewMetrics()
	metric.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("foo")
	buf := make([]byte, 1024)
	err = processor.ConsumeMetrics(t.Context(), metric)
	require.NoError(t, err)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		n, _ := wsConn.Read(buf)
		assert.Equal(tt, 94, n)
	}, 1*time.Second, 100*time.Millisecond)
	require.JSONEq(t, `{"resourceMetrics":[{"resource":{},"scopeMetrics":[{"scope":{},"metrics":[{"name":"foo"}]}]}]}`, string(buf[0:94]))
}

func TestSocketConnectionTraces(t *testing.T) {
	cfg := &Config{
		ServerConfig: confighttp.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Transport: "tcp",
				Endpoint:  "localhost:12003",
			},
		},
		Limit: 1,
	}
	tracesSink := &consumertest.TracesSink{}
	processor, err := NewFactory().CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg,
		tracesSink)
	require.NoError(t, err)
	t.Cleanup(func() {
		errProcessorShutdown := processor.Shutdown(t.Context())
		require.NoError(t, errProcessorShutdown)
	})
	err = processor.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	rawConn, err := net.Dial("tcp", "localhost:12003")
	require.NoError(t, err)
	wsConfig, err := websocket.NewConfig("http://localhost:12001", "http://localhost:12001")
	require.NoError(t, err)
	wsConn, err := websocket.NewClient(wsConfig, rawConn)
	require.NoError(t, err)
	t.Cleanup(func() {
		errWsClose := wsConn.Close()
		require.NoError(t, errWsClose)
	})

	requireClientWaitingForData(t, cfg)
	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("foo")
	buf := make([]byte, 1024)
	err = processor.ConsumeTraces(t.Context(), trace)
	require.NoError(t, err)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		n, _ := wsConn.Read(buf)
		assert.Equal(tt, 100, n)
	}, 1*time.Second, 100*time.Millisecond)
	require.JSONEq(t, `{"resourceSpans":[{"resource":{},"scopeSpans":[{"scope":{},"spans":[{"name":"foo","status":{}}]}]}]}`, string(buf[0:100]))
}

func requireClientWaitingForData(t *testing.T, cfg *Config) {
	wsProc := getWsProcessorUnderTesting(t, cfg)
	require.Eventually(t, func() bool {
		wsProc.cs.mu.RLock()
		defer wsProc.cs.mu.RUnlock()
		return len(wsProc.cs.chanmap) > 0
	}, 2*time.Second, 100*time.Millisecond)
}

func getWsProcessorUnderTesting(t *testing.T, cfg *Config) *wsprocessor {
	wp := processors.GetOrAdd(cfg, nil)
	require.NotNil(t, wp)
	return wp.Unwrap().(*wsprocessor)
}
