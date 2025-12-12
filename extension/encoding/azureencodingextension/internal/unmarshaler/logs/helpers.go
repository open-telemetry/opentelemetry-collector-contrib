// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"crypto/tls"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

const (
	// OpenTelemetry attribute name for TLS protocol original value,
	// in case if the value could not be parsed into
	// `tls.protocol.name` and `tls.protocol.version` attributes
	attributeTLSProtocolOriginal = "tls.protocol.original"
)

var (
	// As tls.VersionSSL30 constant is deprecated, will use simple string here
	tlsVersionSSLv3 = "SSLv3"
	tlsVersionTLS10 = tls.VersionName(tls.VersionTLS10)
	tlsVersionTLS11 = tls.VersionName(tls.VersionTLS11)
	tlsVersionTLS12 = tls.VersionName(tls.VersionTLS12)
	tlsVersionTLS13 = tls.VersionName(tls.VersionTLS13)
)

// asSeverity converts the Azure log level to equivalent
// OpenTelemetry severity numbers. If the log level is not
// valid, then the 'Unspecified' value is returned.
// According to the documentation, the level Must be one of:
// `Informational`, `Warning`, `Error` or `Critical`.
// see https://learn.microsoft.com/en-us/azure/azure-monitor/platform/resource-logs-schema
func asSeverity(input string) plog.SeverityNumber {
	switch input {
	case "Informational":
		return plog.SeverityNumberInfo
	case "Warning":
		return plog.SeverityNumberWarn
	case "Error":
		return plog.SeverityNumberError
	case "Critical":
		return plog.SeverityNumberFatal
	default:
		return plog.SeverityNumberUnspecified
	}
}

// attrPutTLSProtoIf tries to parse provided value as TLS security protocol version,
// for example, "TLS 1.2" will be parsed into tls.protocol.name = "TLS" and tls.protocol.version = "1.2"
// If the value is not recognized - will set original value into "tls.protocol.original" attribute
// Puts at most 2 attributes
func attrPutTLSProtoIf(attrs pcommon.Map, securityProtocol string) {
	if securityProtocol == "" {
		// Nothing to do here
		return
	}

	var name, version string
	switch securityProtocol {
	case tlsVersionSSLv3:
		name = "SSL"
		version = "3"
	case tlsVersionTLS10:
		name = "TLS"
		version = "1.0"
	case tlsVersionTLS11:
		name = "TLS"
		version = "1.1"
	case tlsVersionTLS12:
		name = "TLS"
		version = "1.2"
	case tlsVersionTLS13:
		name = "TLS"
		version = "1.3"
	default:
		unmarshaler.AttrPutStrIf(attrs, attributeTLSProtocolOriginal, securityProtocol)
		return
	}

	unmarshaler.AttrPutStrIf(attrs, string(conventions.TLSProtocolNameKey), name)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.TLSProtocolVersionKey), version)
}

// attrPutDestination puts the value for the backend host name and endpoint
// in the expected field names. If backend hostname is empty, then the
// destination address and port depend only on the endpoint. If backend
// hostname is filled, then the destination address and port are based on
// it. If both are filled but different, then the endpoint will cover the
// network address and port.
// Puts at most 4 attributes
func attrPutDestination(attrs pcommon.Map, backendHostname, endpoint string) {
	if backendHostname == "" && endpoint == "" {
		// Nothing to do here
		return
	}

	// Only `endpoint` provided
	if backendHostname == "" {
		unmarshaler.AttrPutHostPortIf(attrs, string(conventions.DestinationAddressKey), string(conventions.DestinationPortKey), endpoint)
		return
	}

	// If both `endpoint` and `backendHostname` are provided - following rules are applied:
	// `backendHostname` will be parsed into `destination.address` and `destination.port`
	// `endpoint` will be parsed into `network.peer.address` and `network.peer.port` if it's value doesn't match `backendHostname` value
	unmarshaler.AttrPutHostPortIf(attrs, string(conventions.DestinationAddressKey), string(conventions.DestinationPortKey), backendHostname)
	if endpoint != backendHostname && endpoint != "" {
		unmarshaler.AttrPutHostPortIf(attrs, string(conventions.NetworkPeerAddressKey), string(conventions.NetworkPeerPortKey), endpoint)
	}
}
