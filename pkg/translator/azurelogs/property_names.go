// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"strings"

	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
)

const (
	categoryAzureCdnAccessLog                  = "AzureCdnAccessLog"
	categoryFrontDoorAccessLog                 = "FrontDoorAccessLog"
	categoryFrontDoorHealthProbeLog            = "FrontDoorHealthProbeLog"
	categoryFrontdoorWebApplicationFirewallLog = "FrontdoorWebApplicationFirewallLog"
	categoryAppServiceAppLogs                  = "AppServiceAppLogs"
	categoryAppServiceAuditLogs                = "AppServiceAuditLogs"
	// TODO Add log and expected file to the unit tests for authentication logs
	categoryAppServiceAuthenticationLogs = "AppServiceAuthenticationLogs"
	categoryAppServiceConsoleLogs        = "AppServiceConsoleLogs"
	categoryAppServiceHTTPLogs           = "AppServiceHTTPLogs"
	categoryAppServiceIPSecAuditLogs     = "AppServiceIPSecAuditLogs"
	categoryAppServicePlatformLogs       = "AppServicePlatformLogs"
)

func handleAzureCDNAccessLog(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "BackendHostname":
		attrs[conventions.AttributeDestinationAddress] = value
	case "ClientIp":
		attrs["client.address"] = value
	case "ClientPort":
		// TODO Should be a port
		attrs["client.port"] = value
	case "HttpMethod":
		attrs[conventions.AttributeHTTPRequestMethod] = value
	case "HttpStatusCode":
		attrs[conventions.AttributeHTTPResponseStatusCode] = toInt(value)
	case "HttpVersion":
		attrs[conventions.AttributeNetworkProtocolVersion] = value
	case "RequestBytes":
		attrs[conventions.AttributeHTTPRequestSize] = toInt(value)
	case "RequestUri":
		attrs[conventions.AttributeURLFull] = value
	case "ResponseBytes":
		attrs[conventions.AttributeHTTPResponseSize] = toInt(value)
	case "TrackingReference":
		attrs[conventions.AttributeAzServiceRequestID] = value
	case "UserAgent":
		attrs[conventions.AttributeUserAgentOriginal] = value
	case "ErrorInfo":
		attrs[conventions.AttributeErrorType] = value
	case "SecurityProtocol":
		str, ok := value.(string)
		if !ok {
			return
		}
		name, remaining, _ := strings.Cut(str, " ")
		if name == "" || remaining == "" {
			return
		}
		version, remaining, _ := strings.Cut(remaining, " ")
		if version == "" || remaining != "" {
			return
		}
		attrs[conventions.AttributeTLSProtocolName] = strings.ToLower(name)
		attrs[conventions.AttributeTLSProtocolVersion] = version
	default:
		attrsProps[field] = value
	}
}

func handleFrontDoorAccessLog(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "trackingReference":
		attrs[conventions.AttributeAzServiceRequestID] = value
	case "httpMethod":
		attrs[conventions.AttributeHTTPRequestMethod] = value
	case "httpVersion":
		attrs[conventions.AttributeNetworkProtocolVersion] = value
	case "requestUri":
		attrs[conventions.AttributeURLFull] = value
	case "hostName":
		attrs[conventions.AttributeServerAddress] = value
	case "requestBytes":
		attrs[conventions.AttributeHTTPRequestSize] = toInt(value)
	case "responseBytes":
		attrs[conventions.AttributeHTTPResponseSize] = toInt(value)
	case "userAgent":
		attrs[conventions.AttributeUserAgentOriginal] = value
	case "ClientIp", "clientIp":
		attrs["client.address"] = value
	case "ClientPort", "clientPort":
		// TODO Should be a port
		attrs["client.port"] = value
	case "socketIp":
		attrs[conventions.AttributeNetworkPeerAddress] = value
	case "timeTaken":
		attrs["http.server.request.duration"] = toFloat(value)
	case "requestProtocol":
		attrs[conventions.AttributeNetworkProtocolName] = toLower(value)
	case "securityCipher":
		attrs[conventions.AttributeTLSCipher] = value
	case "securityCurves":
		attrs[conventions.AttributeTLSCurve] = value
	case "httpStatusCode":
		attrs[conventions.AttributeHTTPResponseStatusCode] = toInt(value)
	case "routeName":
		attrs[conventions.AttributeHTTPRoute] = value
	case "referer":
		attrs["http.request.header.referer"] = value
	case "errorInfo":
		attrs[conventions.AttributeErrorType] = value
	case "securityProtocol":
		str, ok := value.(string)
		if !ok {
			return
		}
		name, remaining, _ := strings.Cut(str, " ")
		if name == "" || remaining == "" {
			return
		}
		version, remaining, _ := strings.Cut(remaining, " ")
		if version == "" || remaining != "" {
			return
		}
		attrs[conventions.AttributeTLSProtocolName] = strings.ToLower(name)
		attrs[conventions.AttributeTLSProtocolVersion] = version
	default:
		attrsProps[field] = value
	}
}

