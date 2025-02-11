// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// The EncodeTimeout function is forked and modified from the original
// https://github.com/grpc/grpc-go/blob/master/internal/grpcutil/encode_duration.go

// This DecodeTimeout function is forked and modified from the original
// https://github.com/grpc/grpc-go/blob/master/internal/transport/http_util.go

package grpcutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil"

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

const maxTimeoutValue int64 = 100000000 - 1

// div does integer division and round-up the result. Note that this is
// equivalent to (d+r-1)/r but has less chance to overflow.
func div(d, r time.Duration) int64 {
	if d%r > 0 {
		return int64(d/r + 1)
	}
	return int64(d / r)
}

type timeoutUnit uint8

const (
	hour        timeoutUnit = 'H'
	minute      timeoutUnit = 'M'
	second      timeoutUnit = 'S'
	millisecond timeoutUnit = 'm'
	microsecond timeoutUnit = 'u'
	nanosecond  timeoutUnit = 'n'
)

// EncodeTimeout encodes the duration to the format grpc-timeout
// header accepts.  This is copied from the gRPC-Go implementation,
// with two branches of the original six branches removed, leaving the
// four you see for milliseconds, seconds, minutes, and hours. This
// code will not encode timeouts less than one millisecond.  See:
//
// https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests
func EncodeTimeout(t time.Duration) string {
	if t < time.Millisecond {
		return "0m"
	}
	if d := div(t, time.Millisecond); d <= maxTimeoutValue {
		return fmt.Sprintf("%d%c", d, millisecond)
	}
	if d := div(t, time.Second); d <= maxTimeoutValue {
		return fmt.Sprintf("%d%c", d, second)
	}
	if d := div(t, time.Minute); d <= maxTimeoutValue {
		return fmt.Sprintf("%d%c", d, minute)
	}
	// Note that maxTimeoutValue * time.Hour > MaxInt64.
	return fmt.Sprintf("%d%c", div(t, time.Hour), hour)
}

func timeoutUnitToDuration(u timeoutUnit) (d time.Duration, ok bool) {
	switch u {
	case hour:
		return time.Hour, true
	case minute:
		return time.Minute, true
	case second:
		return time.Second, true
	case millisecond:
		return time.Millisecond, true
	case microsecond:
		return time.Microsecond, true
	case nanosecond:
		return time.Nanosecond, true
	default:
	}
	return
}

// DecodeTimeout parses a string associated with the "grpc-timeout"
// header.  Note this will accept all valid gRPC units including
// microseconds and nanoseconds, which EncodeTimeout avoids.  This is
// specified in:
//
// https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests
func DecodeTimeout(s string) (time.Duration, error) {
	size := len(s)
	if size < 2 {
		return 0, fmt.Errorf("transport: timeout string is too short: %q", s)
	}
	if size > 9 {
		// Spec allows for 8 digits plus the unit.
		return 0, fmt.Errorf("transport: timeout string is too long: %q", s)
	}
	unit := timeoutUnit(s[size-1])
	d, ok := timeoutUnitToDuration(unit)
	if !ok {
		return 0, fmt.Errorf("transport: timeout unit is not recognized: %q", s)
	}
	t, err := strconv.ParseInt(s[:size-1], 10, 64)
	if err != nil {
		return 0, err
	}
	const maxHours = math.MaxInt64 / int64(time.Hour)
	if d == time.Hour && t > maxHours {
		// This timeout would overflow math.MaxInt64; clamp it.
		return time.Duration(math.MaxInt64), nil
	}
	return d * time.Duration(t), nil
}
