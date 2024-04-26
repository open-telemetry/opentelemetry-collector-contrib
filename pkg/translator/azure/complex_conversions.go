package azure

import (
	"fmt"
	"strconv"
	"strings"
)

type ComplexConversion func(string, any, map[string]any) bool
type TypeConversion func(string, any, map[string]any, string) bool

var conversions = map[string]ComplexConversion{
	"AzureCDNAccessLog:SecurityProtocol":               azureCDNAccessLogSecurityProtocol,
	"FrontDoorAccessLog:securityProtocol":              azureCDNAccessLogSecurityProtocol,
	"AppServiceHTTPLogs:Protocol":                      appServiceHTTPLogsProtocol,
	"AppServiceHTTPLogs:TimeTaken":                     appServiceHTTPLogTimeTakenMilliseconds,
	"FrontDoorHealthProbeLog:DNSLatencyMicroseconds":   frontDoorHealthProbeLogDNSLatencyMicroseconds,
	"FrontDoorHealthProbeLog:totalLatencyMilliseconds": frontDoorHealthProbeLogTotalLatencyMilliseconds,
}

// Splits the "TLS 1.2" value into "TLS" and "1.2" and sets as "network.protocol.name" and "network.protocol.version"
func azureCDNAccessLogSecurityProtocol(key string, value any, attrs map[string]any) bool {
	if str, ok := value.(string); ok {
		if parts := strings.SplitN(str, " ", 2); len(parts) == 2 {
			attrs["tls.protocol.name"] = strings.ToLower(parts[0])
			attrs["tls.protocol.version"] = parts[1]
			return true
		}
	}
	return false
}

// Splits the "HTTP/1.1" value into "HTTP" and "1.1" and sets as "network.protocol.name" and "network.protocol.version"
func appServiceHTTPLogsProtocol(key string, value any, attrs map[string]any) bool {
	if str, ok := value.(string); ok {
		if parts := strings.SplitN(str, "/", 2); len(parts) == 2 {
			attrs["network.protocol.name"] = strings.ToLower(parts[0])
			attrs["network.protocol.version"] = parts[1]
			return true
		}
	}
	return false
}

// Converts Microseconds value to Seconds and sets as "dns.lookup.duration"
func frontDoorHealthProbeLogDNSLatencyMicroseconds(key string, value any, attrs map[string]any) bool {
	microseconds, ok := tryParseFloat64(value)
	if !ok {
		return false
	}
	seconds := microseconds / 1_000_000
	attrs["dns.lookup.duration"] = seconds
	return true
}

// Converts Milliseconds value to Seconds and sets as "http.client.request.duration"
func frontDoorHealthProbeLogTotalLatencyMilliseconds(key string, value any, attrs map[string]any) bool {
	milliseconds, ok := tryParseFloat64(value)
	if !ok {
		return false
	}
	seconds := milliseconds / 1_000
	attrs["http.client.request.duration"] = seconds
	return true
}

// Converts Milliseconds value to Seconds and sets as "http.server.request.duration"
func appServiceHTTPLogTimeTakenMilliseconds(key string, value any, attrs map[string]any) bool {
	milliseconds, ok := tryParseFloat64(value)
	if !ok {
		return false
	}
	seconds := milliseconds / 1_000
	attrs["http.server.request.duration"] = seconds
	return true
}

func tryParseFloat64(value any) (float64, bool) {
	switch value.(type) {
	case float32:
		return float64(value.(float32)), true
	case float64:
		return value.(float64), true
	case int:
		return float64(value.(int)), true
	case int32:
		return float64(value.(int32)), true
	case int64:
		return float64(value.(int64)), true
	case string:
		f, err := strconv.ParseFloat(value.(string), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func tryGetComplexConversion(category string, propertyName string) (ComplexConversion, bool) {
	key := fmt.Sprintf("%s:%s", category, propertyName)
	conversion, ok := conversions[key]
	return conversion, ok
}
