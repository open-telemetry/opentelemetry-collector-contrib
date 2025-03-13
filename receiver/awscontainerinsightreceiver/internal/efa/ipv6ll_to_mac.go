// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package efa // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/efa"

import (
	"fmt"
	"net"
)

// IPv6LinkLocalToMAC converts an IPv6 link-local address to its corresponding MAC address.
// The IPv6 address must be in EUI-64 format for this conversion to work.
func IPv6LinkLocalToMAC(ipv6Addr string) (string, error) {
	// Parse the IPv6 address
	ip := net.ParseIP(ipv6Addr)
	if ip == nil || ip.To16() == nil {
		return "", fmt.Errorf("invalid IPv6 address")
	}

	// Verify it's a link-local address (fe80::/10)
	if !ip.IsLinkLocalUnicast() {
		return "", fmt.Errorf("not a link-local address")
	}

	// Extract interface identifier (last 64 bits)
	interfaceID := ip.To16()[8:]
	if len(interfaceID) != 8 {
		return "", fmt.Errorf("invalid interface identifier")
	}

	// Verify EUI-64 format (check for ff:fe in bytes 3-4)
	if interfaceID[3] != 0xff || interfaceID[4] != 0xfe {
		return "", fmt.Errorf("address does not use EUI-64 format")
	}

	// Reconstruct MAC address
	mac := make(net.HardwareAddr, 6)
	mac[0] = interfaceID[0] ^ 0x02 // XOR with 0b00000010 to invert Universal/Local bit
	mac[1] = interfaceID[1]
	mac[2] = interfaceID[2]
	mac[3] = interfaceID[5]
	mac[4] = interfaceID[6]
	mac[5] = interfaceID[7]

	return mac.String(), nil
}
