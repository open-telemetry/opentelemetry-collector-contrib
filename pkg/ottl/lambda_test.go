// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

type stubBoolExpr[K any] struct {
	eval func(context.Context, K) (bool, error)
}

func (s stubBoolExpr[K]) Eval(ctx context.Context, tCtx K) (bool, error) {
	return s.eval(ctx, tCtx)
}

func (stubBoolExpr[K]) unexported() {}

func TestLambdaExpression_Formals(t *testing.T) {
	tests := []struct {
		name       string
		paramNames []LocalIdentifier
		want       []LocalIdentifier
	}{
		{
			name:       "named params",
			paramNames: makeLocalIdentifiers("$a", "$b"),
			want:       makeLocalIdentifiers("$a", "$b"),
		},
		{
			name:       "blank and named params",
			paramNames: makeLocalIdentifiers("_", "$a"),
			want:       makeLocalIdentifiers("_", "$a"),
		},
		{
			name:       "all blank params",
			paramNames: makeLocalIdentifiers("_", "_", "_"),
			want:       makeLocalIdentifiers("_", "_", "_"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &LambdaExpression[any]{paramNames: tt.paramNames}
			assert.Equal(t, tt.want, expr.Formals())
		})
	}
}

func TestLambdaExpression_Eval(t *testing.T) {
	tests := []struct {
		name    string
		expr    *LambdaExpression[any]
		ctx     context.Context
		params  []any
		want    any
		wantErr string
	}{
		{
			name: "not enough args",
			expr: &LambdaExpression[any]{paramNames: makeLocalIdentifiers("$a", "$b")},
			params: []any{
				1,
			},
			wantErr: "lambda expects exactly 1 argument(s), got 2",
		},
		{
			name: "literal body evaluate as-is",
			expr: &LambdaExpression[any]{
				paramNames: makeLocalIdentifiers("$a"),
				body:       newLiteral[any, any]("literal"),
			},
			params: []any{"a value"},
			want:   "literal",
		},
		{
			name: "body expression",
			expr: &LambdaExpression[any]{
				paramNames: makeLocalIdentifiers("$a"),
				bodyExpr: stubBoolExpr[any]{
					eval: func(ctx context.Context, _ any) (bool, error) {
						bindings, ok := ctx.Value(localBindingsKey{}).(map[string]any)
						if !ok {
							return false, errors.New("missing bindings")
						}
						return bindings["$a"] == "bound", nil
					},
				},
			},
			params: []any{"bound"},
			want:   true,
		},
		{
			name: "body expression error",
			expr: &LambdaExpression[any]{
				paramNames: makeLocalIdentifiers("$a"),
				bodyExpr: stubBoolExpr[any]{
					eval: func(context.Context, any) (bool, error) {
						return false, errors.New("failed to evaluate")
					},
				},
			},
			params:  []any{"bound"},
			wantErr: "failed to evaluate",
		},
		{
			name: "body getter reads parameter",
			expr: &LambdaExpression[any]{
				paramNames: makeLocalIdentifiers("$a"),
				body: &localBindingGetter[any]{
					identifierPath: &localIdentifier{Name: localIdentifierDecl("$a")},
				},
			},
			params: []any{42},
			want:   42,
		},
		{
			name: "parent binding is available",
			expr: &LambdaExpression[any]{
				body: &localBindingGetter[any]{
					identifierPath: &localIdentifier{Name: localIdentifierDecl("$parent")},
				},
			},
			ctx:    context.WithValue(t.Context(), localBindingsKey{}, map[string]any{"$parent": "value"}),
			params: []any{},
			want:   "value",
		},
		{
			name: "formal overrides parent binding",
			expr: &LambdaExpression[any]{
				paramNames: makeLocalIdentifiers("$a"),
				body: &localBindingGetter[any]{
					identifierPath: &localIdentifier{Name: localIdentifierDecl("$a")},
				},
			},
			ctx:    context.WithValue(t.Context(), localBindingsKey{}, map[string]any{"$a": "old"}),
			params: []any{"new"},
			want:   "new",
		},
		{
			name: "parameter indexing",
			expr: &LambdaExpression[any]{
				paramNames: makeLocalIdentifiers("$a"),
				body: &localBindingGetter[any]{
					identifierPath: &localIdentifier{
						Name: localIdentifierDecl("$a"),
						Keys: []key{
							{String: ottltest.Strp("name")},
							{Int: ottltest.Intp(1)},
						},
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
			expr:    &LambdaExpression[any]{},
			params:  []any{},
			wantErr: "invalid lambda: no body",
		},
		{
			name: "blank parameter is not bound",
			expr: &LambdaExpression[any]{
				paramNames: makeLocalIdentifiers("_", "$a"),
				body: &localBindingGetter[any]{
					identifierPath: &localIdentifier{Name: localIdentifierDecl("$a")},
				},
			},
			params: []any{"skip", "bound"},
			want:   "bound",
		},
		{
			name: "blank parameter is omitted from bindings",
			expr: &LambdaExpression[any]{
				paramNames: makeLocalIdentifiers("_"),
				bodyExpr: stubBoolExpr[any]{
					eval: func(ctx context.Context, _ any) (bool, error) {
						bindings, ok := ctx.Value(localBindingsKey{}).(map[string]any)
						if !ok {
							return false, errors.New("missing bindings")
						}
						_, hasBlank := bindings["_"]
						return !hasBlank && len(bindings) == 0, nil
					},
				},
			},
			params: []any{"skip"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx
			if ctx == nil {
				ctx = t.Context()
			}

			got, err := tt.expr.Eval(ctx, nil, tt.params)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
