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
		name    string
		formals []LocalIdentifierDecl
		want    []LocalIdentifierDecl
	}{
		{
			name:    "named params",
			formals: makeLocalIdentifiers("a", "b"),
			want:    makeLocalIdentifiers("a", "b"),
		},
		{
			name:    "blank and named params",
			formals: makeLocalIdentifiers("_", "a"),
			want:    makeLocalIdentifiers("_", "a"),
		},
		{
			name:    "all blank params",
			formals: makeLocalIdentifiers("_", "_", "_"),
			want:    makeLocalIdentifiers("_", "_", "_"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &LambdaExpression[any]{formals: tt.formals}
			assert.Equal(t, tt.want, expr.Formals())
		})
	}
}

func TestLambdaExpression_ValidateArity(t *testing.T) {
	tests := []struct {
		name    string
		formals []LocalIdentifierDecl
		arity   int
		wantErr string
	}{
		{
			name:    "matching arity",
			formals: makeLocalIdentifiers("a", "b"),
			arity:   2,
		},
		{
			name:    "no formals matches zero arity",
			formals: nil,
			arity:   0,
		},
		{
			name:    "blank formals count toward arity",
			formals: makeLocalIdentifiers("_", "a"),
			arity:   2,
		},
		{
			name:    "too few arguments",
			formals: makeLocalIdentifiers("a", "b"),
			arity:   1,
			wantErr: "lambda should be defined with exactly 1 formal(s), but has 2",
		},
		{
			name:    "too many arguments",
			formals: makeLocalIdentifiers("a"),
			arity:   3,
			wantErr: "lambda should be defined with exactly 3 formal(s), but has 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := newLambdaExpression[any](tt.formals, nil, nil)
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
			name: "literal body evaluate as-is",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("a"),
				newLiteral[any, any]("literal"),
				nil,
			),
			params: []any{"a value"},
			want:   "literal",
		},
		{
			name: "body expression",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("a"),
				nil,
				stubBoolExpr[any]{
					eval: func(ctx context.Context, _ any) (bool, error) {
						activation, ok := ctx.Value(localActivationKey{}).(*localActivation)
						if !ok {
							return false, errors.New("missing bindings")
						}
						v, ok := activation.resolve("a")
						return ok && v == "bound", nil
					},
				},
			),
			params: []any{"bound"},
			want:   true,
		},
		{
			name: "body expression error",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("a"),
				nil,
				stubBoolExpr[any]{
					eval: func(context.Context, any) (bool, error) {
						return false, errors.New("failed to evaluate")
					},
				},
			),
			params:  []any{"bound"},
			wantErr: "failed to evaluate",
		},
		{
			name: "body getter reads parameter",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("a"),
				&localIdentifierGetter[any]{
					identifier: &basePath[any]{name: "a"},
				},
				nil,
			),
			params: []any{42},
			want:   42,
		},
		{
			name: "parent binding is available",
			expr: newLambdaExpression[any](
				nil,
				&localIdentifierGetter[any]{
					identifier: &basePath[any]{name: "parent"},
				},
				nil,
			),
			ctx:    context.WithValue(t.Context(), localActivationKey{}, &localActivation{bindings: map[string]any{"parent": "value"}}),
			params: []any{},
			want:   "value",
		},
		{
			name: "formal overrides parent binding",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("a"),
				&localIdentifierGetter[any]{
					identifier: &basePath[any]{name: "a"},
				},
				nil,
			),
			ctx:    context.WithValue(t.Context(), localActivationKey{}, &localActivation{bindings: map[string]any{"a": "old"}}),
			params: []any{"new"},
			want:   "new",
		},
		{
			name: "parameter indexing",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("a"),
				&localIdentifierGetter[any]{
					identifier: &basePath[any]{
						name: "a",
						keys: []Key[any]{
							&baseKey[any]{s: ottltest.Strp("name")},
							&baseKey[any]{i: ottltest.Intp(1)},
						},
					},
				},
				nil,
			),
			params: []any{
				map[string]any{"name": []any{"zero", "one"}},
			},
			want: "one",
		},
		{
			name:    "invalid lambda without body",
			expr:    newLambdaExpression[any](nil, nil, nil),
			params:  []any{},
			wantErr: "invalid lambda: no body",
		},
		{
			name: "blank parameter is not bound",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("_", "a"),
				&localIdentifierGetter[any]{
					identifier: &basePath[any]{name: "a"},
				},
				nil,
			),
			params: []any{"skip", "bound"},
			want:   "bound",
		},
		{
			name: "blank parameter is omitted from bindings",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("_"),
				nil,
				stubBoolExpr[any]{
					eval: func(ctx context.Context, _ any) (bool, error) {
						activation, ok := ctx.Value(localActivationKey{}).(*localActivation)
						if !ok {
							return false, errors.New("missing bindings")
						}
						_, hasBlank := activation.bindings["_"]
						return !hasBlank && len(activation.bindings) == 0, nil
					},
				},
			),
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

			require.NoError(t, tt.expr.ValidateArity(len(tt.expr.Formals())))
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
	expr := newLambdaExpression[any](
		makeLocalIdentifiers("a"),
		&localIdentifierGetter[any]{
			identifier: &basePath[any]{name: "a"},
		},
		nil,
	)
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
		return newLambdaExpression(
			makeLocalIdentifiers("a"),
			&localIdentifierGetter[any]{
				identifier: &basePath[any]{name: "a"},
			},
			nil,
		)
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
	expr := newLambdaExpression[any](
		makeLocalIdentifiers("a"),
		nil,
		nil,
	)
	require.NoError(t, expr.ValidateArity(1))

	lb, err := expr.Activate(t.Context())
	require.NoError(t, err)

	err = lb.SetArg(-1, "x")
	require.EqualError(t, err, "argument index -1 out of range (len=1)")

	err = lb.SetArg(1, "x")
	require.EqualError(t, err, "argument index 1 out of range (len=1)")
}

func TestLambdaActivation_IsArgBound(t *testing.T) {
	expr := newLambdaExpression[any](
		makeLocalIdentifiers("acc", "_", "v"),
		nil,
		nil,
	)
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
	expr := newLambdaExpression[any](
		makeLocalIdentifiers("a", "b"),
		&localIdentifierGetter[any]{
			identifier: &basePath[any]{name: "b"},
		},
		nil,
	)
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
	outerExpr := newLambdaExpression[any](
		makeLocalIdentifiers("outer"),
		nil,
		stubBoolExpr[any]{
			eval: func(context.Context, any) (bool, error) {
				return true, nil
			},
		},
	)
	innerExpr := newLambdaExpression[any](
		makeLocalIdentifiers("inner"),
		&localIdentifierGetter[any]{
			identifier: &basePath[any]{name: "outer"},
		},
		nil,
	)
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
	expr := newLambdaExpression[any](
		makeLocalIdentifiers("a", "b"),
		&localIdentifierGetter[any]{
			identifier: &basePath[any]{name: "a"},
		},
		nil,
	)
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
