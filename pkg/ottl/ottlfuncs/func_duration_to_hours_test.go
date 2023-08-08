// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_DurationToHours(t *testing.T) {
	tests := []struct {
		name     string
		duration ottl.StringGetter[interface{}]
		expected float64
	}{
		{
			name: "100 hours",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "100h", nil
				},
			},
			expected: 100,
		},
		{
			name: "1 min",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "100m", nil
				},
			},
			expected: 1.6666666666666665,
		},
		{
			name: "234 milliseconds",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "234ms", nil
				},
			},
			expected: 0.000065,
		},
		{
			name: "1 hour 40 mins 3 seconds 30 milliseconds 100 microseconds 1 nanosecond",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1h40m3s30ms100us1ns", nil
				},
			},
			expected: 1.667508361111389,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := DurationToHours(tt.duration)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
