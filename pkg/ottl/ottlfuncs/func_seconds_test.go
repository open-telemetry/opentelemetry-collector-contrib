// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Seconds(t *testing.T) {
	tests := []struct {
		name     string
		duration ottl.DurationGetter[any]
		expected float64
	}{
		{
			name: "100 seconds",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("100s")
				},
			},
			expected: 100,
		},
		{
			name: "1 hour",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("100h")
				},
			},
			expected: 360000,
		},
		{
			name: "11 mins",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("11m")
				},
			},
			expected: 660,
		},
		{
			name: "50 microseconds",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("11us")
				},
			},
			expected: 0.000011,
		},
		{
			name: "1 hour 40 mins 3 seconds 30 milliseconds 100 microseconds 1 nanosecond",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("1h40m3s30ms100us1ns")
				},
			},
			expected: 6003.030100001,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Seconds(tt.duration)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
