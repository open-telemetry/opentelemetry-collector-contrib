// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Log(t *testing.T) {
	noErrorTests := []struct {
		name     string
		value    interface{}
		expected interface{}
	}{
		{
			name:     "string",
			value:    "50",
			expected: math.Log(50),
		},
		{
			name:     "int64",
			value:    int64(333),
			expected: math.Log(333),
		},
		{
			name:     "float64",
			value:    float64(2.7),
			expected: math.Log(2.7),
		},
		{
			name:     "float64 without decimal",
			value:    float64(55),
			expected: math.Log(55),
		},
		{
			name:     "true",
			value:    true, // casts to 1 which Log(1) is 0 so it works.
			expected: float64(0),
		},
	}
	for _, tt := range noErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := logFunc[interface{}](&ottl.StandardFloatLikeGetter[interface{}]{
				Getter: func(context.Context, interface{}) (interface{}, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
	errorTests := []struct {
		name     string
		value    interface{}
		errorStr string
	}{
		{
			name:     "not a number string",
			value:    "test",
			errorStr: "invalid",
		},
		{
			name:     "false",
			value:    false,
			errorStr: "invalid",
		},
		{
			name:     "zero is undefined",
			value:    0.0,
			errorStr: "greater than zero",
		},
		{
			name:     "negative is undefined",
			value:    -30.3,
			errorStr: "greater than zero",
		},
		{
			name:     "nil",
			value:    nil,
			errorStr: "invalid",
		},
		{
			name:     "some struct",
			value:    struct{}{},
			errorStr: "unsupported",
		},
	}
	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := logFunc[interface{}](&ottl.StandardFloatLikeGetter[interface{}]{
				Getter: func(context.Context, interface{}) (interface{}, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.errorStr)
			assert.Equal(t, nil, result)
		})
	}
}
