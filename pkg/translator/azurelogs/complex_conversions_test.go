// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFrontDoorAccessLogSecurityProtocol(t *testing.T) {
	f, ok := tryGetComplexConversion("FrontDoorAccessLog", "securityProtocol")
	assert.True(t, ok)
	attrs := map[string]any{}
	ok = f("securityProtocol", "TLS 1.2", attrs)
	assert.True(t, ok)
	protocolName, ok := attrs["tls.protocol.name"]
	assert.True(t, ok)
	// Protocol name is normalized to lower case
	assert.Equal(t, "tls", protocolName)
	protocolVersion, ok := attrs["tls.protocol.version"]
	assert.True(t, ok)
	assert.Equal(t, "1.2", protocolVersion)
}

func TestAzureCDNAccessLogSecurityProtocol(t *testing.T) {
	f, ok := tryGetComplexConversion("AzureCdnAccessLog", "SecurityProtocol")
	assert.True(t, ok)
	attrs := map[string]any{}
	ok = f("SecurityProtocol", "TLS 1.2", attrs)
	assert.True(t, ok)
	protocolName, ok := attrs["tls.protocol.name"]
	assert.True(t, ok)
	// Protocol name is normalized to lower case
	assert.Equal(t, "tls", protocolName)
	protocolVersion, ok := attrs["tls.protocol.version"]
	assert.True(t, ok)
	assert.Equal(t, "1.2", protocolVersion)
}

func TestAppServiceHTTPLogsProtocol(t *testing.T) {
	f, ok := tryGetComplexConversion("AppServiceHTTPLogs", "Protocol")
	assert.True(t, ok)
	attrs := map[string]any{}
	ok = f("Protocol", "HTTP/1.1", attrs)
	assert.True(t, ok)
	protocolName, ok := attrs["network.protocol.name"]
	assert.True(t, ok)
	assert.Equal(t, "http", protocolName)
	protocolVersion, ok := attrs["network.protocol.version"]
	assert.True(t, ok)
	assert.Equal(t, "1.1", protocolVersion)
}

func TestFrontDoorHealthProbeLogDNSLatencyMicroseconds(t *testing.T) {
	f, ok := tryGetComplexConversion("FrontDoorHealthProbeLog", "DNSLatencyMicroseconds")
	assert.True(t, ok)
	attrs := map[string]any{}
	ok = f("DNSLatencyMicroseconds", 123456, attrs)
	assert.True(t, ok)
	duration, ok := attrs["dns.lookup.duration"].(float64)
	assert.True(t, ok)
	assert.Equal(t, 0.123456, duration)
}

func TestFrontDoorHealthProbeLogTotalLatencyMilliseconds(t *testing.T) {
	f, ok := tryGetComplexConversion("FrontDoorHealthProbeLog", "totalLatencyMilliseconds")
	assert.True(t, ok)
	attrs := map[string]any{}
	ok = f("totalLatencyMilliseconds", 123, attrs)
	assert.True(t, ok)
	duration, ok := attrs["http.request.duration"].(float64)
	assert.True(t, ok)
	assert.Equal(t, 0.123, duration)
}
