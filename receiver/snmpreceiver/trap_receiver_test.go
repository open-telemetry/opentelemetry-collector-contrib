// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

func TestTrapReceiverUDPDelivery(t *testing.T) {
	sink := &consumertest.LogsSink{}
	cfg := defaultTrapsConfig()
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.Attributes = map[string]string{"site": "lab"}
	r := newLifecycleTrapReceiver(t, cfg, receivertest.NewNopSettings(metadata.Type), sink)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))

	// Malformed input must not terminate the listener or prevent a later trap.
	sendLifecycleTrap(t, r, []byte{0x30, 0xff})
	sendLifecycleTrap(t, r, lifecycleTrapWire(t, gosnmp.SNMPv2Trap))
	require.Eventually(t, func() bool { return sink.LogRecordCount() == 1 }, 3*time.Second, time.Millisecond)
	lr := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	identity, ok := lr.Attributes().Get("snmp.trap.oid")
	require.True(t, ok)
	require.Equal(t, "1.3.6.1.6.3.1.1.5.3", identity.Str())
	site, ok := lr.Attributes().Get("site")
	require.True(t, ok)
	require.Equal(t, "lab", site.Str())
	peer, ok := lr.Attributes().Get("network.peer.address")
	require.True(t, ok)
	require.Equal(t, "127.0.0.1", peer.Str())
	_, ok = lr.Body().Map().Get("community")
	require.False(t, ok)
	bindings, ok := lr.Body().Map().Get("varbinds")
	require.True(t, ok)
	require.Equal(t, 3, bindings.Slice().Len())
}

func TestTrapReceiverMutatingConsumerTelemetry(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.WithoutCancel(t.Context()))) })
	set := receivertest.NewNopSettings(metadata.Type)
	set.TelemetrySettings = tt.NewTelemetrySettings()
	delivered := make(chan trace.SpanContext, 1)
	next, err := consumer.NewLogs(func(ctx context.Context, logs plog.Logs) error {
		delivered <- trace.SpanContextFromContext(ctx)
		// Model downstream ownership transfer: reading logs after this returns
		// would observe zero records rather than the received notification.
		logs.MoveTo(plog.NewLogs())
		return nil
	}, consumer.WithCapabilities(consumer.Capabilities{MutatesData: true}))
	require.NoError(t, err)
	cfg := defaultTrapsConfig()
	cfg.ListenAddress = "127.0.0.1:0"
	r := newLifecycleTrapReceiver(t, cfg, set, next)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	sendLifecycleTrap(t, r, lifecycleTrapWire(t, gosnmp.SNMPv2Trap))
	var downstreamSpan trace.SpanContext
	select {
	case downstreamSpan = <-delivered:
	case <-time.After(3 * time.Second):
		t.Fatal("notification was not delivered")
	}
	require.True(t, downstreamSpan.IsValid(), "downstream must receive the notification observation context")
	shutdownLifecycleTrap(t, r)
	spans := tt.SpanRecorder.Ended()
	require.Len(t, spans, 1)
	require.Equal(t, spans[0].SpanContext(), downstreamSpan)
	require.Equal(t, int64(1), lifecycleTrapMetric(t, tt, "otelcol_receiver_accepted_log_records"))
}

func TestTrapReceiverInformAdmission(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		name := "community rejected"
		if allowed {
			name = "admitted"
		}
		t.Run(name, func(t *testing.T) {
			tt := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.WithoutCancel(t.Context()))) })
			set := receivertest.NewNopSettings(metadata.Type)
			set.TelemetrySettings = tt.NewTelemetrySettings()
			cfg := defaultTrapsConfig()
			cfg.ListenAddress = "127.0.0.1:0"
			if !allowed {
				cfg.Communities = []configopaque.String{"private"}
			}
			sink := &consumertest.LogsSink{}
			r := newLifecycleTrapReceiver(t, cfg, set, sink)
			require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
			conn, err := net.DialUDP("udp", nil, r.conn.LocalAddr().(*net.UDPAddr))
			require.NoError(t, err)
			defer func() { require.NoError(t, conn.Close()) }()
			_, err = conn.Write(lifecycleTrapWire(t, gosnmp.InformRequest))
			require.NoError(t, err)
			if !allowed {
				require.Eventually(t, func() bool { return len(tt.SpanRecorder.Ended()) == 1 }, 3*time.Second, time.Millisecond)
				require.NoError(t, conn.SetReadDeadline(time.Now().Add(50*time.Millisecond)))
				_, err = conn.Read(make([]byte, 65535))
				var timeout net.Error
				require.ErrorAs(t, err, &timeout)
				require.True(t, timeout.Timeout(), "rejected informs must not be acknowledged")
				require.Zero(t, sink.LogRecordCount())
				return
			}
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
			buffer := make([]byte, 65535)
			n, err := conn.Read(buffer)
			require.NoError(t, err)
			parser := &gosnmp.GoSNMP{Version: gosnmp.Version2c}
			response, err := parser.UnmarshalTrap(buffer[:n], false)
			require.NoError(t, err)
			require.Equal(t, gosnmp.GetResponse, response.PDUType)
			require.Equal(t, uint32(17), response.RequestID)
			require.Equal(t, gosnmp.NoError, response.Error)
			require.Equal(t, uint8(0), response.ErrorIndex)
			require.Len(t, response.Variables, 3)
			require.Eventually(t, func() bool { return sink.LogRecordCount() == 1 }, 3*time.Second, time.Millisecond)
		})
	}
}

