// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceexhaustedretryextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestWrapGRPCError_Disabled(t *testing.T) {
	e := newExtension(&Config{}) // RetryDelay == 0
	err := status.Error(codes.ResourceExhausted, "full")
	assert.Equal(t, err, e.wrapGRPCError(err))
}

func TestWrapGRPCError_NilError(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	assert.NoError(t, e.wrapGRPCError(nil))
}

func TestWrapGRPCError_Permanent(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	inner := status.Error(codes.ResourceExhausted, "full")
	err := consumererror.NewPermanent(inner)
	assert.Equal(t, err, e.wrapGRPCError(err))
}

func TestWrapGRPCError_OtherCodes(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	for _, code := range []codes.Code{codes.Unavailable, codes.Internal, codes.InvalidArgument} {
		err := status.Error(code, "err")
		assert.Equal(t, err, e.wrapGRPCError(err), "code %v should not be modified", code)
	}
}

func TestWrapGRPCError_ExistingRetryInfo(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	st := status.New(codes.ResourceExhausted, "full")
	st, _ = st.WithDetails(&errdetails.RetryInfo{
		RetryDelay: durationpb.New(42 * time.Second),
	})
	err := st.Err()
	got := e.wrapGRPCError(err)
	assert.Equal(t, err, got)
}

func TestWrapGRPCError_InjectsRetryInfo(t *testing.T) {
	delay := 2 * time.Second
	e := newExtension(&Config{RetryDelay: delay})
	err := status.Error(codes.ResourceExhausted, "full")

	got := e.wrapGRPCError(err)
	require.NotEqual(t, err, got)

	st, ok := status.FromError(got)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())

	var ri *errdetails.RetryInfo
	for _, d := range st.Details() {
		if r, ok := d.(*errdetails.RetryInfo); ok {
			ri = r
		}
	}
	require.NotNil(t, ri, "RetryInfo not found in status details")
	assert.Equal(t, delay, ri.RetryDelay.AsDuration())
}

func TestEffectiveDelay_NoJitter(t *testing.T) {
	e := newExtension(&Config{RetryDelay: 3 * time.Second})
	assert.Equal(t, 3*time.Second, e.effectiveDelay())
}

func TestEffectiveDelay_WithJitter(t *testing.T) {
	base := 2 * time.Second
	jitter := 3 * time.Second
	e := newExtension(&Config{RetryDelay: base, Jitter: jitter})
	for range 1000 {
		d := e.effectiveDelay()
		assert.GreaterOrEqual(t, d, base, "delay below RetryDelay")
		assert.LessOrEqual(t, d, base+jitter, "delay above RetryDelay+Jitter")
	}
}

func TestGetGRPCServerOptions_Disabled(t *testing.T) {
	e := newExtension(&Config{})
	opts, err := e.GetGRPCServerOptions()
	require.NoError(t, err)
	assert.Empty(t, opts)
}

func TestGetGRPCServerOptions_ReturnsInterceptors(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	opts, err := e.GetGRPCServerOptions()
	require.NoError(t, err)
	require.Len(t, opts, 2, "expected unary + stream interceptor options")
	require.NotPanics(t, func() { grpc.NewServer(opts...) })
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "zero values valid",
			cfg:  Config{},
		},
		{
			name: "positive values valid",
			cfg:  Config{RetryDelay: 2 * time.Second, Jitter: 3 * time.Second},
		},
		{
			name:    "negative retry_delay",
			cfg:     Config{RetryDelay: -1 * time.Second},
			wantErr: "retry_delay must be non-negative",
		},
		{
			name:    "negative jitter",
			cfg:     Config{Jitter: -1 * time.Second},
			wantErr: "jitter must be non-negative",
		},
		{
			name:    "jitter exceeds 24h",
			cfg:     Config{Jitter: 25 * time.Hour},
			wantErr: "jitter must not exceed 24h",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