func handleFrontDoorHealthProbeLog(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "httpVerb":
		attrs[conventions.AttributeHTTPRequestMethod] = value
	case "httpStatusCode":
		attrs[conventions.AttributeHTTPResponseStatusCode] = toInt(value)
	case "probeURL":
		attrs[conventions.AttributeURLFull] = value
	case "originIP":
		attrs[conventions.AttributeServerAddress] = value
	case "DNSLatencyMicroseconds":
		microseconds, ok := tryParseFloat64(value)
		if !ok {
			return
		}
		seconds := microseconds / 1_000_000
		attrs["dns.lookup.duration"] = seconds
	case "totalLatencyMilliseconds":
		milliseconds, ok := tryParseFloat64(value)
		if !ok {
			return
		}
		seconds := milliseconds / 1_000
		attrs["http.request.duration"] = seconds
	default:
		attrsProps[field] = value
	}
}

func handleFrontdoorWebApplicationFirewallLog(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "clientIP":
		attrs["client.address"] = value
	case "clientPort":
		attrs["client.port"] = value
	case "socketIP":
		attrs[conventions.AttributeNetworkPeerAddress] = value
	case "requestUri":
		attrs[conventions.AttributeURLFull] = value
	case "host":
		attrs[conventions.AttributeServerAddress] = value
	case "trackingReference":
		attrs[conventions.AttributeAzServiceRequestID] = value
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceAppLogs(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "ContainerId":
		attrs[conventions.AttributeContainerID] = value
	case "ExceptionClass":
		attrs[conventions.AttributeExceptionType] = value
	case "Host":
		attrs[conventions.AttributeHostID] = value
	case "Method":
		attrs[conventions.AttributeCodeFunction] = value
	case "Source":
		attrs[conventions.AttributeCodeFilepath] = value
	case "Stacktrace", "StackTrace":
		attrs[conventions.AttributeExceptionStacktrace] = value
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceAuditLogs(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "Protocol":
		attrs[conventions.AttributeNetworkProtocolName] = toLower(value)
	case "User":
		attrs["enduser.id"] = value
	case "UserAddress":
		attrs["client.address"] = value
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceAuthenticationLogs(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "StatusCode":
		attrs[conventions.AttributeHTTPResponseStatusCode] = toInt(value)
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceConsoleLogs(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "ContainerId":
		attrs[conventions.AttributeContainerID] = value
	case "Host":
		attrs[conventions.AttributeHostID] = value
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceHTTPLogs(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "CIp":
		attrs["client.address"] = value
	case "ComputerName":
		attrs[conventions.AttributeHostName] = value
	case "CsBytes":
		attrs[conventions.AttributeHTTPRequestBodySize] = toInt(value)
	case "CsHost":
		attrs[conventions.AttributeURLDomain] = value
	case "CsMethod":
		attrs[conventions.AttributeHTTPRequestMethod] = value
	case "CsUriQuery":
		attrs[conventions.AttributeURLQuery] = value
	case "CsUriStem":
		attrs[conventions.AttributeURLPath] = value
	case "Referer":
		attrs["http.request.header.referer"] = value
	case "ScBytes":
		attrs[conventions.AttributeHTTPResponseBodySize] = toInt(value)
	case "ScStatus":
		attrs[conventions.AttributeHTTPResponseStatusCode] = toInt(value)
	case "SPort":
		attrs[conventions.AttributeServerPort] = toInt(value)
	case "TimeTaken":
		milliseconds, ok := tryParseFloat64(value)
		if !ok {
			return
		}
		seconds := milliseconds / 1_000
		attrs["http.server.request.duration"] = seconds
	case "UserAgent":
		attrs[conventions.AttributeUserAgentOriginal] = value
	case "Protocol":
		str, ok := value.(string)
		if !ok {
			return
		}
		name, remaining, _ := strings.Cut(str, "/")
		if name == "" || remaining == "" {
			return
		}
		version, remaining, _ := strings.Cut(remaining, "/")
		if version == "" || remaining != "" {
			return
		}
		attrs[conventions.AttributeNetworkProtocolName] = strings.ToLower(name)
		attrs[conventions.AttributeNetworkProtocolVersion] = version
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceIPSecAuditLogs(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "CIp":
		attrs["client.address"] = value
	case "CsHost":
		attrs["url.domain"] = value
	case "XAzureFDID":
		attrs["http.request.header.x-azure-fdid"] = value
	case "XFDHealthProbe":
		attrs["http.request.header.x-fd-healthprobe"] = value
	case "XForwardedFor":
		attrs["http.request.header.x-forwarded-for"] = value
	case "XForwardedHost":
		attrs["http.request.header.x-forwarded-host"] = value
	default:
		attrsProps[field] = value
	}
}

func handleAppServicePlatformLogs(field string, value any, attrs map[string]any, attrsProps map[string]any) {
	switch field {
	case "containerId":
		attrs[conventions.AttributeContainerID] = value
	case "containerName":
		attrs[conventions.AttributeContainerName] = value
	case "exception":
		attrs[conventions.AttributeErrorType] = value
	default:
		attrsProps[field] = value
	}
}
