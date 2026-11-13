// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"bytes"
	"context"
	"encoding/asn1"
	"math"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

func TestTrapReceiverInformWideIgnoredErrorStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status []byte
	}{
		{name: "above Integer64", status: []byte{0, 0x80, 0, 0, 0, 0, 0, 0, 0}},
		{name: "below Integer64", status: []byte{0xff, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			_, bindings := informTestRequest(t, requestID, 200)
			wire := informTestMessage([]byte{2, 1, 1}, 0xa6,
				informTestInteger(t, requestID), informTestTLV(0x02, tc.status),
				informTestInteger(t, math.MaxInt32), bindings)
			_, err = conn.Write(wire)
			require.NoError(t, err)
			buffer := make([]byte, 65535)
			n, err := conn.Read(buffer)
			require.NoError(t, err, "a valid ignored status outside Integer64 must not prevent an acknowledgment")
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

func TestTrapReceiverMalformedInformBeforeAdmission(t *testing.T) {
	_, bindings := informTestRequest(t, 1, 0)
	var rawBindings asn1.RawValue
	_, err := asn1.Unmarshal(bindings, &rawBindings)
	require.NoError(t, err)
	// Preserve the mandatory first two notification bindings so malformed
	// trailing values cannot be hidden by notification identity validation.
	malformedBinding := informTestTLV(0x30, []byte{6, 1, 43, 5, 1, 0})
	malformedBindings := informTestTLV(0x30, append(bytes.Clone(rawBindings.Bytes), malformedBinding...))
	zero := []byte{2, 1, 0}
	for name, wire := range map[string][]byte{
		"request ID above Integer32":  informTestMessage([]byte{2, 1, 1}, 0xa6, informTestInteger(t, math.MaxInt32+1), zero, zero, bindings),
		"wrong request ID tag":        informTestMessage([]byte{2, 1, 1}, 0xa6, []byte{4, 1, '1'}, zero, zero, bindings),
		"nonminimal error status":     informTestMessage([]byte{2, 1, 1}, 0xa6, zero, []byte{2, 2, 0, 1}, zero, bindings),
		"negative error index":        informTestMessage([]byte{2, 1, 1}, 0xa6, zero, zero, informTestInteger(t, -1), bindings),
		"error index above Integer32": informTestMessage([]byte{2, 1, 1}, 0xa6, zero, zero, informTestInteger(t, math.MaxInt32+1), bindings),
		"nonempty binding null":       informTestMessage([]byte{2, 1, 1}, 0xa6, zero, zero, zero, malformedBindings),
	} {
		t.Run(name, func(t *testing.T) {
			tt := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.WithoutCancel(t.Context()))) })
			set := receivertest.NewNopSettings(metadata.Type)
			set.TelemetrySettings = tt.NewTelemetrySettings()
			cfg := defaultTrapsConfig()
			cfg.ListenAddress = "127.0.0.1:0"
			sink := &consumertest.LogsSink{}
			r := newLifecycleTrapReceiver(t, cfg, set, sink)
			require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
			conn, err := net.DialUDP("udp", nil, r.conn.LocalAddr().(*net.UDPAddr))
			require.NoError(t, err)
			defer func() { require.NoError(t, conn.Close()) }()
			_, err = conn.Write(wire)
			require.NoError(t, err)
			require.Eventually(t, func() bool { return len(tt.SpanRecorder.Ended()) == 1 }, 3*time.Second, time.Millisecond)
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(50*time.Millisecond)))
			_, err = conn.Read(make([]byte, 65535))
			var timeout net.Error
			require.ErrorAs(t, err, &timeout)
			require.True(t, timeout.Timeout(), "malformed informs must not be acknowledged")
			shutdownLifecycleTrap(t, r)
			require.Zero(t, sink.LogRecordCount(), "malformed informs must not enter the delivery queue")
		})
	}
}
