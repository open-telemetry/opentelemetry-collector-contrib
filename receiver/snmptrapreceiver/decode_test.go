// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/require"
)

func TestDecodeV2ColdStart(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	addr := &net.UDPAddr{IP: net.ParseIP("172.20.20.2"), Port: 161}
	packet := &gosnmp.SnmpPacket{
		Version:   gosnmp.Version2c,
		Community: "public",
		PDUType:   gosnmp.SNMPv2Trap,
		Variables: []gosnmp.SnmpPDU{
			{Name: ".1.3.6.1.2.1.1.3.0", Type: gosnmp.TimeTicks, Value: uint32(1234)},
			{Name: ".1.3.6.1.6.3.1.1.4.1.0", Type: gosnmp.ObjectIdentifier, Value: ".1.3.6.1.6.3.1.1.5.1"},
			{Name: ".1.3.6.1.2.1.1.5.0", Type: gosnmp.OctetString, Value: []byte("spine1")},
		},
	}

	rec := Decode(packet, addr, now, DecodeOptions{})
	require.Equal(t, "172.20.20.2", rec.Source)
	require.Equal(t, "2c", rec.Version)
	require.Equal(t, "trap", rec.PDUType)
	require.Equal(t, "1.3.6.1.6.3.1.1.5.1", rec.TrapOID)
	require.Equal(t, rec.TrapOID, rec.TrapName)
	require.False(t, rec.Resolved())
	require.Empty(t, rec.Community)
	require.Equal(t, uint(1234), rec.SysUpTime)
	require.Len(t, rec.Varbinds, 2) // snmpTrapOID omitted from body; sysUpTime kept
	require.Equal(t, "spine1", rec.Varbinds[1].Value)

	line, err := rec.MarshalJSONLine()
	require.NoError(t, err)
	var round Record
	require.NoError(t, json.Unmarshal(line, &round))
	require.Equal(t, rec.TrapOID, round.TrapOID)
}

func TestDecodeIncludeCommunity(t *testing.T) {
	packet := &gosnmp.SnmpPacket{
		Version:   gosnmp.Version2c,
		Community: "secret",
		Variables: []gosnmp.SnmpPDU{
			{Name: snmpTrapOID, Type: gosnmp.ObjectIdentifier, Value: "1.3.6.1.6.3.1.1.5.1"},
		},
	}
	rec := Decode(packet, nil, time.Now(), DecodeOptions{IncludeCommunity: true})
	require.Equal(t, "secret", rec.Community)
}

func TestDecodeV1GenericColdStart(t *testing.T) {
	packet := &gosnmp.SnmpPacket{
		Version: gosnmp.Version1,
		PDUType: gosnmp.Trap,
		SnmpTrap: gosnmp.SnmpTrap{
			Enterprise:   ".1.3.6.1.4.1.6527",
			AgentAddress: "10.0.0.1",
			GenericTrap:  0,
			SpecificTrap: 0,
			Timestamp:    99,
		},
	}
	rec := Decode(packet, &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1)}, time.Now(), DecodeOptions{})
	require.Equal(t, "1", rec.Version)
	require.Equal(t, "1.3.6.1.6.3.1.1.5.1", rec.TrapOID)
	require.Equal(t, "10.0.0.1", rec.AgentAddress)
	require.Equal(t, uint(99), rec.SysUpTime)
	require.NotNil(t, rec.GenericTrap)
	require.Equal(t, 0, *rec.GenericTrap)
}

func TestDecodeV1EnterpriseSpecific(t *testing.T) {
	packet := &gosnmp.SnmpPacket{
		Version: gosnmp.Version1,
		SnmpTrap: gosnmp.SnmpTrap{
			Enterprise:   "1.3.6.1.4.1.9.9",
			GenericTrap:  6,
			SpecificTrap: 42,
		},
	}
	rec := Decode(packet, nil, time.Now(), DecodeOptions{})
	require.Equal(t, "1.3.6.1.4.1.9.9.0.42", rec.TrapOID)
}

func TestDecodeV3EngineLabels(t *testing.T) {
	packet := &gosnmp.SnmpPacket{
		Version:         gosnmp.Version3,
		ContextName:     "mgmt",
		ContextEngineID: "\x80\x00\x1f\x88\x80",
		PDUType:         gosnmp.SNMPv2Trap,
		Variables:       []gosnmp.SnmpPDU{{Name: snmpTrapOID, Type: gosnmp.ObjectIdentifier, Value: "1.3.6.1.6.3.1.1.5.3"}},
	}
	rec := Decode(packet, nil, time.Now(), DecodeOptions{})
	require.Equal(t, "3", rec.Version)
	require.Equal(t, "mgmt", rec.ContextName)
	require.Equal(t, "80001f8880", rec.EngineID)
	require.Equal(t, "1.3.6.1.6.3.1.1.5.3", rec.TrapOID)
}

func TestDecodeOctetStringHex(t *testing.T) {
	packet := &gosnmp.SnmpPacket{
		Version: gosnmp.Version2c,
		Variables: []gosnmp.SnmpPDU{
			{Name: snmpTrapOID, Type: gosnmp.ObjectIdentifier, Value: "1.3.6.1.6.3.1.1.5.1"},
			{Name: "1.3.6.1.2.1.2.2.1.6.1", Type: gosnmp.OctetString, Value: []byte{0x00, 0x11, 0xff}},
		},
	}
	rec := Decode(packet, nil, time.Now(), DecodeOptions{})
	require.Equal(t, "0011ff", rec.Varbinds[0].Value)
}

func TestLookupTranslator(t *testing.T) {
	tr := mapTranslator{
		"1.3.6.1.6.3.1.1.5.1": {Name: "SNMPv2-MIB::coldStart", MIB: "SNMPv2-MIB"},
	}
	packet := &gosnmp.SnmpPacket{
		Version: gosnmp.Version2c,
		Variables: []gosnmp.SnmpPDU{
			{Name: snmpTrapOID, Type: gosnmp.ObjectIdentifier, Value: ".1.3.6.1.6.3.1.1.5.1"},
		},
	}
	rec := Decode(packet, nil, time.Now(), DecodeOptions{Translator: tr})
	require.True(t, rec.Resolved())
	require.Equal(t, "SNMPv2-MIB::coldStart", rec.TrapName)
	require.Equal(t, "SNMPv2-MIB", rec.MIB)
}

type mapTranslator map[string]Lookup

func (m mapTranslator) Lookup(oid string) (Lookup, error) {
	if l, ok := m[NormalizeOID(oid)]; ok {
		return l, nil
	}
	return Lookup{}, errUnresolved
}

func TestNewGosmiTranslatorEmpty(t *testing.T) {
	tr, err := NewGosmiTranslator(nil, nil)
	require.NoError(t, err)
	_, ok := tr.(NoopTranslator)
	require.True(t, ok)
}
