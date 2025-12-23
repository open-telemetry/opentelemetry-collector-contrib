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
	conventionsv112 "go.opentelemetry.io/otel/semconv/v1.12.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

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
		case string(conventionsv112.HTTPMethodKey), string(conventions.HTTPRequestMethodKey):
			info.Request.Method = awsxray.String(value.Str())
			hasHTTP = true
		case string(conventionsv112.HTTPClientIPKey):
			info.Request.ClientIP = awsxray.String(value.Str())
			hasHTTP = true
		case string(conventionsv112.HTTPUserAgentKey), string(conventions.UserAgentOriginalKey):
			info.Request.UserAgent = awsxray.String(value.Str())
			hasHTTP = true
		case string(conventionsv112.HTTPStatusCodeKey), string(conventions.HTTPResponseStatusCodeKey):
			info.Response.Status = aws.Int64(value.Int())
			hasHTTP = true
		case string(conventionsv112.HTTPURLKey), string(conventions.URLFullKey):
			urlParts[string(conventionsv112.HTTPURLKey)] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case string(conventionsv112.HTTPSchemeKey), string(conventions.URLSchemeKey):
			urlParts[string(conventionsv112.HTTPSchemeKey)] = value.Str()
			hasHTTP = true
		case string(conventionsv112.HTTPHostKey):
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case string(conventionsv112.HTTPTargetKey):
			urlParts[key] = value.Str()
			hasHTTP = true
		case string(conventionsv112.HTTPServerNameKey):
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case string(conventionsv112.NetHostPortKey):
			urlParts[key] = value.Str()
			hasHTTP = true
			if urlParts[key] == "" {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		case string(conventionsv112.HostNameKey):
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string(conventionsv112.NetHostNameKey):
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string(conventionsv112.NetPeerNameKey):
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string(conventionsv112.NetPeerPortKey):
			urlParts[key] = value.Str()
			if urlParts[key] == "" {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		case string(conventionsv112.NetPeerIPKey):
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
		if sizeVal, ok := attributes.Get(string(conventionsv112.MessagingMessagePayloadSizeBytesKey)); ok {
			return sizeVal.Int()
		}
	}
	return 0
}

func constructClientURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for client spans described in
	// https://github.com/open-telemetry/semantic-conventionsv112/blob/main/docs/http/http-spans.md#http-client

	url, ok := urlParts[string(conventionsv112.HTTPURLKey)]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[string(conventionsv112.HTTPSchemeKey)]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[string(conventionsv112.HTTPHostKey)]
	if !ok {
		host, ok = urlParts[string(conventionsv112.NetPeerNameKey)]
		if !ok {
			host = urlParts[string(conventionsv112.NetPeerIPKey)]
		}
		port, ok = urlParts[string(conventionsv112.NetPeerPortKey)]
		if !ok {
			port = ""
		}
	}
	url = scheme + "://" + host
	if port != "" && (scheme != "http" || port != "80") && (scheme != "https" || port != "443") {
		url += ":" + port
	}
	target, ok := urlParts[string(conventionsv112.HTTPTargetKey)]
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
	// https://github.com/open-telemetry/semantic-conventionsv112/blob/main/docs/http/http-spans.md#http-server

	url, ok := urlParts[string(conventionsv112.HTTPURLKey)]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[string(conventionsv112.HTTPSchemeKey)]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[string(conventionsv112.HTTPHostKey)]
	if !ok {
		host, ok = urlParts[string(conventionsv112.HTTPServerNameKey)]
		if !ok {
			host, ok = urlParts[string(conventionsv112.NetHostNameKey)]
			if !ok {
				host, ok = urlParts[string(conventionsv112.HostNameKey)]
				if !ok {
					host = urlParts[string(conventions.ServerAddressKey)]
				}
			}
		}
		port, ok = urlParts[string(conventionsv112.NetHostPortKey)]
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
	target, ok := urlParts[string(conventionsv112.HTTPTargetKey)]
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
