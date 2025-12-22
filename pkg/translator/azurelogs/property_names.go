// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"strings"

	conventionsv128 "go.opentelemetry.io/otel/semconv/v1.28.0"
	conventionsv134 "go.opentelemetry.io/otel/semconv/v1.34.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
)

// TODO @constanca-m remove this file once the logic for the remaining categories
// is added to category_logs.go

func handleFrontDoorAccessLog(field string, value any, attrs, attrsProps map[string]any) {
	switch field {
	case "trackingReference":
		attrs[string(conventionsv134.AzServiceRequestIDKey)] = value
	case "httpMethod":
		attrs[string(conventions.HTTPRequestMethodKey)] = value
	case "httpVersion":
		attrs[string(conventions.NetworkProtocolVersionKey)] = value
	case "requestUri":
		attrs[string(conventions.URLFullKey)] = value
	case "hostName":
		attrs[string(conventions.ServerAddressKey)] = value
	case "requestBytes":
		attrs[string(conventions.HTTPRequestSizeKey)] = toInt(value)
	case "responseBytes":
		attrs[string(conventions.HTTPResponseSizeKey)] = toInt(value)
	case "userAgent":
		attrs[string(conventions.UserAgentOriginalKey)] = value
	case "ClientIp", "clientIp":
		attrs["client.address"] = value
	case "ClientPort", "clientPort":
		// TODO Should be a port
		attrs["client.port"] = value
	case "socketIp":
		attrs[string(conventions.NetworkPeerAddressKey)] = value
	case "timeTaken":
		attrs["http.server.request.duration"] = toFloat(value)
	case "requestProtocol":
		attrs[string(conventions.NetworkProtocolNameKey)] = toLower(value)
	case "securityCipher":
		attrs[string(conventions.TLSCipherKey)] = value
	case "securityCurves":
		attrs[string(conventions.TLSCurveKey)] = value
	case "httpStatusCode":
		attrs[string(conventions.HTTPResponseStatusCodeKey)] = toInt(value)
	case "routeName":
		attrs[string(conventions.HTTPRouteKey)] = value
	case "referer":
		attrs["http.request.header.referer"] = value
	case "errorInfo":
		attrs[string(conventions.ErrorTypeKey)] = value
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
		attrs[string(conventions.TLSProtocolNameKey)] = strings.ToLower(name)
		attrs[string(conventions.TLSProtocolVersionKey)] = version
	default:
		attrsProps[field] = value
	}
}

func handleFrontDoorHealthProbeLog(field string, value any, attrs, attrsProps map[string]any) {
	switch field {
	case "httpVerb":
		attrs[string(conventions.HTTPRequestMethodKey)] = value
	case "httpStatusCode":
		attrs[string(conventions.HTTPResponseStatusCodeKey)] = toInt(value)
	case "probeURL":
		attrs[string(conventions.URLFullKey)] = value
	case "originIP":
		attrs[string(conventions.ServerAddressKey)] = value
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

func handleAppServiceAppLogs(field string, value any, attrs, attrsProps map[string]any) {
	switch field {
	case "ContainerId":
		attrs[string(conventions.ContainerIDKey)] = value
	case "ExceptionClass":
		attrs[string(conventions.ExceptionTypeKey)] = value
	case "Host":
		attrs[string(conventions.HostIDKey)] = value
	case "Method":
		attrs[string(conventionsv128.CodeFunctionKey)] = value
	case "Source":
		attrs[string(conventionsv128.CodeFilepathKey)] = value
	case "Stacktrace", "StackTrace":
		attrs[string(conventions.ExceptionStacktraceKey)] = value
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceAuditLogs(field string, value any, attrs, attrsProps map[string]any) {
	switch field {
	case "Protocol":
		attrs[string(conventions.NetworkProtocolNameKey)] = toLower(value)
	case "User":
		attrs["enduser.id"] = value
	case "UserAddress":
		attrs["client.address"] = value
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceAuthenticationLogs(field string, value any, attrs, attrsProps map[string]any) {
	switch field {
	case "StatusCode":
		attrs[string(conventions.HTTPResponseStatusCodeKey)] = toInt(value)
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceConsoleLogs(field string, value any, attrs, attrsProps map[string]any) {
	switch field {
	case "ContainerId":
		attrs[string(conventions.ContainerIDKey)] = value
	case "Host":
		attrs[string(conventions.HostIDKey)] = value
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceHTTPLogs(field string, value any, attrs, attrsProps map[string]any) {
	switch field {
	case "CIp":
		attrs["client.address"] = value
	case "ComputerName":
		attrs[string(conventions.HostNameKey)] = value
	case "CsBytes":
		attrs[string(conventions.HTTPRequestBodySizeKey)] = toInt(value)
	case "CsHost":
		attrs[string(conventions.URLDomainKey)] = value
	case "CsMethod":
		attrs[string(conventions.HTTPRequestMethodKey)] = value
	case "CsUriQuery":
		attrs[string(conventions.URLQueryKey)] = value
	case "CsUriStem":
		attrs[string(conventions.URLPathKey)] = value
	case "Referer":
		attrs["http.request.header.referer"] = value
	case "ScBytes":
		attrs[string(conventions.HTTPResponseBodySizeKey)] = toInt(value)
	case "ScStatus":
		attrs[string(conventions.HTTPResponseStatusCodeKey)] = toInt(value)
	case "SPort":
		attrs[string(conventions.ServerPortKey)] = toInt(value)
	case "TimeTaken":
		milliseconds, ok := tryParseFloat64(value)
		if !ok {
			return
		}
		seconds := milliseconds / 1_000
		attrs["http.server.request.duration"] = seconds
	case "UserAgent":
		attrs[string(conventions.UserAgentOriginalKey)] = value
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
		attrs[string(conventions.NetworkProtocolNameKey)] = strings.ToLower(name)
		attrs[string(conventions.NetworkProtocolVersionKey)] = version
	default:
		attrsProps[field] = value
	}
}

func handleAppServiceIPSecAuditLogs(field string, value any, attrs, attrsProps map[string]any) {
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

func handleAppServicePlatformLogs(field string, value any, attrs, attrsProps map[string]any) {
	switch field {
	case "containerId":
		attrs[string(conventions.ContainerIDKey)] = value
	case "containerName":
		attrs[string(conventions.ContainerNameKey)] = value
	case "exception":
		attrs[string(conventions.ErrorTypeKey)] = value
	default:
		attrsProps[field] = value
	}
}
