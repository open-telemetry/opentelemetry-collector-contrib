// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lambda

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resolveBody(name string) func(context.Context, any) (any, error) {
	return func(ctx context.Context, _ any) (any, error) {
		return ResolveBinding(ctx, name)
	}
}

func TestLambdaExpression_ValidateArity(t *testing.T) {
	tests := []struct {
		name    string
		formals []string
		arity   int
		wantErr string
	}{
		{
			name:    "matching arity",
			formals: []string{"a", "b"},
			arity:   2,
		},
		{
			name:    "no formals matches zero arity",
			formals: nil,
			arity:   0,
		},
		{
			name:    "blank formals count toward arity",
			formals: []string{"_", "a"},
			arity:   2,
		},
		{
			name:    "too few arguments",
			formals: []string{"a", "b"},
			arity:   1,
			wantErr: "lambda should be defined with exactly 1 formal(s), but has 2",
		},
		{
			name:    "too many arguments",
			formals: []string{"a"},
			arity:   3,
			wantErr: "lambda should be defined with exactly 3 formal(s), but has 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := New[any](tt.formals, nil)
			err := expr.ValidateArity(tt.arity)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
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
			name:   "literal body evaluate as-is",
			expr:   NewLiteral[any]([]string{"a"}, "literal"),
			params: []any{"a value"},
			want:   "literal",
		},
		{
			name: "body expression",
			expr: New([]string{"a"}, func(ctx context.Context, _ any) (any, error) {
				v, err := ResolveBinding(ctx, "a")
				return err == nil && v == "bound", nil
			}),
			params: []any{"bound"},
			want:   true,
		},
		{
			name: "body expression error",
			expr: New([]string{"a"}, func(context.Context, any) (any, error) {
				return nil, errors.New("failed to evaluate")
			}),
			params:  []any{"bound"},
			wantErr: "failed to evaluate",
		},
		{
			name:   "body reads parameter",
			expr:   New([]string{"a"}, resolveBody("a")),
			params: []any{42},
			want:   42,
		},
		{
			name:   "parent binding is available",
			expr:   New(nil, resolveBody("parent")),
			ctx:    context.WithValue(t.Context(), localActivationKey{}, &localActivation{bindings: map[string]any{"parent": "value"}}),
			params: []any{},
			want:   "value",
		},
		{
			name:   "formal overrides parent binding",
			expr:   New([]string{"a"}, resolveBody("a")),
			ctx:    context.WithValue(t.Context(), localActivationKey{}, &localActivation{bindings: map[string]any{"a": "old"}}),
			params: []any{"new"},
			want:   "new",
		},
		{
			name:    "invalid lambda without body",
			expr:    New[any](nil, nil),
			params:  []any{},
			wantErr: "invalid lambda: no body",
		},
		{
			name:   "blank parameter is not bound",
			expr:   New([]string{"_", "a"}, resolveBody("a")),
			params: []any{"skip", "bound"},
			want:   "bound",
		},
		{
			name: "blank parameter is omitted from bindings",
			expr: New([]string{"_"}, func(ctx context.Context, _ any) (any, error) {
				a, ok := ctx.Value(localActivationKey{}).(*localActivation)
				if !ok {
					return false, errors.New("missing bindings")
				}
				_, hasBlank := a.bindings["_"]
				return !hasBlank && len(a.bindings) == 0, nil
			}),
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

			require.NoError(t, tt.expr.ValidateArity(len(tt.expr.formals)))
			lb, err := tt.expr.Activate(ctx)
			require.NoError(t, err)
			defer lb.Close()
			for i, param := range tt.params {
				require.NoError(t, lb.SetArg(i, param))
			}

			got, err := lb.Eval(nil)
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

func TestLambdaExpression_Activate(t *testing.T) {
	expr := New([]string{"a"}, resolveBody("a"))
	require.NoError(t, expr.ValidateArity(1))

	lb, err := expr.Activate(t.Context())
	require.NoError(t, err)

	require.NoError(t, lb.SetArg(0, 1))
	got, err := lb.Eval(nil)
	require.NoError(t, err)
	assert.Equal(t, 1, got)

	require.NoError(t, lb.SetArg(0, 2))
	got, err = lb.Eval(nil)
	require.NoError(t, err)
	assert.Equal(t, 2, got)

	// Each Activate yields independent state, so overlapping activations of the same expression do not
	// interfere with one another.
	lb1, err := expr.Activate(t.Context())
	require.NoError(t, err)
	lb2, err := expr.Activate(t.Context())
	require.NoError(t, err)

	require.NoError(t, lb1.SetArg(0, "one"))
	require.NoError(t, lb2.SetArg(0, "two"))

	got1, err := lb1.Eval(nil)
	require.NoError(t, err)
	assert.Equal(t, "one", got1)

	got2, err := lb2.Eval(nil)
	require.NoError(t, err)
	assert.Equal(t, "two", got2)
}

func TestLambdaExpression_Activate_RequiresValidateArity(t *testing.T) {
	newExpr := func() *LambdaExpression[any] {
		return New([]string{"a"}, resolveBody("a"))
	}

	t.Run("errors when ValidateArity was not called", func(t *testing.T) {
		expr := newExpr()

		lb, err := expr.Activate(t.Context())
		require.Error(t, err)
		require.Nil(t, lb)
	})

	t.Run("errors when ValidateArity failed", func(t *testing.T) {
		expr := newExpr()

		require.Error(t, expr.ValidateArity(2))

		lb, err := expr.Activate(t.Context())
		require.Error(t, err)
		require.Nil(t, lb)
	})

	t.Run("succeeds after ValidateArity passed", func(t *testing.T) {
		expr := newExpr()

		require.NoError(t, expr.ValidateArity(1))

		lb, err := expr.Activate(t.Context())
		require.NoError(t, err)
		defer lb.Close()

		require.NoError(t, lb.SetArg(0, "value"))
		got, err := lb.Eval(nil)
		require.NoError(t, err)
		assert.Equal(t, "value", got)
	})

	t.Run("requires revalidation after a failed ValidateArity", func(t *testing.T) {
		expr := newExpr()

		require.NoError(t, expr.ValidateArity(1))
		require.Error(t, expr.ValidateArity(2))

		lb, err := expr.Activate(t.Context())
		require.Error(t, err)
		require.Nil(t, lb)

		require.NoError(t, expr.ValidateArity(1))

		lb, err = expr.Activate(t.Context())
		require.NoError(t, err)
		defer lb.Close()

		require.NoError(t, lb.SetArg(0, "value"))
		got, err := lb.Eval(nil)
		require.NoError(t, err)
		assert.Equal(t, "value", got)
	})
}

func TestLambdaActivation_SetArg(t *testing.T) {
	expr := New[any]([]string{"a"}, nil)
	require.NoError(t, expr.ValidateArity(1))

	lb, err := expr.Activate(t.Context())
	require.NoError(t, err)

	err = lb.SetArg(-1, "x")
	require.EqualError(t, err, "argument index -1 out of range (len=1)")

	err = lb.SetArg(1, "x")
	require.EqualError(t, err, "argument index 1 out of range (len=1)")
}

func TestLambdaActivation_IsArgBound(t *testing.T) {
	expr := New[any]([]string{"acc", "_", "v"}, nil)
	require.NoError(t, expr.ValidateArity(3))

	lb, err := expr.Activate(t.Context())
	require.NoError(t, err)

	assert.True(t, lb.IsArgBound(0), "named formal acc")
	assert.False(t, lb.IsArgBound(1), "blank formal")
	assert.True(t, lb.IsArgBound(2), "named formal v")

	assert.Panics(t, func() { lb.IsArgBound(-1) })
	assert.Panics(t, func() { lb.IsArgBound(3) })
}

func TestLambdaActivation_StaleArg(t *testing.T) {
	expr := New([]string{"a", "b"}, resolveBody("b"))
	require.NoError(t, expr.ValidateArity(2))

	lb, err := expr.Activate(t.Context())
	require.NoError(t, err)

	require.NoError(t, lb.SetArg(0, "first-a"))
	require.NoError(t, lb.SetArg(1, "first-b"))
	got, err := lb.Eval(nil)
	require.NoError(t, err)
	assert.Equal(t, "first-b", got)

	require.NoError(t, lb.SetArg(0, "second-a"))
	got, err = lb.Eval(nil)
	require.NoError(t, err)
	assert.Equal(t, "first-b", got)

	require.NoError(t, lb.SetArg(1, "second-b"))
	got, err = lb.Eval(nil)
	require.NoError(t, err)
	assert.Equal(t, "second-b", got)
}

func TestLambdaActivation_ParentChain(t *testing.T) {
	outerExpr := New([]string{"outer"}, func(context.Context, any) (any, error) {
		return true, nil
	})
	innerExpr := New([]string{"inner"}, resolveBody("outer"))
	require.NoError(t, outerExpr.ValidateArity(1))
	require.NoError(t, innerExpr.ValidateArity(1))

	outerLb, err := outerExpr.Activate(t.Context())
	require.NoError(t, err)
	require.NoError(t, outerLb.SetArg(0, "from-outer"))
	_, err = outerLb.Eval(nil)
	require.NoError(t, err)

	innerLb, err := innerExpr.Activate(outerLb.ctx)
	require.NoError(t, err)
	require.NoError(t, innerLb.SetArg(0, "inner-val"))

	got, err := innerLb.Eval(nil)
	require.NoError(t, err)
	assert.Equal(t, "from-outer", got)
}

func TestLambdaActivation_Close(t *testing.T) {
	expr := New([]string{"a", "b"}, resolveBody("a"))
	require.NoError(t, expr.ValidateArity(2))

	lb, err := expr.Activate(t.Context())
	require.NoError(t, err)
	require.NoError(t, lb.SetArg(0, 1))
	eval, err := lb.Eval(nil)
	require.NoError(t, err)
	assert.Equal(t, 1, eval)
	lb.Close()

	lb2, err := expr.Activate(t.Context())
	require.NoError(t, err)
	require.NotNil(t, lb2.activation)
	assert.Nil(t, lb2.activation.parent)
	assert.Empty(t, lb2.activation.bindings)
	assert.Equal(t, []any{nil, nil}, lb2.argValues)
}

func TestWithBindings(t *testing.T) {
	ctx := WithBindings(t.Context(), map[string]any{"outer": "parent-value", "value": "parent"})
	ctx = WithBindings(ctx, map[string]any{"value": "child"})

	got, err := ResolveBinding(ctx, "outer")
	require.NoError(t, err)
	assert.Equal(t, "parent-value", got)

	got, err = ResolveBinding(ctx, "value")
	require.NoError(t, err)
	assert.Equal(t, "child", got)
}

func TestResolveBinding(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		binding string
		want    any
		wantErr string
	}{
		{
			name:    "outside active local scope",
			ctx:     t.Context(),
			binding: "a",
			wantErr: `local identifier "a" evaluated outside of an active local scope`,
		},
		{
			name: "bound value",
			ctx: context.WithValue(t.Context(), localActivationKey{}, &localActivation{
				bindings: map[string]any{"a": 1},
			}),
			binding: "a",
			want:    1,
		},
		{
			name: "missing binding",
			ctx: context.WithValue(t.Context(), localActivationKey{}, &localActivation{
				bindings: map[string]any{"a": 1},
			}),
			binding: "missing",
			wantErr: `missing value for local identifier "missing"`,
		},
		{
			name: "inherits from parent activation",
			ctx: context.WithValue(t.Context(), localActivationKey{}, &localActivation{
				parent:   &localActivation{bindings: map[string]any{"outer": "parent-value"}},
				bindings: map[string]any{"inner": "child-value"},
			}),
			binding: "outer",
			want:    "parent-value",
		},
		{
			name: "child shadows parent binding",
			ctx: context.WithValue(t.Context(), localActivationKey{}, &localActivation{
				parent:   &localActivation{bindings: map[string]any{"value": "parent"}},
				bindings: map[string]any{"value": "child"},
			}),
			binding: "value",
			want:    "child",
		},
		{
			name: "explicit nil binding",
			ctx: context.WithValue(t.Context(), localActivationKey{}, &localActivation{
				bindings: map[string]any{"a": nil},
			}),
			binding: "a",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveBinding(tt.ctx, tt.binding)
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
