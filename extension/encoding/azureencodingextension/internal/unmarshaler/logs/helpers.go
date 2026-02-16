// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"bytes"
	"crypto/tls"
	"encoding/json"

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
	// OpenTelemetry attribute name for Network protocol original value,
	// in case if the value could not be parsed into
	// `network.protocol.name` and `network.protocol.version` attributes
	attributeNetworkProtocolOriginal = "network.protocol.original"
)

var (
	// As tls.VersionSSL30 constant is deprecated, will use simple string here
	tlsVersionSSLv3 = "SSLv3"
	tlsVersionTLS10 = tls.VersionName(tls.VersionTLS10)
	tlsVersionTLS11 = tls.VersionName(tls.VersionTLS11)
	tlsVersionTLS12 = tls.VersionName(tls.VersionTLS12)
	tlsVersionTLS13 = tls.VersionName(tls.VersionTLS13)
	// HTTP protocol versions
	httpVersion09 = "HTTP/0.9"
	httpVersion10 = "HTTP/1.0"
	httpVersion11 = "HTTP/1.1"
	httpVersion20 = "HTTP/2.0"
	httpVersion30 = "HTTP/3.0"
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

// attrPutHTTPProtoIf tries to parse provided value as HTTP protocol version,
// for example, "HTTP/1.1" will be parsed into network.protocol.name = "http" and network.protocol.version = "1.1"
// If the value is not recognized - will set original value into "network.protocol.original" attribute
// Puts at most 2 attributes
func attrPutHTTPProtoIf(attrs pcommon.Map, httpProtocol string) {
	if httpProtocol == "" {
		// Nothing to do here
		return
	}

	var version string
	switch httpProtocol {
	case httpVersion09:
		version = "0.9"
	case httpVersion10:
		version = "1.0"
	case httpVersion11:
		version = "1.1"
	case httpVersion20:
		version = "2.0"
	case httpVersion30:
		version = "3.0"
	default:
		unmarshaler.AttrPutStrIf(attrs, attributeNetworkProtocolOriginal, httpProtocol)
		return
	}

	// Protocol values SHOULD be normalized to lowercase as per SemConv
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkProtocolNameKey), "http")
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkProtocolVersionKey), version)
}

// convertInvalidSingleQuotedJSON tries to convert invalid, single quoted, JSON
// into valid and parsable JSON by substituting `'` to `"`, taking in account
// potential escaped single quotes, e.g. `\'`
func convertInvalidSingleQuotedJSON(data []byte) []byte {
	dataLen := len(data)

	if dataLen == 0 {
		return data
	}

	var newData bytes.Buffer
	newData.Grow(dataLen)

	inQuote := byte(0)
	for idx, b := range data {
		// Enter quoted string
		if (b == '\'' || b == '"') && (idx == 0 || data[idx-1] != '\\') && inQuote == byte(0) {
			// Mark that we are in quoted string
			inQuote = b
			// Replace single quote to double quote
			newData.WriteByte('"')
			continue
		}

		// Leave quoted string
		if (b == '\'' || b == '"') && (idx == 0 || data[idx-1] != '\\') && inQuote == b {
			// Mark that we left quoted string
			inQuote = byte(0)
			// Write closing double quote instead single quote
			newData.WriteByte('"')
			continue
		}

		// Unescape escaped single quote inside quoted string
		if b == '\\' && idx != dataLen-1 && data[idx+1] == '\'' && inQuote != byte(0) {
			// Sometimes Azure double escapes single quotes,
			// we will keep it as is to keep escaping consistent
			// in whole line
			if data[idx-1] != '\\' {
				continue
			}
		}

		// Escape unescaped double quote
		if b == '"' && inQuote != b && idx != 0 && data[idx-1] != '\\' {
			newData.WriteByte('\\')
			newData.WriteByte('"')
			continue
		}

		// All other chars are going to output as-is
		newData.WriteByte(b)
	}

	return newData.Bytes()
}

// convertStringToJSONNumber is a special helper function to mitigate issue with
// invalid JSON in Azure Log Records
// In some cases Azure can put into field, which is expected to be number (int/real),
// invalid data like empty string ("") or even dash ("-")
// This misbehavior was detected at leas in ApplicationGatewayAccessLog
// In any case of non-number input - it will return json.Number("")
func convertStringToJSONNumber(s string) json.Number {
	if s == "" || s == "-" {
		// Actually it's invalid json.Number as it's will return an error on
		// any Int64() or Float64() calls, but that's OK for our case
		return json.Number("")
	}

	// Scan input string for allowed chars that represents numbers
	for _, b := range []byte(s) {
		switch b {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			'.', 'e', 'E', '+', '-':
			continue
		default:
			return json.Number("")
		}
	}

	return json.Number(s)
}
