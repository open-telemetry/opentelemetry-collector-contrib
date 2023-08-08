// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_DurationToSecs(t *testing.T) {
	tests := []struct {
		name     string
		duration ottl.StringGetter[interface{}]
		expected float64
	}{
		{
			name: "100 seconds",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "100s", nil
				},
			},
			expected: 100,
		},
		{
			name: "1 hour",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "100h", nil
				},
			},
			expected: 360000,
		},
		{
			name: "11 mins",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "11m", nil
				},
			},
			expected: 660,
		},
		{
			name: "50 microseconds",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "11us", nil
				},
			},
			expected: 0.000011,
		},
		{
			name: "1 hour 40 mins 3 seconds 30 milliseconds 100 microseconds 1 nanosecond",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1h40m3s30ms100us1ns", nil
				},
			},
			expected: 6003.030100001,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := DurationToSecs(tt.duration)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
