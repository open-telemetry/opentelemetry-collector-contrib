// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

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
				getterFunc[any](func(_ context.Context, _ any) (any, error) {
					return 2, nil
				}),
			},
			expected: "test-0002",
		},
		{
			name:         "multiple-args",
			formatString: "test-%04d-%4s",
			formatArgs: []ottl.Getter[any]{
				getterFunc[any](func(_ context.Context, _ any) (any, error) {
					return 2, nil
				}),
				getterFunc[any](func(_ context.Context, _ any) (any, error) {
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
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormat_error(t *testing.T) {
	target := getterFunc[any](func(_ context.Context, _ any) (any, error) {
		return nil, errors.New("failed to get")
	})

	exprFunc := format[any]("test-%d", []ottl.Getter[any]{target})
	_, err := exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
