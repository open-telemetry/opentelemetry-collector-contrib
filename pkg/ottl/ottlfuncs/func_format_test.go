// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
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
