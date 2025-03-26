// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"net"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventionsv112 "go.opentelemetry.io/collector/semconv/v1.12.0"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"

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
		case conventionsv112.AttributeHTTPMethod, conventions.AttributeHTTPRequestMethod:
			info.Request.Method = awsxray.String(value.Str())
			hasHTTP = true
		case conventionsv112.AttributeHTTPClientIP:
			info.Request.ClientIP = awsxray.String(value.Str())
			hasHTTP = true
		case conventionsv112.AttributeHTTPUserAgent, conventions.AttributeUserAgentOriginal:
			info.Request.UserAgent = awsxray.String(value.Str())
			hasHTTP = true
		case conventionsv112.AttributeHTTPStatusCode, conventions.AttributeHTTPResponseStatusCode:
			info.Response.Status = aws.Int64(value.Int())
			hasHTTP = true
		case conventionsv112.AttributeHTTPURL, conventions.AttributeURLFull:
			urlParts[conventionsv112.AttributeHTTPURL] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case conventionsv112.AttributeHTTPScheme, conventions.AttributeURLScheme:
			urlParts[conventionsv112.AttributeHTTPScheme] = value.Str()
			hasHTTP = true
		case conventionsv112.AttributeHTTPHost:
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case conventionsv112.AttributeHTTPTarget, conventions.AttributeURLQuery:
			urlParts[conventionsv112.AttributeHTTPTarget] = value.Str()
			hasHTTP = true
		case conventionsv112.AttributeHTTPServerName:
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case conventionsv112.AttributeNetHostPort:
			urlParts[key] = value.Str()
			hasHTTP = true
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		case conventionsv112.AttributeHostName:
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case conventionsv112.AttributeNetHostName:
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case conventionsv112.AttributeNetPeerName:
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case conventionsv112.AttributeNetPeerPort:
			urlParts[key] = value.Str()
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		case conventionsv112.AttributeNetPeerIP:
			// Prefer HTTP forwarded information (AttributeHTTPClientIP) when present.
			if info.Request.ClientIP == nil {
				info.Request.ClientIP = awsxray.String(value.Str())
			}
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
			hasNetPeerAddr = true
		case conventions.AttributeNetworkPeerAddress:
			// Prefer HTTP forwarded information (AttributeHTTPClientIP) when present.
			if net.ParseIP(value.Str()) != nil {
				if info.Request.ClientIP == nil {
					info.Request.ClientIP = awsxray.String(value.Str())
				}
				hasHTTPRequestURLAttributes = true
				hasNetPeerAddr = true
			}
		case conventions.AttributeClientAddress:
			if net.ParseIP(value.Str()) != nil {
				info.Request.ClientIP = awsxray.String(value.Str())
			}
		case conventions.AttributeURLPath:
			urlParts[key] = value.Str()
			hasHTTP = true
		case conventions.AttributeServerAddress:
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case conventions.AttributeServerPort:
			urlParts[key] = value.Str()
			if len(urlParts[key]) == 0 {
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
		if sizeVal, ok := attributes.Get(conventionsv112.AttributeMessagingMessagePayloadSizeBytes); ok {
			return sizeVal.Int()
		}
	}
	return 0
}

func constructClientURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for client spans described in
	// https://github.com/open-telemetry/semantic-conventionsv112/blob/main/docs/http/http-spans.md#http-client

	url, ok := urlParts[conventionsv112.AttributeHTTPURL]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[conventionsv112.AttributeHTTPScheme]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[conventionsv112.AttributeHTTPHost]
	if !ok {
		host, ok = urlParts[conventionsv112.AttributeNetPeerName]
		if !ok {
			host = urlParts[conventionsv112.AttributeNetPeerIP]
		}
		port, ok = urlParts[conventionsv112.AttributeNetPeerPort]
		if !ok {
			port = ""
		}
	}
	url = scheme + "://" + host
	if len(port) > 0 && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		url += ":" + port
	}
	target, ok := urlParts[conventionsv112.AttributeHTTPTarget]
	if ok {
		url += target
	} else {
		url += "/"
	}
	return url
}

func constructServerURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for server spans described in
	// https://github.com/open-telemetry/semantic-conventionsv112/blob/main/docs/http/http-spans.md#http-server

	url, ok := urlParts[conventionsv112.AttributeHTTPURL]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[conventionsv112.AttributeHTTPScheme]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[conventionsv112.AttributeHTTPHost]
	if !ok {
		host, ok = urlParts[conventionsv112.AttributeHTTPServerName]
		if !ok {
			host, ok = urlParts[conventionsv112.AttributeNetHostName]
			if !ok {
				host, ok = urlParts[conventionsv112.AttributeHostName]
				if !ok {
					host = urlParts[conventions.AttributeServerAddress]
				}
			}
		}
		port, ok = urlParts[conventionsv112.AttributeNetHostPort]
		if !ok {
			port, ok = urlParts[conventions.AttributeServerPort]
			if !ok {
				port = ""
			}
		}
	}
	url = scheme + "://" + host
	if len(port) > 0 && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		url += ":" + port
	}
	target, ok := urlParts[conventionsv112.AttributeHTTPTarget]
	if ok {
		url += target
	} else {
		path, ok := urlParts[conventions.AttributeURLPath]
		if ok {
			url += path
		} else {
			url += "/"
		}
	}
	return url
}
