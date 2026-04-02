// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// proxyProtocolV2Signature is the fixed 12-byte signature every PPv2 header starts with.
var proxyProtocolV2Signature = []byte{
	0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A,
}

const (
	proxyProtocolV2HeaderLen = 16

	ppV2CmdLocal = 0x00
	ppV2CmdProxy = 0x01

	ppV2FamilyUnspec = 0x00
	ppV2FamilyIPv4   = 0x10
	ppV2FamilyIPv6   = 0x20
	ppV2FamilyUnix   = 0x30

	ppV2AddrLenIPv4 = 12
	ppV2AddrLenIPv6 = 36
)

// parseProxyProtocolV2Header reads and parses a Proxy Protocol v2 header from r.
// It returns the source address reported by the proxy, or nil when the command
// is LOCAL or the address family is UNSPEC/UNIX.
//
// Spec: https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt §2.2
func parseProxyProtocolV2Header(r io.Reader) (net.Addr, error) {
	header := make([]byte, proxyProtocolV2HeaderLen)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("reading proxy protocol v2 header: %w", err)
	}

	for i, b := range proxyProtocolV2Signature {
		if header[i] != b {
			return nil, fmt.Errorf("invalid proxy protocol v2 signature at byte %d: got 0x%02X, want 0x%02X", i, header[i], b)
		}
	}

	verCmd := header[12]
	if verCmd>>4 != 0x2 {
		return nil, fmt.Errorf("unsupported proxy protocol version %d, only version 2 is supported", verCmd>>4)
	}
	cmd := verCmd & 0x0F

	family := header[13] & 0xF0
	addrLen := binary.BigEndian.Uint16(header[14:16])

	addrBlock := make([]byte, addrLen)
	if addrLen > 0 {
		if _, err := io.ReadFull(r, addrBlock); err != nil {
			return nil, fmt.Errorf("reading proxy protocol v2 address block: %w", err)
		}
	}

	if cmd == ppV2CmdLocal {
		return nil, nil
	}
	if cmd != ppV2CmdProxy {
		return nil, fmt.Errorf("unsupported proxy protocol v2 command: 0x%02X", cmd)
	}

	switch family {
	case ppV2FamilyIPv4:
		if int(addrLen) < ppV2AddrLenIPv4 {
			return nil, fmt.Errorf("proxy protocol v2 IPv4 address block too short: got %d bytes, need %d", addrLen, ppV2AddrLenIPv4)
		}
		srcIP := make(net.IP, 4)
		copy(srcIP, addrBlock[0:4])
		srcPort := binary.BigEndian.Uint16(addrBlock[8:10])
		return &net.UDPAddr{IP: srcIP, Port: int(srcPort)}, nil

	case ppV2FamilyIPv6:
		if int(addrLen) < ppV2AddrLenIPv6 {
			return nil, fmt.Errorf("proxy protocol v2 IPv6 address block too short: got %d bytes, need %d", addrLen, ppV2AddrLenIPv6)
		}
		srcIP := make(net.IP, 16)
		copy(srcIP, addrBlock[0:16])
		srcPort := binary.BigEndian.Uint16(addrBlock[32:34])
		return &net.UDPAddr{IP: srcIP, Port: int(srcPort)}, nil

	default:
		// UNSPEC and UNIX carry no usable IP address — fall back to the actual
		// connection remote address.
		return nil, nil
	}
}

// applyProxyProtocolV2 parses a PPv2 header from the start of datagram and
// returns the payload bytes (everything after the header) together with the
// proxied source address. When the command is LOCAL or the family is
// UNSPEC/UNIX, fallbackAddr is returned unchanged.
func applyProxyProtocolV2(datagram []byte, fallbackAddr net.Addr) ([]byte, net.Addr, error) {
	reader := bytes.NewReader(datagram)
	proxyAddr, err := parseProxyProtocolV2Header(reader)
	if err != nil {
		return nil, nil, err
	}
	headerLen := len(datagram) - reader.Len()
	payload := datagram[headerLen:]
	if proxyAddr != nil {
		return payload, proxyAddr, nil
	}
	return payload, fallbackAddr, nil
}
