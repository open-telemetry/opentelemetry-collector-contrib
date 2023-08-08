// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_DurationToMins(t *testing.T) {
	tests := []struct {
		name     string
		duration ottl.StringGetter[interface{}]
		expected float64
	}{
		{
			name: "100 minutes",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "100m", nil
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
			expected: 360,
		},
		{
			name: "234 milliseconds",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "234ms", nil
				},
			},
			expected: 0.0039,
		},
		{
			name: "1 hour 40 mins 3 seconds 30 milliseconds 100 microseconds 1 nanosecond",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1h40m3s30ms100us1ns", nil
				},
			},
			expected: 100.05050166668333,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := DurationToMins(tt.duration)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
