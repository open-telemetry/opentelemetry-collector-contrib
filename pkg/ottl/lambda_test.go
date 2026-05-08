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

func TestLambdaScopeStack(t *testing.T) {
	var scopes lambdaScopeStack
	assert.True(t, scopes.empty())
	assert.False(t, scopes.allows("value"))

	scopes.push(map[string]struct{}{"outer": {}})
	assert.False(t, scopes.empty())
	assert.True(t, scopes.allows("outer"))
	assert.False(t, scopes.allows("inner"))

	scopes.push(map[string]struct{}{"inner": {}})
	assert.True(t, scopes.allows("outer"))
	assert.True(t, scopes.allows("inner"))

	scopes.pop()
	assert.True(t, scopes.allows("outer"))
	assert.False(t, scopes.allows("inner"))

	scopes.pop()
	assert.True(t, scopes.empty())
}

func TestLambdaExpression_NumParams(t *testing.T) {
	expr := &LambdaExpression[any]{paramNames: []string{"a", "b"}}
	assert.Equal(t, 2, expr.NumParams())
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
			expr: &LambdaExpression[any]{paramNames: []string{"a", "b"}},
			params: []any{
				1,
			},
			wantErr: "lambda expected at least 2 argument(s), got 1",
		},
		{
			name: "literal body evaluate as-is",
			expr: &LambdaExpression[any]{
				paramNames: []string{"a"},
				body:       newLiteral[any, any]("literal"),
			},
			params: []any{"a value"},
			want:   "literal",
		},
		{
			name: "body expression",
			expr: &LambdaExpression[any]{
				paramNames: []string{"a"},
				bodyExpr: stubBoolExpr[any]{
					eval: func(ctx context.Context, _ any) (bool, error) {
						bindings, ok := ctx.Value(lambdaBindingsKey{}).(map[string]any)
						if !ok {
							return false, errors.New("missing bindings")
						}
						return bindings["a"] == "bound", nil
					},
				},
			},
			params: []any{"bound"},
			want:   true,
		},
		{
			name: "body expression error",
			expr: &LambdaExpression[any]{
				paramNames: []string{"a"},
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
				paramNames: []string{"a"},
				body: &lambdaBindingGetter[any]{
					paramPath: &lambdaParamPath{Name: lambdaParamDecl("a")},
				},
			},
			params: []any{42, "ignored"},
			want:   42,
		},
		{
			name: "parent binding is available",
			expr: &LambdaExpression[any]{
				body: &lambdaBindingGetter[any]{
					paramPath: &lambdaParamPath{Name: lambdaParamDecl("parent")},
				},
			},
			ctx:    context.WithValue(t.Context(), lambdaBindingsKey{}, map[string]any{"parent": "value"}),
			params: []any{},
			want:   "value",
		},
		{
			name: "formal overrides parent binding",
			expr: &LambdaExpression[any]{
				paramNames: []string{"a"},
				body: &lambdaBindingGetter[any]{
					paramPath: &lambdaParamPath{Name: lambdaParamDecl("a")},
				},
			},
			ctx:    context.WithValue(t.Context(), lambdaBindingsKey{}, map[string]any{"a": "old"}),
			params: []any{"new"},
			want:   "new",
		},
		{
			name: "parameter indexing",
			expr: &LambdaExpression[any]{
				paramNames: []string{"a"},
				body: &lambdaBindingGetter[any]{
					paramPath: &lambdaParamPath{
						Name: lambdaParamDecl("a"),
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
			name: "invalid lambda without body",
			expr: &LambdaExpression[any]{},
			params: []any{
				1,
			},
			wantErr: "invalid lambda: no body",
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

func TestLambdaBindingGetter_Get(t *testing.T) {
	tests := []struct {
		name    string
		getter  *lambdaBindingGetter[any]
		ctx     context.Context
		want    any
		wantErr string
	}{
		{
			name: "outside eval context",
			getter: &lambdaBindingGetter[any]{
				paramPath: &lambdaParamPath{Name: lambdaParamDecl("missing")},
			},
			ctx:     t.Context(),
			wantErr: "lambda parameter $missing evaluated outside of LambdaExpression.Eval",
		},
		{
			name: "missing binding",
			getter: &lambdaBindingGetter[any]{
				paramPath: &lambdaParamPath{Name: lambdaParamDecl("missing")},
			},
			ctx:     context.WithValue(t.Context(), lambdaBindingsKey{}, map[string]any{"other": 1}),
			wantErr: "missing value for lambda parameter $missing",
		},
		{
			name: "returns direct binding",
			getter: &lambdaBindingGetter[any]{
				paramPath: &lambdaParamPath{Name: lambdaParamDecl("value")},
			},
			ctx:  context.WithValue(t.Context(), lambdaBindingsKey{}, map[string]any{"value": "ok"}),
			want: "ok",
		},
		{
			name: "returns indexed binding",
			getter: &lambdaBindingGetter[any]{
				paramPath: &lambdaParamPath{
					Name: lambdaParamDecl("value"),
					Keys: []key{{String: ottltest.Strp("field")}},
				},
			},
			ctx:  context.WithValue(t.Context(), lambdaBindingsKey{}, map[string]any{"value": map[string]any{"field": "ok"}}),
			want: "ok",
		},
		{
			name: "indexing error is wrapped",
			getter: &lambdaBindingGetter[any]{
				paramPath: &lambdaParamPath{
					Name: lambdaParamDecl("value"),
					Keys: []key{{Int: ottltest.Intp(2)}},
				},
			},
			ctx:     context.WithValue(t.Context(), lambdaBindingsKey{}, map[string]any{"value": []any{"only"}}),
			wantErr: "cannot index lambda parameter $value: index 2 out of bounds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.getter.Get(tt.ctx, nil)
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