func TestTrapReceiverInformIgnoredErrorStatus(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int64
	}{
		{name: "unknown positive", status: 19},
		{name: "negative", status: -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultTrapsConfig()
			cfg.ListenAddress = "127.0.0.1:0"
			sink := &consumertest.LogsSink{}
			r := newLifecycleTrapReceiver(t, cfg, receivertest.NewNopSettings(metadata.Type), sink)
			require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
			conn, err := net.DialUDP("udp", nil, r.conn.LocalAddr().(*net.UDPAddr))
			require.NoError(t, err)
			defer func() { require.NoError(t, conn.Close()) }()
			require.NoError(t, conn.SetDeadline(time.Now().Add(3*time.Second)))

			const requestID int64 = -12345
			_, bindings := informTestRequest(t, requestID, 0)
			wire := informTestMessage([]byte{2, 1, 1}, 0xa6,
				informTestInteger(t, requestID), informTestInteger(t, tt.status),
				informTestInteger(t, 0), bindings)
			_, err = conn.Write(wire)
			require.NoError(t, err)
			buffer := make([]byte, 65535)
			n, err := conn.Read(buffer)
			require.NoError(t, err, "an admitted inform must be acknowledged regardless of its incoming error-status")
			id, status, index, responseBindings := decodeInformTestResponse(t, buffer[:n])
			require.Equal(t, requestID, id)
			require.Zero(t, status)
			require.Zero(t, index)
			require.Equal(t, bindings, responseBindings)

			require.Eventually(t, func() bool { return sink.LogRecordCount() == 1 }, 3*time.Second, time.Millisecond)
			shutdownLifecycleTrap(t, r)
			require.Equal(t, 1, sink.LogRecordCount(), "one inform must deliver exactly one notification log")
		})
	}
}

func TestTrapReceiverStartFailure(t *testing.T) {
	t.Run("canceled context", func(t *testing.T) {
		cfg := defaultTrapsConfig()
		cfg.ListenAddress = "127.0.0.1:0"
		r := newLifecycleTrapReceiver(t, cfg, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, r.Start(ctx, componenttest.NewNopHost()), context.Canceled)
		require.Nil(t, r.conn)
		require.False(t, r.started)
	})
	t.Run("occupied UDP port", func(t *testing.T) {
		occupied, err := net.ListenPacket("udp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, occupied.Close()) })
		cfg := defaultTrapsConfig()
		cfg.ListenAddress = occupied.LocalAddr().String()
		r := newLifecycleTrapReceiver(t, cfg, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
		require.Error(t, r.Start(t.Context(), componenttest.NewNopHost()))
		require.Nil(t, r.conn)
		require.False(t, r.started)
	})
}

func TestTrapReceiverQueueFullAndDrain(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.WithoutCancel(t.Context()))) })
	set := receivertest.NewNopSettings(metadata.Type)
	set.TelemetrySettings = tt.NewTelemetrySettings()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var delivered atomic.Int32
	next, err := consumer.NewLogs(func(ctx context.Context, _ plog.Logs) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
			delivered.Add(1)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	require.NoError(t, err)
	cfg := defaultTrapsConfig()
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.QueueSize = 1
	r := newLifecycleTrapReceiver(t, cfg, set, next)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	wire := lifecycleTrapWire(t, gosnmp.SNMPv2Trap)
	sendLifecycleTrap(t, r, wire)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not receive the first notification")
	}
	sendLifecycleTrap(t, r, wire)
	require.Eventually(t, func() bool { return len(r.queue) == 1 }, 3*time.Second, time.Millisecond)
	fullConn, err := net.DialUDP("udp", nil, r.conn.LocalAddr().(*net.UDPAddr))
	require.NoError(t, err)
	defer func() { require.NoError(t, fullConn.Close()) }()
	_, err = fullConn.Write(lifecycleTrapWire(t, gosnmp.InformRequest))
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		for _, span := range tt.SpanRecorder.Ended() {
			if span.Status().Description == errTrapQueueFull.Error() {
				return true
			}
		}
		return false
	}, 3*time.Second, time.Millisecond, "queue overflow must be reported")
	require.NoError(t, fullConn.SetReadDeadline(time.Now().Add(50*time.Millisecond)))
	_, err = fullConn.Read(make([]byte, 65535))
	var timeout net.Error
	require.ErrorAs(t, err, &timeout)
	require.True(t, timeout.Timeout(), "an inform dropped by a full queue must not be acknowledged")
	close(release)
	shutdownLifecycleTrap(t, r)
	require.Equal(t, int32(2), delivered.Load(), "shutdown must drain both admitted notifications")
	require.Equal(t, int64(2), lifecycleTrapMetric(t, tt, "otelcol_receiver_accepted_log_records"))
	require.Equal(t, int64(1), lifecycleTrapMetric(t, tt, "otelcol_receiver_refused_log_records"))
}

