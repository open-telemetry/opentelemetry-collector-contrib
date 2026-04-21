// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/clientutil"

import (
	"net"
	"strings"

	"go.opentelemetry.io/collector/client"
)

// Address returns the address of the client connecting to the collector.
func Address(client client.Info) string {
	if client.Addr == nil {
		return ""
	}
	switch addr := client.Addr.(type) {
	case *net.UDPAddr:
		return addr.IP.String()
	case *net.TCPAddr:
		return addr.IP.String()
	case *net.IPAddr:
		return addr.IP.String()
	}

	// If this is not a known address type, check for known "untyped" formats.
	// 1.1.1.1:<port>

	lastColonIndex := strings.LastIndex(client.Addr.String(), ":")
	if lastColonIndex != -1 {
		ipString := client.Addr.String()[:lastColonIndex]
		ip := net.ParseIP(ipString)
		if ip != nil {
			return ip.String()
		}
	}

	return client.Addr.String()
}
