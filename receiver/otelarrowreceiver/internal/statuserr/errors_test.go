// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statuserr

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetStatusFromError_PlainError(t *testing.T) {
	err := errors.New("some transient error")
	got := GetStatusFromError(err)

	s, ok := status.FromError(got)
	require.True(t, ok)
	assert.Equal(t, codes.Unavailable, s.Code())
	assert.Equal(t, "some transient error", s.Message())
}

func TestGetStatusFromError_PermanentError(t *testing.T) {
	err := consumererror.NewPermanent(errors.New("bad data"))
	got := GetStatusFromError(err)

	s, ok := status.FromError(got)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, s.Code())
	assert.Contains(t, s.Message(), "bad data")
}

func TestGetStatusFromError_GRPCStatusError(t *testing.T) {
	err := status.Error(codes.ResourceExhausted, "too much data")
	got := GetStatusFromError(err)

	s, ok := status.FromError(got)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, s.Code())
	assert.Equal(t, "too much data", s.Message())
}
