// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"net/http"
	"strconv"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// shouldRetry returns true if the error should be retried.
// The second return value indicates whether the error should trigger a stop in retries by flagging
// the rate limiting mechanism, since these errors (like authentication or quota failures) indicate a problem
// that won't be fixed just by retrying.
func shouldRetry(code codes.Code, retryInfo *errdetails.RetryInfo) (bool, bool) {
	switch code {
	case codes.Canceled,
		codes.DeadlineExceeded,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unavailable,
		codes.DataLoss:
		return true, false
	case codes.ResourceExhausted:
		// Retry only if RetryInfo was supplied by the server.
		// This indicates that the server can still recover from resource exhaustion.
		return retryInfo != nil, retryInfo == nil
	case codes.Unauthenticated, codes.PermissionDenied:
		return false, true
	default:
		return false, false
	}
}

func getRetryInfo(status *status.Status) *errdetails.RetryInfo {
	for _, detail := range status.Details() {
		if t, ok := detail.(*errdetails.RetryInfo); ok {
			return t
		}
	}
	return nil
}

func getThrottleDuration(t *errdetails.RetryInfo) time.Duration {
	if t == nil || t.RetryDelay == nil {
		return 0
	}
	if t.RetryDelay.Seconds > 0 || t.RetryDelay.Nanos > 0 {
		return time.Duration(t.RetryDelay.Seconds)*time.Second + time.Duration(t.RetryDelay.Nanos)*time.Nanosecond
	}
	return 0
}

// shouldRetryHTTP returns true if the HTTP status code should be retried.
// The second return value indicates whether the error should trigger a stop in retries by flagging
// the rate limiting mechanism.
func shouldRetryHTTP(statusCode int) (bool, bool) {
	switch statusCode {
	case http.StatusOK:
		return false, false
	case http.StatusBadRequest,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusUnprocessableEntity:
		// Permanent errors - don't retry
		return false, false
	case http.StatusUnauthorized,
		http.StatusForbidden:
		// Don't retry, but flag for rate limiting
		return false, true
	case http.StatusRequestTimeout,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		// Retryable errors
		return true, false
	case http.StatusTooManyRequests:
		// Retry with throttle
		return true, false
	default:
		return false, false
	}
}

// getHTTPThrottleDuration extracts retry delay from Retry-After header for HTTP 429 responses
func getHTTPThrottleDuration(statusCode int, headers http.Header) time.Duration {
	if statusCode != http.StatusTooManyRequests {
		return 0
	}

	retryAfter := headers.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try parsing as seconds (integer)
	if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP date
	if retryTime, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		duration := time.Until(retryTime)
		if duration > 0 {
			return duration
		}
	}

	return 0
}
