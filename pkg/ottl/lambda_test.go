// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/lambda"
)

type stubBoolExpr[K any] struct {
	eval func(context.Context, K) (bool, error)
}

func (s stubBoolExpr[K]) Eval(ctx context.Context, tCtx K) (bool, error) {
	return s.eval(ctx, tCtx)
}

func (stubBoolExpr[K]) unexported() {}

func Test_newLambdaExpression(t *testing.T) {
	tests := []struct {
		name     string
		formals  []string
		body     Getter[any]
		bodyExpr boolExpr[any]
		params   []any
		want     any
		wantErr  string
	}{
		{
			name:    "literal body evaluates as-is",
			formals: []string{"a"},
			body:    newLiteral[any, any]("literal"),
			params:  []any{"a value"},
			want:    "literal",
		},
		{
			name:     "literal body expression evaluates as-is",
			formals:  []string{"a"},
			bodyExpr: newAlwaysTrue[any](),
			params:   []any{"a value"},
			want:     true,
		},
		{
			name:    "body expression",
			formals: []string{"a"},
			bodyExpr: stubBoolExpr[any]{
				eval: func(ctx context.Context, _ any) (bool, error) {
					v, err := lambda.ResolveBinding(ctx, "a")
					return err == nil && v == "bound", nil
				},
			},
			params: []any{"bound"},
			want:   true,
		},
		{
			name:    "body expression error",
			formals: []string{"a"},
			bodyExpr: stubBoolExpr[any]{
				eval: func(context.Context, any) (bool, error) {
					return false, errors.New("failed to evaluate")
				},
			},
			params:  []any{"bound"},
			wantErr: "failed to evaluate",
		},
		{
			name:    "body getter reads parameter",
			formals: []string{"a"},
			body:    &localIdentifierGetter[any]{identifier: &basePath[any]{name: "a"}},
			params:  []any{42},
			want:    42,
		},
		{
			name:    "parameter indexing",
			formals: []string{"a"},
			body: &localIdentifierGetter[any]{
				identifier: &basePath[any]{
					name: "a",
					keys: []Key[any]{
						&baseKey[any]{s: new("name")},
						&baseKey[any]{i: new(int64(1))},
					},
				},
			},
			params: []any{
				map[string]any{"name": []any{"zero", "one"}},
			},
			want: "one",
		},
		{
			name:    "invalid lambda without body",
			wantErr: "invalid lambda: no body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := newLambdaExpression[any](tt.formals, tt.body, tt.bodyExpr)
			require.NoError(t, expr.ValidateArity(len(tt.formals)))
			activation, err := expr.Activate(t.Context())
			require.NoError(t, err)
			defer activation.Close()
			for i, param := range tt.params {
				require.NoError(t, activation.SetArg(i, param))
			}

			got, err := activation.Eval(nil)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
