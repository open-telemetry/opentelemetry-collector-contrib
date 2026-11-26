// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"math"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestDecodeTrapNotification(t *testing.T) {
	for _, version := range []gosnmp.SnmpVersion{gosnmp.Version2c, gosnmp.Version3} {
		t.Run(version.String(), func(t *testing.T) {
			received := time.Unix(123456, 789)
			binary := []byte{0, 0xff, 0x80, 'x'}
			packet := &gosnmp.SnmpPacket{
				Version: version, PDUType: gosnmp.SNMPv2Trap, Community: "secret",
				ContextEngineID: "\x80\x00\x00\x01\x02", ContextName: "device",
				Variables: []gosnmp.SnmpPDU{
					{Name: "." + trapUptimeOID, Type: gosnmp.TimeTicks, Value: uint32(math.MaxUint32)},
					{Name: "." + trapIdentityOID, Type: gosnmp.ObjectIdentifier, Value: ".1.3.6.1.6.3.1.1.5.3"},
					{Name: ".1.3.6.1.4.1.9.1.0", Type: gosnmp.OctetString, Value: binary},
					{Name: ".1.3.6.1.4.1.9.1.0", Type: gosnmp.Counter64, Value: uint64(math.MaxUint64)},
				},
			}
			logs, err := decodeTrap(packet, &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 1234}, received, false)
			require.NoError(t, err)
			require.Equal(t, 1, logs.LogRecordCount())
			record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			assert.Equal(t, pcommon.NewTimestampFromTime(received), record.ObservedTimestamp())
			assert.Zero(t, record.Timestamp())
			assert.Equal(t, "192.0.2.1", record.Attributes().AsRaw()["network.peer.address"])
			assert.Equal(t, int64(1234), record.Attributes().AsRaw()["network.peer.port"])
			body := record.Body().Map().AsRaw()
			assert.Equal(t, "1.3.6.1.6.3.1.1.5.3", body["trap_oid"])
			assert.Equal(t, int64(math.MaxUint32), body["sys_up_time"])
			assert.NotContains(t, body, "community")
			bindings := body["varbinds"].([]any)
			require.Len(t, bindings, 4)
			assert.Equal(t, binary, bindings[2].(map[string]any)["value"])
			assert.Equal(t, "18446744073709551615", bindings[3].(map[string]any)["value"])
			if version == gosnmp.Version3 {
				assert.Equal(t, []byte(packet.ContextEngineID), body["context_engine_id"])
				assert.Equal(t, "device", body["context_name"])
			}
			// Queued byte values must not alias the reusable UDP input buffer.
			binary[0] = 42
			storedBindings, ok := record.Body().Map().Get("varbinds")
			require.True(t, ok)
			storedValue, ok := storedBindings.Slice().At(2).Map().Get("value")
			require.True(t, ok)
			assert.Equal(t, byte(0), storedValue.Bytes().At(0))
		})
	}
}

func TestDecodeTrapV1Identity(t *testing.T) {
	for _, tt := range []struct {
		name              string
		generic, specific int
		want              string
	}{
		{name: "cold start", generic: 0, want: "1.3.6.1.6.3.1.1.5.1"},
		{name: "link down", generic: 2, want: "1.3.6.1.6.3.1.1.5.3"},
		{name: "enterprise specific", generic: 6, specific: 42, want: "1.3.6.1.4.1.9.0.42"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			packet := &gosnmp.SnmpPacket{
				Version: gosnmp.Version1, PDUType: gosnmp.Trap, Community: "public",
				SnmpTrap: gosnmp.SnmpTrap{Enterprise: ".1.3.6.1.4.1.9", GenericTrap: tt.generic, SpecificTrap: tt.specific, Timestamp: 12345, AgentAddress: "192.0.2.2"},
			}
			logs, err := decodeTrap(packet, nil, time.Now(), true)
			require.NoError(t, err)
			body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()
			assert.Equal(t, tt.want, body["trap_oid"])
			assert.Equal(t, "1.3.6.1.4.1.9", body["enterprise"])
			assert.Equal(t, "192.0.2.2", body["agent_address"])
			assert.Equal(t, "public", body["community"])
			assert.Equal(t, int64(12345), body["sys_up_time"])
		})
	}
}

