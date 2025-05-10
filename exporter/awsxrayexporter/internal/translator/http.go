// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"net"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.12.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

const (
	AttributeHTTPRequestMethod      = "http.request.method"
	AttributeHTTPResponseStatusCode = "http.response.status_code"
	AttributeServerAddress          = "server.address"
	AttributeServerPort             = "server.port"
	AttributeNetworkPeerAddress     = "network.peer.address"
	AttributeClientAddress          = "client.address"
	AttributeURLScheme              = "url.scheme"
	AttributeURLFull                = "url.full"
	AttributeURLPath                = "url.path"
	AttributeUserAgentOriginal      = "user_agent.original"
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

	span.Attributes().Range(func(key string, value pcommon.Value) bool {
		switch key {
		case conventions.AttributeHTTPMethod, AttributeHTTPRequestMethod:
			info.Request.Method = awsxray.String(value.Str())
			hasHTTP = true
		case conventions.AttributeHTTPClientIP:
			info.Request.ClientIP = awsxray.String(value.Str())
			hasHTTP = true
		case conventions.AttributeHTTPUserAgent, AttributeUserAgentOriginal:
			info.Request.UserAgent = awsxray.String(value.Str())
			hasHTTP = true
		case conventions.AttributeHTTPStatusCode, AttributeHTTPResponseStatusCode:
			info.Response.Status = aws.Int64(value.Int())
			hasHTTP = true
		case conventions.AttributeHTTPURL, AttributeURLFull:
			urlParts[conventions.AttributeHTTPURL] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case conventions.AttributeHTTPScheme, AttributeURLScheme:
			urlParts[conventions.AttributeHTTPScheme] = value.Str()
			hasHTTP = true
		case conventions.AttributeHTTPHost:
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case conventions.AttributeHTTPTarget:
			urlParts[key] = value.Str()
			hasHTTP = true
		case conventions.AttributeHTTPServerName:
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case conventions.AttributeNetHostPort:
			urlParts[key] = value.Str()
			hasHTTP = true
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		case conventions.AttributeHostName:
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case conventions.AttributeNetHostName:
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case conventions.AttributeNetPeerName:
			urlParts[key] = value.Str()
		case conventions.AttributeNetPeerPort:
			urlParts[key] = value.Str()
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		case conventions.AttributeNetPeerIP:
			// Prefer HTTP forwarded information (AttributeHTTPClientIP) when present.
			if info.Request.ClientIP == nil {
				info.Request.ClientIP = awsxray.String(value.Str())
			}
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
			hasNetPeerAddr = true
		case AttributeNetworkPeerAddress:
			// Prefer HTTP forwarded information (AttributeHTTPClientIP) when present.
			if net.ParseIP(value.Str()) != nil {
				if info.Request.ClientIP == nil {
					info.Request.ClientIP = awsxray.String(value.Str())
				}
				hasHTTPRequestURLAttributes = true
				hasNetPeerAddr = true
			}
		case AttributeClientAddress:
			if net.ParseIP(value.Str()) != nil {
				info.Request.ClientIP = awsxray.String(value.Str())
			}
		case AttributeURLPath:
			urlParts[key] = value.Str()
			hasHTTP = true
		case AttributeServerAddress:
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case AttributeServerPort:
			urlParts[key] = value.Str()
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		default:
			filtered[key] = value
		}
		return true
	})

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
	// Support insrumentation that sets response size in span or as an event.
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
		if sizeVal, ok := attributes.Get(conventions.AttributeMessagingMessagePayloadSizeBytes); ok {
			return sizeVal.Int()
		}
	}
	return 0
}

func constructClientURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for client spans described in
	// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/http/http-spans.md#http-client

	url, ok := urlParts[conventions.AttributeHTTPURL]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[conventions.AttributeHTTPScheme]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[conventions.AttributeHTTPHost]
	if !ok {
		host, ok = urlParts[conventions.AttributeNetPeerName]
		if !ok {
			host = urlParts[conventions.AttributeNetPeerIP]
		}
		port, ok = urlParts[conventions.AttributeNetPeerPort]
		if !ok {
			port = ""
		}
	}
	url = scheme + "://" + host
	if len(port) > 0 && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		url += ":" + port
	}
	target, ok := urlParts[conventions.AttributeHTTPTarget]
	if ok {
		url += target
	} else {
		url += "/"
	}
	return url
}

func constructServerURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for server spans described in
	// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/http/http-spans.md#http-server

	url, ok := urlParts[conventions.AttributeHTTPURL]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[conventions.AttributeHTTPScheme]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[conventions.AttributeHTTPHost]
	if !ok {
		host, ok = urlParts[conventions.AttributeHTTPServerName]
		if !ok {
			host, ok = urlParts[conventions.AttributeNetHostName]
			if !ok {
				host, ok = urlParts[conventions.AttributeHostName]
				if !ok {
					host = urlParts[AttributeServerAddress]
				}
			}
		}
		port, ok = urlParts[conventions.AttributeNetHostPort]
		if !ok {
			port, ok = urlParts[AttributeServerPort]
			if !ok {
				port = ""
			}
		}
	}
	url = scheme + "://" + host
	if len(port) > 0 && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		url += ":" + port
	}
	target, ok := urlParts[conventions.AttributeHTTPTarget]
	if ok {
		url += target
	} else {
		path, ok := urlParts[AttributeURLPath]
		if ok {
			url += path
		} else {
			url += "/"
		}
	}
	return url
}
