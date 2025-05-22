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
