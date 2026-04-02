// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildPPv2Header builds a minimal Proxy Protocol v2 binary header.
// family: e.g. ppV2FamilyIPv4 (0x10), ppV2FamilyIPv6 (0x20), ppV2FamilyUnspec (0x00)
// cmd:    ppV2CmdProxy (0x01) or ppV2CmdLocal (0x00)
// addr:   the address block bytes (may be nil)
func buildPPv2Header(cmd, family byte, addr []byte) []byte {
	var buf bytes.Buffer
	buf.Write(proxyProtocolV2Signature)
	buf.WriteByte(0x20 | cmd)    // version 2, command
	buf.WriteByte(family | 0x01) // family, STREAM transport
	addrLen := uint16(len(addr))
	_ = binary.Write(&buf, binary.BigEndian, addrLen)
	buf.Write(addr)
	return buf.Bytes()
}

func buildIPv4AddrBlock(srcIP net.IP, dstIP net.IP, srcPort, dstPort uint16) []byte {
	block := make([]byte, ppV2AddrLenIPv4)
	copy(block[0:4], srcIP.To4())
	copy(block[4:8], dstIP.To4())
	binary.BigEndian.PutUint16(block[8:10], srcPort)
	binary.BigEndian.PutUint16(block[10:12], dstPort)
	return block
}

func buildIPv6AddrBlock(srcIP net.IP, dstIP net.IP, srcPort, dstPort uint16) []byte {
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
	raw := buildPPv2Header(ppV2CmdProxy, ppV2FamilyIPv4, addrBlock)
	// Append a fake payload so the reader position is correct after parsing.
	raw = append(raw, []byte("hello\n")...)

	r := bytes.NewReader(raw)
	addr, err := parseProxyProtocolV2Header(r)
	require.NoError(t, err)
	require.NotNil(t, addr)

	tcpAddr, ok := addr.(*net.TCPAddr)
	require.True(t, ok)
	assert.Equal(t, "192.168.1.10", tcpAddr.IP.String())
	assert.Equal(t, 12345, tcpAddr.Port)

	// Verify that the remaining bytes are the payload (reader is positioned correctly).
	remaining := make([]byte, 6)
	n, _ := r.Read(remaining)
	assert.Equal(t, "hello\n", string(remaining[:n]))
}

func TestParseProxyProtocolV2_IPv6(t *testing.T) {
	srcIP := net.ParseIP("2001:db8::1")
	dstIP := net.ParseIP("2001:db8::2")
	addrBlock := buildIPv6AddrBlock(srcIP, dstIP, 54321, 9000)
	raw := buildPPv2Header(ppV2CmdProxy, ppV2FamilyIPv6, addrBlock)

	r := bytes.NewReader(raw)
	addr, err := parseProxyProtocolV2Header(r)
	require.NoError(t, err)
	require.NotNil(t, addr)

	tcpAddr, ok := addr.(*net.TCPAddr)
	require.True(t, ok)
	assert.Equal(t, "2001:db8::1", tcpAddr.IP.String())
	assert.Equal(t, 54321, tcpAddr.Port)
}

func TestParseProxyProtocolV2_LocalCommand(t *testing.T) {
	// LOCAL command — no address is proxied; addr should be nil.
	raw := buildPPv2Header(ppV2CmdLocal, ppV2FamilyUnspec, nil)

	r := bytes.NewReader(raw)
	addr, err := parseProxyProtocolV2Header(r)
	require.NoError(t, err)
	assert.Nil(t, addr)
}

func TestParseProxyProtocolV2_UnspecFamily(t *testing.T) {
	// PROXY command with UNSPEC family — addr should be nil.
	raw := buildPPv2Header(ppV2CmdProxy, ppV2FamilyUnspec, nil)

	r := bytes.NewReader(raw)
	addr, err := parseProxyProtocolV2Header(r)
	require.NoError(t, err)
	assert.Nil(t, addr)
}

func TestParseProxyProtocolV2_UnixFamily(t *testing.T) {
	// PROXY command with UNIX family — addr should be nil (no IP address available).
	unixBlock := make([]byte, 216)
	raw := buildPPv2Header(ppV2CmdProxy, ppV2FamilyUnix, unixBlock)

	r := bytes.NewReader(raw)
	addr, err := parseProxyProtocolV2Header(r)
	require.NoError(t, err)
	assert.Nil(t, addr)
}

func TestParseProxyProtocolV2_InvalidSignature(t *testing.T) {
	raw := buildPPv2Header(ppV2CmdProxy, ppV2FamilyIPv4, make([]byte, ppV2AddrLenIPv4))
	raw[5] = 0xFF // corrupt a signature byte

	r := bytes.NewReader(raw)
	_, err := parseProxyProtocolV2Header(r)
	require.ErrorContains(t, err, "invalid proxy protocol v2 signature")
}

func TestParseProxyProtocolV2_WrongVersion(t *testing.T) {
	raw := buildPPv2Header(ppV2CmdProxy, ppV2FamilyIPv4, make([]byte, ppV2AddrLenIPv4))
	// Override byte 12: set version to 1 instead of 2.
	raw[12] = 0x11

	r := bytes.NewReader(raw)
	_, err := parseProxyProtocolV2Header(r)
	require.ErrorContains(t, err, "unsupported proxy protocol version")
}

func TestParseProxyProtocolV2_TruncatedHeader(t *testing.T) {
	// Only 8 bytes — not enough for a full 16-byte header.
	r := bytes.NewReader(proxyProtocolV2Signature[:8])
	_, err := parseProxyProtocolV2Header(r)
	require.Error(t, err)
}

func TestParseProxyProtocolV2_TruncatedAddressBlock(t *testing.T) {
	// Header claims 12-byte IPv4 address block but provides only 4 bytes.
	var buf bytes.Buffer
	buf.Write(proxyProtocolV2Signature)
	buf.WriteByte(0x21)                                           // version 2, PROXY command
	buf.WriteByte(ppV2FamilyIPv4 | 0x01)                          // IPv4, STREAM
	binary.Write(&buf, binary.BigEndian, uint16(ppV2AddrLenIPv4)) //nolint:errcheck
	buf.Write(make([]byte, 4))                                    // only 4 bytes instead of 12

	_, err := parseProxyProtocolV2Header(bytes.NewReader(buf.Bytes()))
	require.ErrorContains(t, err, "reading proxy protocol v2 address block")
}

func TestParseProxyProtocolV2_UnsupportedCommand(t *testing.T) {
	raw := buildPPv2Header(0x0F, ppV2FamilyIPv4, make([]byte, ppV2AddrLenIPv4))
	r := bytes.NewReader(raw)
	_, err := parseProxyProtocolV2Header(r)
	require.ErrorContains(t, err, "unsupported proxy protocol v2 command")
}
