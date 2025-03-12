// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Murmur3Hash(t *testing.T) {
	tests := []struct {
		name          string
		value         any
		expected      any
		err           bool
		expectedError string
	}{
		{
			name:     "string",
			value:    "Hello World",
			expected: "ce837619",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "00000000",
		},
		{
			name:          "non-string",
			value:         123,
			err:           true,
			expectedError: "expected string but got int",
		},
		{
			name:          "nil",
			value:         nil,
			err:           true,
			expectedError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := murmur3Hash[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			if tt.err {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func Test_CreateMurmur3HashFunc(t *testing.T) {
	factory := NewMurmur3HashFactory[any]()
	fCtx := ottl.FunctionContext{}

	// invalid args
	exprFunc, err := factory.CreateFunction(fCtx, nil)
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// valid args
	exprFunc, err = factory.CreateFunction(
		fCtx, &Murmur3HashArguments[any]{
			Target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Hello World", nil
				},
			},
		})
	assert.NoError(t, err)
	assert.NotNil(t, exprFunc)
}
