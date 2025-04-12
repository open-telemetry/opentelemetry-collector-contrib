// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter"

import (
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newStatusFromMsgAndHTTPCode(errMsg string, statusCode int) *status.Status {
	var c codes.Code
	// Mapping based on https://github.com/grpc/grpc/blob/master/doc/http-grpc-status-mapping.md
	// 429 mapping to ResourceExhausted and 400 mapping to StatusBadRequest are exceptions.
	switch statusCode {
	case http.StatusBadRequest:
		c = codes.InvalidArgument
	case http.StatusUnauthorized:
		c = codes.Unauthenticated
	case http.StatusForbidden:
		c = codes.PermissionDenied
	case http.StatusNotFound:
		c = codes.Unimplemented
	case http.StatusTooManyRequests:
		c = codes.ResourceExhausted
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		c = codes.Unavailable
	default:
		c = codes.Unknown
	}
	return status.New(c, errMsg)
}

func isRetryableStatusCode(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
