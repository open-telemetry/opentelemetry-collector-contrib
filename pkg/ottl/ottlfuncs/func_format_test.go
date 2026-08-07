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

type getterFunc[K any] func(ctx context.Context, tCtx K) (any, error)

func (g getterFunc[K]) Get(ctx context.Context, tCtx K) (any, error) {
	return g(ctx, tCtx)
}

func Test_Format(t *testing.T) {
	tests := []struct {
		name         string
		formatString string
		formatArgs   []ottl.Getter[any]
		expected     string
	}{
		{
			name:         "non formatting string",
			formatString: "test",
			formatArgs:   []ottl.Getter[any]{},
			expected:     "test",
		},
		{
			name:         "padded int",
			formatString: "test-%04d",
			formatArgs: []ottl.Getter[any]{
				getterFunc[any](func(context.Context, any) (any, error) {
					return 2, nil
				}),
			},
			expected: "test-0002",
		},
		{
			name:         "multiple-args",
			formatString: "test-%04d-%4s",
			formatArgs: []ottl.Getter[any]{
				getterFunc[any](func(context.Context, any) (any, error) {
					return 2, nil
				}),
				getterFunc[any](func(context.Context, any) (any, error) {
					return "te", nil
				}),
			},
			expected: "test-0002-  te",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := format(tt.formatString, tt.formatArgs)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormat_error(t *testing.T) {
	target := getterFunc[any](func(context.Context, any) (any, error) {
		return nil, errors.New("failed to get")
	})

	exprFunc := format[any]("test-%d", []ottl.Getter[any]{target})
	_, err := exprFunc(t.Context(), nil)
	assert.Error(t, err)
}

func Test_FormatFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewFormatFactory[any]()
		assert.Equal(t, "Format", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewFormatFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &FormatArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewFormatFactory[any]()
		args := factory.CreateDefaultArguments()
		formatArgs, ok := args.(*FormatArguments[any])
		require.True(t, ok)
		formatArgs.Format = "%s"
		formatArgs.Vals = []ottl.Getter[any]{
			&ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "value", nil
				},
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createFormatFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "FormatFactory args must be of type *FormatArguments[K]")
	})
}
