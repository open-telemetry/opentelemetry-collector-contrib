package azure

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAppServiceHTTPLogsProtocol(t *testing.T) {
	f, ok := tryGetComplexConversion("AppServiceHTTPLogs", "Protocol")
	assert.True(t, ok)
	attrs := map[string]any{}
	ok = f("Protocol", "HTTP/1.1", attrs)
	assert.True(t, ok)
	protocolName, ok := attrs["network.protocol.name"]
	assert.True(t, ok)
	assert.Equal(t, "HTTP", protocolName)
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
	duration, ok := attrs["http.client.request.duration"].(float64)
	assert.True(t, ok)
	assert.Equal(t, 0.123, duration)
}
