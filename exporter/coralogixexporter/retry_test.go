// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name             string
		code             codes.Code
		retryInfo        *errdetails.RetryInfo
		expected         bool
		expectedThrottle bool
	}{
		{
			name:             "Canceled is retryable",
			code:             codes.Canceled,
			retryInfo:        nil,
			expected:         true,
			expectedThrottle: false,
		},
		{
			name:             "DeadlineExceeded is retryable",
			code:             codes.DeadlineExceeded,
			retryInfo:        nil,
			expected:         true,
			expectedThrottle: false,
		},
		{
			name:             "Unavailable is retryable",
			code:             codes.Unavailable,
			retryInfo:        nil,
			expected:         true,
			expectedThrottle: false,
		},
		{
			name:             "ResourceExhausted without retry info is not retryable",
			code:             codes.ResourceExhausted,
			retryInfo:        nil,
			expected:         false,
			expectedThrottle: true,
		},
		{
			name: "ResourceExhausted with retry info is retryable",
			code: codes.ResourceExhausted,
			retryInfo: &errdetails.RetryInfo{
				RetryDelay: durationpb.New(time.Second),
			},
			expected:         true,
			expectedThrottle: false,
		},
		{
			name:             "InvalidArgument is not retryable",
			code:             codes.InvalidArgument,
			retryInfo:        nil,
			expected:         false,
			expectedThrottle: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, isThrottle := shouldRetry(tt.code, tt.retryInfo)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedThrottle, isThrottle)
		})
	}
}

func TestGetRetryInfo(t *testing.T) {
	tests := []struct {
		name     string
		status   *status.Status
		expected *errdetails.RetryInfo
	}{
		{
			name:     "status without retry info returns nil",
			status:   status.New(codes.OK, "ok"),
			expected: nil,
		},
		{
			name: "status with retry info returns retry info",
			status: func() *status.Status {
				st := status.New(codes.ResourceExhausted, "resource exhausted")
				st, _ = st.WithDetails(&errdetails.RetryInfo{
					RetryDelay: durationpb.New(time.Second),
				})
				return st
			}(),
			expected: &errdetails.RetryInfo{
				RetryDelay: durationpb.New(time.Second),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRetryInfo(tt.status)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else if assert.NotNil(t, result) {
				assert.Equal(t, tt.expected.RetryDelay, result.RetryDelay)
			}
		})
	}
}

func TestGetThrottleDuration(t *testing.T) {
	tests := []struct {
		name      string
		retryInfo *errdetails.RetryInfo
		expected  time.Duration
	}{
		{
			name:      "nil retry info returns 0",
			retryInfo: nil,
			expected:  0,
		},
		{
			name: "nil retry delay returns 0",
			retryInfo: &errdetails.RetryInfo{
				RetryDelay: nil,
			},
			expected: 0,
		},
		{
			name: "valid retry delay returns correct duration",
			retryInfo: &errdetails.RetryInfo{
				RetryDelay: durationpb.New(time.Second + 500*time.Millisecond),
			},
			expected: time.Second + 500*time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getThrottleDuration(tt.retryInfo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldRetryHTTP(t *testing.T) {
	tests := []struct {
		name             string
		statusCode       int
		expectedRetry    bool
		expectedThrottle bool
	}{
		{
			name:             "OK status should not retry",
			statusCode:       200,
			expectedRetry:    false,
			expectedThrottle: false,
		},
		{
			name:             "Bad Request is permanent error",
			statusCode:       400,
			expectedRetry:    false,
			expectedThrottle: false,
		},
		{
			name:             "Unauthorized triggers rate limit flag",
			statusCode:       401,
			expectedRetry:    false,
			expectedThrottle: true,
		},
		{
			name:             "Forbidden triggers rate limit flag",
			statusCode:       403,
			expectedRetry:    false,
			expectedThrottle: true,
		},
		{
			name:             "Not Found is permanent error",
			statusCode:       404,
			expectedRetry:    false,
			expectedThrottle: false,
		},
		{
			name:             "Method Not Allowed is permanent error",
			statusCode:       405,
			expectedRetry:    false,
			expectedThrottle: false,
		},
		{
			name:             "Request Timeout is retryable",
			statusCode:       408,
			expectedRetry:    true,
			expectedThrottle: false,
		},
		{
			name:             "Unprocessable Entity is permanent error",
			statusCode:       422,
			expectedRetry:    false,
			expectedThrottle: false,
		},
		{
			name:             "Too Many Requests is retryable",
			statusCode:       429,
			expectedRetry:    true,
			expectedThrottle: false,
		},
		{
			name:             "Bad Gateway is retryable",
			statusCode:       502,
			expectedRetry:    true,
			expectedThrottle: false,
		},
		{
			name:             "Service Unavailable is retryable",
			statusCode:       503,
			expectedRetry:    true,
			expectedThrottle: false,
		},
		{
			name:             "Gateway Timeout is retryable",
			statusCode:       504,
			expectedRetry:    true,
			expectedThrottle: false,
		},
		{
			name:             "Unknown status code defaults to no retry",
			statusCode:       418,
			expectedRetry:    false,
			expectedThrottle: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retry, throttle := shouldRetryHTTP(tt.statusCode)
			assert.Equal(t, tt.expectedRetry, retry)
			assert.Equal(t, tt.expectedThrottle, throttle)
		})
	}
}

func TestGetHTTPThrottleDuration(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		headers    map[string]string
		expected   time.Duration
	}{
		{
			name:       "Non-429 status returns 0",
			statusCode: 503,
			headers:    map[string]string{"Retry-After": "30"},
			expected:   0,
		},
		{
			name:       "429 without Retry-After returns 0",
			statusCode: 429,
			headers:    map[string]string{},
			expected:   0,
		},
		{
			name:       "429 with integer Retry-After",
			statusCode: 429,
			headers:    map[string]string{"Retry-After": "30"},
			expected:   30 * time.Second,
		},
		{
			name:       "429 with invalid Retry-After",
			statusCode: 429,
			headers:    map[string]string{"Retry-After": "invalid"},
			expected:   0,
		},
		{
			name:       "429 with zero seconds",
			statusCode: 429,
			headers:    map[string]string{"Retry-After": "0"},
			expected:   0,
		},
		{
			name:       "429 with negative seconds",
			statusCode: 429,
			headers:    map[string]string{"Retry-After": "-10"},
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := make(map[string][]string)
			for k, v := range tt.headers {
				headers[k] = []string{v}
			}
			result := getHTTPThrottleDuration(tt.statusCode, headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}
