// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpcutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeoutEncode(t *testing.T) {
	// Note the gRPC specification limits durations to 8 digits,
	// so the use of 123456789 as a multiplier below forces the
	// next-larger unit to be used.
	require.Equal(t, "0m", EncodeTimeout(-time.Second))
	require.Equal(t, "1000m", EncodeTimeout(time.Second))
	require.Equal(t, "123m", EncodeTimeout(123*time.Millisecond))
	require.Equal(t, "123457S", EncodeTimeout(123456789*time.Millisecond))
	require.Equal(t, "2057614M", EncodeTimeout(123456789*time.Second))
	require.Equal(t, "2057614H", EncodeTimeout(123456789*time.Minute))
}

func mustDecode(t *testing.T, s string) time.Duration {
	d, err := DecodeTimeout(s)
	require.NoError(t, err, "must parse a timeout")
	return d
}

func TestTimeoutDecode(t *testing.T) {
	// Note the gRPC specification limits durations to 8 digits,
	// so the use of 123456789 as a multiplier below forces the
	// next-larger unit to be used.
	require.Equal(t, time.Duration(0), mustDecode(t, "0m"))
	require.Equal(t, time.Second, mustDecode(t, "1000m"))
	require.Equal(t, 123*time.Millisecond, mustDecode(t, "123m"))
	require.Equal(t, 123*time.Second, mustDecode(t, "123S"))
	require.Equal(t, 123*time.Minute, mustDecode(t, "123M"))
	require.Equal(t, 123*time.Hour, mustDecode(t, "123H"))

	// these are not encoded by EncodeTimeout, but will be decoded
	require.Equal(t, 123*time.Microsecond, mustDecode(t, "123u"))
	require.Equal(t, 123*time.Nanosecond, mustDecode(t, "123n"))

	// error cases
	testError := func(s string) {
		_, err := DecodeTimeout("123x")
		require.Error(t, err)
	}
	testError("123x")
	testError("x")
	testError("")
}
