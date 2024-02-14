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

func Test_Microseconds(t *testing.T) {
	tests := []struct {
		name     string
		duration ottl.DurationGetter[any]
		expected int64
	}{
		{
			name: "100 microseconds",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("100us")
				},
			},
			expected: 100,
		},
		{
			name: "1000 hour",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("100h")
				},
			},
			expected: 360000000000,
		},
		{
			name: "50 mins",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("50m")
				},
			},
			expected: 3000000000,
		},
		{
			name: "1 hour 40 mins 3 seconds 30 milliseconds 100 microseconds",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.ParseDuration("1h40m3s30ms100us")
				},
			},
			expected: 6003030100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Microseconds(tt.duration)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
