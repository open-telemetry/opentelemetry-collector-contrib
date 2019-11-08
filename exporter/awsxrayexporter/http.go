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
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"net/http"
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
	XForwardedFor string `json:"x_forwarded_for,omitempty"`

	// Traced – (subsegments only) boolean indicating that the downstream call
	// is to another traced service. If this field is set to true, X-Ray considers
	// the trace to be broken until the downstream service uploads a segment with
	// a parent_id that matches the id of the subsegment that contains this block.
	//
	// TODO - need to understand the impact of this field
	//Traced bool `json:"traced"`
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
	// https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto
	// Status codes for use with Span.SetStatus. These correspond to the status
	// codes used by gRPC defined here: https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto
	switch code {
	case trace.StatusCodeOK:
		return http.StatusOK
	case trace.StatusCodeCancelled:
		return 499 // Client Closed Request
	case trace.StatusCodeUnknown:
		return http.StatusInternalServerError
	case trace.StatusCodeInvalidArgument:
		return http.StatusBadRequest
	case trace.StatusCodeDeadlineExceeded:
		return http.StatusGatewayTimeout
	case trace.StatusCodeNotFound:
		return http.StatusNotFound
	case trace.StatusCodeAlreadyExists:
		return http.StatusConflict
	case trace.StatusCodePermissionDenied:
		return http.StatusForbidden
	case trace.StatusCodeResourceExhausted:
		return http.StatusTooManyRequests
	case trace.StatusCodeFailedPrecondition:
		return http.StatusBadRequest
	case trace.StatusCodeAborted:
		return http.StatusConflict
	case trace.StatusCodeOutOfRange:
		return http.StatusBadRequest
	case trace.StatusCodeUnimplemented:
		return http.StatusNotImplemented
	case trace.StatusCodeInternal:
		return http.StatusInternalServerError
	case trace.StatusCodeUnavailable:
		return http.StatusServiceUnavailable
	case trace.StatusCodeDataLoss:
		return http.StatusInternalServerError
	case trace.StatusCodeUnauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func makeHttp(spanName string, code int32, attributes map[string]interface{}) (map[string]interface{}, *httpInfo) {
	var (
		info     httpInfo
		filtered = map[string]interface{}{}
	)

	for key, value := range attributes {
		switch key {
		case ochttp.MethodAttribute:
			info.Request.Method, _ = value.(string)

		case ochttp.UserAgentAttribute:
			info.Request.UserAgent, _ = value.(string)

		case ochttp.StatusCodeAttribute:
			info.Response.Status, _ = value.(int64)

		default:
			filtered[key] = value
		}
	}

	info.Request.URL = spanName

	if info.Response.Status == 0 {
		// this is a fallback because the ochttp.StatusCodeAttribute isn't being set by opencensus-go
		// https://github.com/census-instrumentation/opencensus-go/issues/899
		info.Response.Status = convertToStatusCode(code)
	}

	if len(filtered) == len(attributes) {
		return attributes, nil
	}

	return filtered, &info
}
