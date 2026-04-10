// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"net"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func makeHTTP(span ptrace.Span) (map[string]pcommon.Value, *awsxray.HTTPData) {
	var (
		info = awsxray.HTTPData{
			Request:  &awsxray.RequestData{},
			Response: &awsxray.ResponseData{},
		}
		filtered = make(map[string]pcommon.Value)
		urlParts = make(map[string]string)
	)

	if span.Attributes().Len() == 0 {
		return filtered, nil
	}

	hasHTTP := false
	hasHTTPRequestURLAttributes := false
	hasNetPeerAddr := false

	for key, value := range span.Attributes().All() {
		switch key {
		case string("http.method"), string(conventions.HTTPRequestMethodKey):
			// TODO: Remove v0 key handling when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if key == string("http.method") && metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			info.Request.Method = awsxray.String(value.Str())
			hasHTTP = true
		case string("http.client_ip"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			info.Request.ClientIP = awsxray.String(value.Str())
			hasHTTP = true
		case string("http.user_agent"), string(conventions.UserAgentOriginalKey):
			// TODO: Remove v0 key handling when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if key == string("http.user_agent") && metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			info.Request.UserAgent = awsxray.String(value.Str())
			hasHTTP = true
		case string("http.status_code"), string(conventions.HTTPResponseStatusCodeKey):
			// TODO: Remove v0 key handling when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if key == string("http.status_code") && metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			info.Response.Status = aws.Int64(value.Int())
			hasHTTP = true
		case string("http.url"), string(conventions.URLFullKey):
			// TODO: Remove v0 key handling when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if key == string("http.url") && metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[string("http.url")] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case string("http.scheme"), string(conventions.URLSchemeKey):
			// TODO: Remove v0 key handling when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if key == string("http.scheme") && metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[string("http.scheme")] = value.Str()
			hasHTTP = true
		case string("http.host"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case string("http.target"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTP = true
		case string("http.server_name"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case string("net.host.port"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTP = true
			if urlParts[key] == "" {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		case string(conventions.HostNameKey):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string("net.host.name"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string("net.peer.name"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string("net.peer.port"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			if urlParts[key] == "" {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		case string("net.peer.ip"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			// Prefer HTTP forwarded information (AttributeHTTPClientIP) when present.
			if info.Request.ClientIP == nil {
				info.Request.ClientIP = awsxray.String(value.Str())
			}
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
			hasNetPeerAddr = true
		case string(conventions.NetworkPeerAddressKey):
			// Prefer HTTP forwarded information (AttributeHTTPClientIP) when present.
			if net.ParseIP(value.Str()) != nil {
				if info.Request.ClientIP == nil {
					info.Request.ClientIP = awsxray.String(value.Str())
				}
				hasHTTPRequestURLAttributes = true
				hasNetPeerAddr = true
			}
		case string(conventions.NetworkPeerPortKey):
			if metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() {
				urlParts[key] = value.Str()
				if urlParts[key] == "" {
					urlParts[key] = strconv.FormatInt(value.Int(), 10)
				}
			} else {
				filtered[key] = value
			}
		case string(conventions.ClientAddressKey):
			if net.ParseIP(value.Str()) != nil {
				info.Request.ClientIP = awsxray.String(value.Str())
			}
		case string(conventions.URLPathKey):
			urlParts[key] = value.Str()
			hasHTTP = true
		case string(conventions.URLQueryKey):
			urlParts[key] = value.Str()
			hasHTTP = true
		case string(conventions.ServerAddressKey):
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string(conventions.ServerPortKey):
			urlParts[key] = value.Str()
			if urlParts[key] == "" {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		default:
			filtered[key] = value
		}
	}

	if !hasNetPeerAddr && info.Request.ClientIP != nil {
		info.Request.XForwardedFor = aws.Bool(true)
	}

	if !hasHTTP {
		// Didn't have any HTTP-specific information so don't need to fill it in segment
		return filtered, nil
	}

	if hasHTTPRequestURLAttributes {
		if span.Kind() == ptrace.SpanKindServer {
			info.Request.URL = awsxray.String(constructServerURL(urlParts))
		} else {
			info.Request.URL = awsxray.String(constructClientURL(urlParts))
		}
	}

	info.Response.ContentLength = aws.Int64(extractResponseSizeFromEvents(span))

	return filtered, &info
}

func extractResponseSizeFromEvents(span ptrace.Span) int64 {
	// Support instrumentation that sets response size in span or as an event.
	size := extractResponseSizeFromAttributes(span.Attributes())
	if size != 0 {
		return size
	}
	for i := 0; i < span.Events().Len(); i++ {
		event := span.Events().At(i)
		size = extractResponseSizeFromAttributes(event.Attributes())
		if size != 0 {
			return size
		}
	}
	return size
}

func extractResponseSizeFromAttributes(attributes pcommon.Map) int64 {
	typeVal, ok := attributes.Get("message.type")
	if ok && typeVal.Str() == "RECEIVED" {
		// TODO: Remove old key when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
		if !metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
			if sizeVal, ok := attributes.Get(string("messaging.message_payload_size_bytes")); ok {
				return sizeVal.Int()
			}
		}
		if metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() {
			if sizeVal, ok := attributes.Get(string(conventions.MessagingMessageBodySizeKey)); ok {
				return sizeVal.Int()
			}
		}
	}
	return 0
}

func constructClientURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for client spans described in
	// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/http/http-spans.md#http-client

	url, ok := urlParts[string("http.url")]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[string("http.scheme")]
	if !ok {
		scheme = "http"
	}
	port := ""
	// TODO: Remove old-key lookups when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
	host, ok := urlParts[string("http.host")]
	if !ok {
		host, ok = urlParts[string("net.peer.name")]
		if !ok {
			host, ok = urlParts[string("net.peer.ip")]
			if !ok {
				host = urlParts[string(conventions.ServerAddressKey)]
			}
		}
		port, ok = urlParts[string("net.peer.port")]
		if !ok {
			port = urlParts[string(conventions.NetworkPeerPortKey)]
		}
	}
	url = scheme + "://" + host
	if port != "" && (scheme != "http" || port != "80") && (scheme != "https" || port != "443") {
		url += ":" + port
	}
	target, ok := urlParts[string("http.target")]
	if ok {
		url += target
	} else {
		path, ok := urlParts[string(conventions.URLPathKey)]
		if ok {
			url += path
		} else {
			url += "/"
		}
		query, ok := urlParts[string(conventions.URLQueryKey)]
		if ok {
			if !strings.HasPrefix(query, "?") {
				query = "?" + query
			}
			url += query
		}
	}
	return url
}

func constructServerURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for server spans described in
	// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/http/http-spans.md#http-server

	url, ok := urlParts[string("http.url")]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[string("http.scheme")]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[string("http.host")]
	if !ok {
		host, ok = urlParts[string("http.server_name")]
		if !ok {
			host, ok = urlParts[string("net.host.name")]
			if !ok {
				host, ok = urlParts[string(conventions.HostNameKey)]
				if !ok {
					host = urlParts[string(conventions.ServerAddressKey)]
				}
			}
		}
		port, ok = urlParts[string("net.host.port")]
		if !ok {
			port, ok = urlParts[string(conventions.ServerPortKey)]
			if !ok {
				port = ""
			}
		}
	}
	url = scheme + "://" + host
	if port != "" && (scheme != "http" || port != "80") && (scheme != "https" || port != "443") {
		url += ":" + port
	}
	target, ok := urlParts[string("http.target")]
	if ok {
		url += target
	} else {
		path, ok := urlParts[string(conventions.URLPathKey)]
		if ok {
			url += path
		} else {
			url += "/"
		}
		query, ok := urlParts[string(conventions.URLQueryKey)]
		if ok {
			if !strings.HasPrefix(query, "?") {
				query = "?" + query
			}
			url += query
		}
	}
	return url
}
