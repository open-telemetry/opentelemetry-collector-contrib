// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildPPv2Header builds a Proxy Protocol v2 binary header for testing.
func buildPPv2Header(cmd, family byte, addr []byte) []byte {
	var buf bytes.Buffer
	buf.Write(proxyProtocolV2Signature)
	buf.WriteByte(0x20 | cmd)    // version 2, command
	buf.WriteByte(family | 0x02) // family, DGRAM transport
	addrLen := uint16(len(addr))
	_ = binary.Write(&buf, binary.BigEndian, addrLen)
	buf.Write(addr)
	return buf.Bytes()
}

func buildIPv4AddrBlock(srcIP, dstIP net.IP, srcPort, dstPort uint16) []byte {
	block := make([]byte, ppV2AddrLenIPv4)
	copy(block[0:4], srcIP.To4())
	copy(block[4:8], dstIP.To4())
	binary.BigEndian.PutUint16(block[8:10], srcPort)
	binary.BigEndian.PutUint16(block[10:12], dstPort)
	return block
}

func buildIPv6AddrBlock(srcIP, dstIP net.IP, srcPort, dstPort uint16) []byte {
	block := make([]byte, ppV2AddrLenIPv6)
	copy(block[0:16], srcIP.To16())
	copy(block[16:32], dstIP.To16())
	binary.BigEndian.PutUint16(block[32:34], srcPort)
	binary.BigEndian.PutUint16(block[34:36], dstPort)
	return block
}

func TestParseProxyProtocolV2_IPv4(t *testing.T) {
	srcIP := net.ParseIP("192.168.1.10").To4()
	dstIP := net.ParseIP("10.0.0.1").To4()
	addrBlock := buildIPv4AddrBlock(srcIP, dstIP, 12345, 9000)
	payload := []byte("hello udp\n")
	raw := append(buildPPv2Header(ppV2CmdProxy, ppV2FamilyIPv4, addrBlock), payload...)

	gotPayload, gotAddr, err := applyProxyProtocolV2(raw, nil)
	require.NoError(t, err)
	require.NotNil(t, gotAddr)

	udpAddr, ok := gotAddr.(*net.UDPAddr)
	require.True(t, ok)
	assert.Equal(t, "192.168.1.10", udpAddr.IP.String())
	assert.Equal(t, 12345, udpAddr.Port)
	assert.Equal(t, payload, gotPayload)
}

func TestParseProxyProtocolV2_IPv6(t *testing.T) {
	srcIP := net.ParseIP("2001:db8::1")
	dstIP := net.ParseIP("2001:db8::2")
	addrBlock := buildIPv6AddrBlock(srcIP, dstIP, 54321, 9000)
	raw := buildPPv2Header(ppV2CmdProxy, ppV2FamilyIPv6, addrBlock)

	gotPayload, gotAddr, err := applyProxyProtocolV2(raw, nil)
	require.NoError(t, err)
	require.NotNil(t, gotAddr)

	udpAddr, ok := gotAddr.(*net.UDPAddr)
	require.True(t, ok)
	assert.Equal(t, "2001:db8::1", udpAddr.IP.String())
	assert.Equal(t, 54321, udpAddr.Port)
	assert.Empty(t, gotPayload)
}

func TestParseProxyProtocolV2_LocalCommandFallback(t *testing.T) {
	raw := buildPPv2Header(ppV2CmdLocal, ppV2FamilyUnspec, nil)
	raw = append(raw, []byte("payload")...)

	fallback := &net.UDPAddr{IP: net.ParseIP("9.9.9.9"), Port: 1234}
	gotPayload, gotAddr, err := applyProxyProtocolV2(raw, fallback)
	require.NoError(t, err)
	// LOCAL command: fallback address must be preserved
	assert.Equal(t, fallback, gotAddr)
	assert.Equal(t, []byte("payload"), gotPayload)
}

func TestParseProxyProtocolV2_UnspecFamilyFallback(t *testing.T) {
	raw := buildPPv2Header(ppV2CmdProxy, ppV2FamilyUnspec, nil)
	fallback := &net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 999}

	_, gotAddr, err := applyProxyProtocolV2(raw, fallback)
	require.NoError(t, err)
	assert.Equal(t, fallback, gotAddr)
}

func TestParseProxyProtocolV2_InvalidSignature(t *testing.T) {
	raw := buildPPv2Header(ppV2CmdProxy, ppV2FamilyIPv4, make([]byte, ppV2AddrLenIPv4))
	raw[5] = 0xFF

	_, _, err := applyProxyProtocolV2(raw, nil)
	require.ErrorContains(t, err, "invalid proxy protocol v2 signature")
}

func TestParseProxyProtocolV2_WrongVersion(t *testing.T) {
	raw := buildPPv2Header(ppV2CmdProxy, ppV2FamilyIPv4, make([]byte, ppV2AddrLenIPv4))
	raw[12] = 0x11 // version 1

	_, _, err := applyProxyProtocolV2(raw, nil)
	require.ErrorContains(t, err, "unsupported proxy protocol version")
}

func TestParseProxyProtocolV2_TruncatedHeader(t *testing.T) {
	_, _, err := applyProxyProtocolV2(proxyProtocolV2Signature[:8], nil)
	require.Error(t, err)
}

func TestParseProxyProtocolV2_UnsupportedCommand(t *testing.T) {
	raw := buildPPv2Header(0x0F, ppV2FamilyIPv4, make([]byte, ppV2AddrLenIPv4))
	_, _, err := applyProxyProtocolV2(raw, nil)
	require.ErrorContains(t, err, "unsupported proxy protocol v2 command")
}