func TestDecodeTrapRejectsInvalidNotification(t *testing.T) {
	base := func() *gosnmp.SnmpPacket {
		return &gosnmp.SnmpPacket{Version: gosnmp.Version2c, PDUType: gosnmp.SNMPv2Trap, Variables: []gosnmp.SnmpPDU{
			{Name: trapUptimeOID, Type: gosnmp.TimeTicks, Value: uint32(10)},
			{Name: trapIdentityOID, Type: gosnmp.ObjectIdentifier, Value: "1.3.6.1.6.3.1.1.5.3"},
		}}
	}
	for _, tt := range []struct {
		name   string
		change func(*gosnmp.SnmpPacket)
	}{
		{name: "non notification", change: func(p *gosnmp.SnmpPacket) { p.PDUType = gosnmp.GetRequest }},
		{name: "missing bindings", change: func(p *gosnmp.SnmpPacket) { p.Variables = nil }},
		{name: "reversed bindings", change: func(p *gosnmp.SnmpPacket) { p.Variables[0], p.Variables[1] = p.Variables[1], p.Variables[0] }},
		{name: "invalid uptime type", change: func(p *gosnmp.SnmpPacket) { p.Variables[0].Type = gosnmp.Integer }},
		{name: "negative uptime", change: func(p *gosnmp.SnmpPacket) { p.Variables[0].Value = -1 }},
		{name: "oversized uptime", change: func(p *gosnmp.SnmpPacket) { p.Variables[0].Value = uint64(math.MaxUint32) + 1 }},
		{name: "invalid identity", change: func(p *gosnmp.SnmpPacket) { p.Variables[1].Value = "3.1.2" }},
		{name: "invalid identity type", change: func(p *gosnmp.SnmpPacket) { p.Variables[1].Type = gosnmp.OctetString }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			packet := base()
			tt.change(packet)
			_, err := decodeTrap(packet, nil, time.Now(), false)
			require.Error(t, err)
		})
	}
	_, err := decodeTrap(nil, nil, time.Now(), false)
	require.Error(t, err)
}

func TestTrapValuePreservesTypes(t *testing.T) {
	for _, tt := range []struct {
		name   string
		typeID gosnmp.Asn1BER
		value  any
		want   any
	}{
		{name: "signed", typeID: gosnmp.Integer, value: int32(-123), want: int64(-123)},
		{name: "counter32", typeID: gosnmp.Counter32, value: uint32(math.MaxUint32), want: int64(math.MaxUint32)},
		{name: "counter64", typeID: gosnmp.Counter64, value: uint64(math.MaxUint64), want: "18446744073709551615"},
		{name: "bytes", typeID: gosnmp.OctetString, value: []byte{0, 255}, want: []byte{0, 255}},
		{name: "oid", typeID: gosnmp.ObjectIdentifier, value: ".1.3.6.1.2.1.1.3.0", want: "1.3.6.1.2.1.1.3.0"},
		{name: "ip", typeID: gosnmp.IPAddress, value: "192.0.2.3", want: "192.0.2.3"},
		{name: "float", typeID: gosnmp.OpaqueFloat, value: float32(1.5), want: float64(1.5)},
		{name: "nan", typeID: gosnmp.OpaqueDouble, value: math.NaN(), want: "NaN"},
		{name: "infinity", typeID: gosnmp.OpaqueDouble, value: math.Inf(1), want: "+Inf"},
		{name: "null", typeID: gosnmp.Null, value: nil, want: nil},
		{name: "boolean", typeID: gosnmp.Boolean, value: true, want: true},
		{name: "bit string", typeID: gosnmp.BitString, value: gosnmp.BitStringValue{Bytes: []byte{0xa0}, BitLength: 3}, want: map[string]any{"bytes": []byte{0xa0}, "bit_length": int64(3)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			value := pcommon.NewValueEmpty()
			require.NoError(t, trapValue(value, gosnmp.SnmpPDU{Type: tt.typeID, Value: tt.value}))
			assert.Equal(t, tt.want, value.AsRaw())
		})
	}
	for _, binding := range []gosnmp.SnmpPDU{
		{Type: gosnmp.Counter32, Value: -1},
		{Type: gosnmp.Counter32, Value: uint64(math.MaxUint32) + 1},
		{Type: gosnmp.ObjectIdentifier, Value: "1..2"},
		{Type: gosnmp.IPAddress, Value: "invalid"},
		{Type: gosnmp.BitString, Value: gosnmp.BitStringValue{Bytes: []byte{0}, BitLength: 9}},
	} {
		require.Error(t, trapValue(pcommon.NewValueEmpty(), binding))
	}
}

func TestTrapOIDValidation(t *testing.T) {
	oid, err := trapOID(".1.03.6.1")
	require.NoError(t, err)
	assert.Equal(t, "1.3.6.1", oid)
	for _, raw := range []string{"", "1", "3.1", "1.40", "1..2", "1.-1", "1.+2", "1.4294967296", "1.3." + strings.Repeat("1.", 127) + "1"} {
		_, err = trapOID(raw)
		require.Error(t, err, raw)
	}
}
