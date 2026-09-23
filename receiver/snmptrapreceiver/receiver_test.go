// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver

import (
	"net"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmptrapreceiver/internal/metadata"
)

func TestHandleTrapForwardsLog(t *testing.T) {
	sink := &consumertest.LogsSink{}
	recv, err := newReceiver(&Config{ListenAddress: "127.0.0.1:0", Attributes: map[string]string{"job": "snmptrap"}}, receivertest.NewNopSettings(metadata.Type), sink)
	require.NoError(t, err)

	packet := &gosnmp.SnmpPacket{
		Version:   gosnmp.Version2c,
		Community: "public",
		PDUType:   gosnmp.SNMPv2Trap,
		Variables: []gosnmp.SnmpPDU{
			{Name: snmpTrapOID, Type: gosnmp.ObjectIdentifier, Value: "1.3.6.1.6.3.1.1.5.3"},
		},
	}
	recv.handleTrap(packet, &net.UDPAddr{IP: net.ParseIP("172.20.20.2"), Port: 161})
	require.Len(t, recv.queue, 1)
	ld := <-recv.queue
	require.Equal(t, 1, ld.LogRecordCount())
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	v, ok := lr.Attributes().Get("trap_oid")
	require.True(t, ok)
	require.Equal(t, "1.3.6.1.6.3.1.1.5.3", v.AsString())
}

func TestStartShutdown(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ListenAddress = "127.0.0.1:0"
	set := receivertest.NewNopSettings(metadata.Type)
	recv, err := factory.CreateLogs(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, recv.Start(t.Context(), nil))
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, recv.Shutdown(t.Context()))
}
