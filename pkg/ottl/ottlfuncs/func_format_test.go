// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Format(t *testing.T) {
	tests := []struct {
		name         string
		formatString string
		formatArgs   []any
		expected     string
	}{
		{
			name:         "non formatting string",
			formatString: "test",
			formatArgs:   []any{},
			expected:     "test",
		},
		{
			name:         "padded int",
			formatString: "test-%04d",
			formatArgs:   []any{2},
			expected:     "test-0002",
		},
		{
			name:         "multiple-args",
			formatString: "test-%04d-%4s",
			formatArgs:   []any{2, "te"},
			expected:     "test-0002-  te",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatArgs := tt.formatArgs
			exprFunc := format[any](tt.formatString, ottl.StandardSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return formatArgs, nil
				},
			})
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormat_error(t *testing.T) {
	exprFunc := format[any]("test-%d", ottl.StandardSliceGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, errors.New("failed to get")
		},
	})
	_, err := exprFunc(t.Context(), nil)
	assert.Error(t, err)
}
