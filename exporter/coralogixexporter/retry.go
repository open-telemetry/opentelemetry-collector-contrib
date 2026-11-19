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

// dropReason classifies permanent errors that count toward the client-side drop window.
type dropReason int32

const (
	dropReasonNone dropReason = iota
	dropReasonAuth
	dropReasonQuota
)

func (r dropReason) String() string {
	switch r {
	case dropReasonAuth:
		return "authentication"
	case dropReasonQuota:
		return "quota"
	default:
		return "none"
	}
}

// shouldRetry returns true if the error should be retried.
// The second return value is the dropReason that counts toward the drop window when
// the error will not go away by retrying (authentication or quota failures).
func shouldRetry(code codes.Code, retryInfo *errdetails.RetryInfo) (bool, dropReason) {
	switch code {
	case codes.Canceled,
		codes.DeadlineExceeded,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unavailable,
		codes.DataLoss:
		return true, dropReasonNone
	case codes.ResourceExhausted:
		// Retry only if RetryInfo was supplied by the server.
		// This indicates that the server can still recover from resource exhaustion.
		if retryInfo != nil {
			return true, dropReasonNone
		}
		return false, dropReasonQuota
	case codes.Unauthenticated, codes.PermissionDenied:
		return false, dropReasonAuth
	default:
		return false, dropReasonNone
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
// The second return value is the dropReason that counts toward the drop window.
func shouldRetryHTTP(statusCode int) (bool, dropReason) {
	switch statusCode {
	case http.StatusOK:
		return false, dropReasonNone
	case http.StatusBadRequest,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusUnprocessableEntity:
		// Permanent errors - don't retry
		return false, dropReasonNone
	case http.StatusUnauthorized,
		http.StatusForbidden:
		// Don't retry; count toward the drop window as authentication
		return false, dropReasonAuth
	case http.StatusRequestTimeout,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		// Retryable errors
		return true, dropReasonNone
	case http.StatusTooManyRequests:
		// Retry with throttle
		return true, dropReasonNone
	default:
		return false, dropReasonNone
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
