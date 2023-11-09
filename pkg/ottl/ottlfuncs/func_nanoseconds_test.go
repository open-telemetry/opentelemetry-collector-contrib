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

func Test_Nanoseconds(t *testing.T) {
	tests := []struct {
		name     string
		duration ottl.DurationGetter[any]
		expected int64
	}{
		{
			name: "100 nanoseconds",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("100ns")
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
			expected: 360000000000000,
		},
		{
			name: "23 mins",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("23m")
				},
			},
			expected: 1380000000000,
		},
		{
			name: "1 hour 40 mins 3 seconds 30 milliseconds 100 microseconds 1 nanosecond",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("1h40m3s30ms100us1ns")
				},
			},
			expected: 6003030100001,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Nanoseconds(tt.duration)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
