// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestSignalExporter_CanSend_AfterRateLimitTimeout(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 1,
			Duration:  time.Minute,
		},
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	rateLimitErr := errors.New("rate limit exceeded")
	exp.EnableRateLimit(rateLimitErr)
	exp.EnableRateLimit(rateLimitErr)

	assert.False(t, exp.canSend())

	// Mock the time to be after the rate limit timeout (1 minute)
	exp.rateError.state.Store(&rateErrorState{
		timestamp: time.Now().Add(-2 * time.Minute), // Set timestamp to 2 minutes ago
		err:       rateLimitErr,
	})

	assert.True(t, exp.canSend())
	assert.False(t, exp.rateError.isRateLimited())
}

func TestSignalExporter_CanSend_FeatureDisabled(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   false,
			Threshold: 1,
			Duration:  time.Minute,
		},
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	rateLimitErr := errors.New("rate limit exceeded")
	exp.EnableRateLimit(rateLimitErr)
	exp.EnableRateLimit(rateLimitErr)

	assert.True(t, exp.canSend())
}

func TestSignalExporter_CanSend_BeforeRateLimitTimeout(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 1,
			Duration:  time.Minute,
		},
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	rateLimitErr := errors.New("rate limit exceeded")
	exp.EnableRateLimit(rateLimitErr)
	exp.EnableRateLimit(rateLimitErr)

	// Mock the time to be before the rate limit timeout (30 seconds ago)
	exp.rateError.state.Store(&rateErrorState{
		timestamp: time.Now().Add(-30 * time.Second),
		err:       rateLimitErr,
	})

	// Should not be able to send because we're still within the timeout period
	assert.False(t, exp.canSend())
	assert.True(t, exp.rateError.isRateLimited())
}

func TestProcessError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{
			name:     "nil error returns nil",
			err:      nil,
			expected: nil,
		},
		{
			name:     "non-gRPC error returns permanent error",
			err:      errors.New("some error"),
			expected: consumererror.NewPermanent(errors.New("some error")),
		},
		{
			name:     "OK status returns nil",
			err:      status.Error(codes.OK, "ok"),
			expected: nil,
		},
		{
			name:     "permanent error returns permanent error",
			err:      status.Error(codes.InvalidArgument, "invalid argument"),
			expected: consumererror.NewPermanent(status.Error(codes.InvalidArgument, "invalid argument")),
		},
		{
			name:     "retryable error returns original error",
			err:      status.Error(codes.Unavailable, "unavailable"),
			expected: status.Error(codes.Unavailable, "unavailable"),
		},
		{
			name: "throttled error returns throttle retry",
			err:  status.Error(codes.ResourceExhausted, "resource exhausted"),
			expected: func() error {
				st := status.New(codes.ResourceExhausted, "resource exhausted")
				st, _ = st.WithDetails(&errdetails.RetryInfo{
					RetryDelay: durationpb.New(time.Second),
				})
				return exporterhelper.NewThrottleRetry(st.Err(), time.Second)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := &signalExporter{}
			err := exp.processError(tt.err)
			if tt.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if consumererror.IsPermanent(err) {
					assert.True(t, consumererror.IsPermanent(err))
				} else {
					assert.Contains(t, err.Error(), tt.expected.Error())
				}
			}
		})
	}
}