func TestTrapReceiverShutdownDeadline(t *testing.T) {
	entered := make(chan struct{}, 1)
	consumerStopped := make(chan struct{})
	next, err := consumer.NewLogs(func(ctx context.Context, _ plog.Logs) error {
		entered <- struct{}{}
		<-ctx.Done()
		close(consumerStopped)
		return ctx.Err()
	})
	require.NoError(t, err)
	cfg := defaultTrapsConfig()
	cfg.ListenAddress = "127.0.0.1:0"
	r := newLifecycleTrapReceiver(t, cfg, receivertest.NewNopSettings(metadata.Type), next)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	sendLifecycleTrap(t, r, lifecycleTrapWire(t, gosnmp.SNMPv2Trap))
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not receive the notification")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, r.Shutdown(ctx), context.DeadlineExceeded)
	select {
	case <-consumerStopped:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown deadline did not cancel downstream consumption")
	}
	select {
	case <-r.consumeDone:
	case <-time.After(3 * time.Second):
		t.Fatal("consumption goroutine did not terminate")
	}
}

func newLifecycleTrapReceiver(t *testing.T, cfg *TrapsConfig, set receiver.Settings, next consumer.Logs) *trapReceiver {
	t.Helper()
	recv, err := newTrapReceiver(cfg, set, next)
	require.NoError(t, err)
	r := recv.(*trapReceiver)
	t.Cleanup(func() { shutdownLifecycleTrap(t, r) })
	return r
}

func shutdownLifecycleTrap(t *testing.T, r *trapReceiver) {
	t.Helper()
	// Cleanup runs after the test context is canceled; allow a bounded drain.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 3*time.Second)
	defer cancel()
	require.NoError(t, r.Shutdown(ctx))
}

func sendLifecycleTrap(t *testing.T, r *trapReceiver, wire []byte) {
	t.Helper()
	conn, err := net.DialUDP("udp", nil, r.conn.LocalAddr().(*net.UDPAddr))
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()
	require.NoError(t, conn.SetWriteDeadline(time.Now().Add(time.Second)))
	_, err = conn.Write(wire)
	require.NoError(t, err)
}

func lifecycleTrapWire(t *testing.T, kind gosnmp.PDUType) []byte {
	t.Helper()
	packet := &gosnmp.SnmpPacket{
		Version: gosnmp.Version2c, Community: "public", PDUType: kind, RequestID: 17,
		Variables: []gosnmp.SnmpPDU{
			{Name: ".1.3.6.1.2.1.1.3.0", Type: gosnmp.TimeTicks, Value: uint32(12345)},
			{Name: ".1.3.6.1.6.3.1.1.4.1.0", Type: gosnmp.ObjectIdentifier, Value: ".1.3.6.1.6.3.1.1.5.3"},
			{Name: ".1.3.6.1.2.1.2.2.1.1.2", Type: gosnmp.Integer, Value: 2},
		},
	}
	wire, err := packet.MarshalMsg()
	require.NoError(t, err)
	return wire
}

func lifecycleTrapMetric(t *testing.T, tt *componenttest.Telemetry, name string) int64 {
	t.Helper()
	metric, err := tt.GetMetric(name)
	require.NoError(t, err)
	sum, ok := metric.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	var total int64
	for _, point := range sum.DataPoints {
		total += point.Value
	}
	return total
}
