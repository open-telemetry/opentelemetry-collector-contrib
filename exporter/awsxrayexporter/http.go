// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsxrayexporter

import (
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	tracetranslator "github.com/open-telemetry/opentelemetry-collector/translator/trace"
	"net/http"
	"strconv"
)

const (
	// Attributes recorded on the span for the requests.
	// Only trace exporters will need them.
	MethodAttribute     = "http.method"
	URLAttribute        = "http.url"
	TargetAttribute     = "http.target"
	HostAttribute       = "http.host"
	SchemeAttribute     = "http.scheme"
	StatusCodeAttribute = "http.status_code"
	StatusTextAttribute = "http.status_text"
	FlavorAttribute     = "http.flavor"
	ServerNameAttribute = "http.server_name"
	PortAttribute       = "http.port"
	RouteAttribute      = "http.route"
	ClientIpAttribute   = "http.client_ip"
	UserAgentAttribute  = "http.user_agent"
	ContentLenAttribute = "http.resp.content_length"
)

// httpRequest – Information about an http request.
type httpRequest struct {
	// Method – The request method. For example, GET.
	Method string `json:"method,omitempty"`

	// URL – The full URL of the request, compiled from the protocol, hostname,
	// and path of the request.
	URL string `json:"url,omitempty"`

	// UserAgent – The user agent string from the requester's client.
	UserAgent string `json:"user_agent,omitempty"`

	// ClientIP – The IP address of the requester. Can be retrieved from the IP
	// packet's Source Address or, for forwarded requests, from an X-Forwarded-For
	// header.
	ClientIP string `json:"client_ip,omitempty"`

	// XForwardedFor – (segments only) boolean indicating that the client_ip was
	// read from an X-Forwarded-For header and is not reliable as it could have
	// been forged.
	XForwardedFor bool `json:"x_forwarded_for,omitempty"`

	// Traced – (subsegments only) boolean indicating that the downstream call
	// is to another traced service. If this field is set to true, X-Ray considers
	// the trace to be broken until the downstream service uploads a segment with
	// a parent_id that matches the id of the subsegment that contains this block.
	Traced bool `json:"traced,omitempty"`
}

// httpResponse - Information about an http response.
type httpResponse struct {
	// Status – number indicating the HTTP status of the response.
	Status int64 `json:"status,omitempty"`

	// ContentLength – number indicating the length of the response body in bytes.
	ContentLength int64 `json:"content_length,omitempty"`
}

type httpInfo struct {
	Request  httpRequest  `json:"request"`
	Response httpResponse `json:"response"`
}

func convertToStatusCode(code int32) int64 {
	switch code {
	case tracetranslator.OCOK:
		return http.StatusOK
	case tracetranslator.OCCancelled:
		return 499 // Client Closed Request
	case tracetranslator.OCUnknown:
		return http.StatusInternalServerError
	case tracetranslator.OCInvalidArgument:
		return http.StatusBadRequest
	case tracetranslator.OCDeadlineExceeded:
		return http.StatusGatewayTimeout
	case tracetranslator.OCNotFound:
		return http.StatusNotFound
	case tracetranslator.OCAlreadyExists:
		return http.StatusConflict
	case tracetranslator.OCPermissionDenied:
		return http.StatusForbidden
	case tracetranslator.OCResourceExhausted:
		return http.StatusTooManyRequests
	case tracetranslator.OCFailedPrecondition:
		return http.StatusBadRequest
	case tracetranslator.OCAborted:
		return http.StatusConflict
	case tracetranslator.OCOutOfRange:
		return http.StatusBadRequest
	case tracetranslator.OCUnimplemented:
		return http.StatusNotImplemented
	case tracetranslator.OCInternal:
		return http.StatusInternalServerError
	case tracetranslator.OCUnavailable:
		return http.StatusServiceUnavailable
	case tracetranslator.OCDataLoss:
		return http.StatusInternalServerError
	case tracetranslator.OCUnauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func makeHttp(spanKind tracepb.Span_SpanKind, code int32, attributes map[string]*tracepb.AttributeValue) (map[string]string, *httpInfo) {
	var (
		info     httpInfo
		filtered = make(map[string]string)
		urlParts = make(map[string]string)
	)

	for key, value := range attributes {
		switch key {
		case MethodAttribute:
			info.Request.Method = value.GetStringValue().GetValue()
		case UserAgentAttribute:
			info.Request.UserAgent = value.GetStringValue().GetValue()
		case ClientIpAttribute:
			info.Request.ClientIP = value.GetStringValue().GetValue()
			info.Request.XForwardedFor = true
		case StatusCodeAttribute:
			info.Response.Status = value.GetIntValue()
		case URLAttribute:
			urlParts[key] = value.GetStringValue().GetValue()
		case SchemeAttribute:
			urlParts[key] = value.GetStringValue().GetValue()
		case HostAttribute:
			urlParts[key] = value.GetStringValue().GetValue()
		case TargetAttribute:
			urlParts[key] = value.GetStringValue().GetValue()
		case ServerNameAttribute:
			urlParts[key] = value.GetStringValue().GetValue()
		case HostNameAttribute:
			urlParts[key] = value.GetStringValue().GetValue()
		case PortAttribute:
			urlParts[key] = value.GetStringValue().GetValue()
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.GetIntValue(), 10)
			}
		case PeerHostAttribute:
			urlParts[key] = value.GetStringValue().GetValue()
		case PeerPortAttribute:
			urlParts[key] = value.GetStringValue().GetValue()
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.GetIntValue(), 10)
			}
		case PeerIpv4Attribute:
			urlParts[key] = value.GetStringValue().GetValue()
		case PeerIpv6Attribute:
			urlParts[key] = value.GetStringValue().GetValue()
		case ContentLenAttribute:
			info.Response.ContentLength = value.GetIntValue()
		default:
			filtered[key] = value.GetStringValue().GetValue()
		}
	}

	if tracepb.Span_SERVER == spanKind {
		info.Request.URL = constructServerUrl(urlParts)
	} else {
		info.Request.URL = constructClientUrl(urlParts)
	}

	if info.Response.Status == 0 {
		info.Response.Status = convertToStatusCode(code)
	}

	return filtered, &info
}

func constructClientUrl(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for client spans described in
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md
	url, ok := urlParts[URLAttribute]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[SchemeAttribute]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[HostAttribute]
	if !ok {
		host, ok = urlParts[PeerHostAttribute]
		if !ok {
			host, ok = urlParts[PeerIpv4Attribute]
			if !ok {
				host = urlParts[PeerIpv6Attribute]
			}
		}
		port, ok = urlParts[PeerPortAttribute]
		if !ok {
			port = ""
		}
	}
	url = scheme + "://" + host
	if len(port) > 0 && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		url += ":" + port
	}
	target, ok := urlParts[TargetAttribute]
	if ok {
		url += target
	} else {
		url += "/"
	}
	return url
}

func constructServerUrl(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for server spans described in
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md
	url, ok := urlParts[URLAttribute]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[SchemeAttribute]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[HostAttribute]
	if !ok {
		host, ok = urlParts[ServerNameAttribute]
		if !ok {
			host, ok = urlParts[HostNameAttribute]
		}
		port, ok = urlParts[PortAttribute]
		if !ok {
			port = ""
		}
	}
	url = scheme + "://" + host
	if len(port) > 0 && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		url += ":" + port
	}
	target, ok := urlParts[TargetAttribute]
	if ok {
		url += target
	} else {
		url += "/"
	}
	return url
}
