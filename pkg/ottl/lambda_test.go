// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type stubBoolExpr[K any] struct {
	eval func(context.Context, K) (bool, error)
}

func (s stubBoolExpr[K]) Eval(ctx context.Context, tCtx K) (bool, error) {
	return s.eval(ctx, tCtx)
}

func (stubBoolExpr[K]) unexported() {}

func TestLambdaExpression_ValidateArity(t *testing.T) {
	tests := []struct {
		name    string
		formals []localIdentifierDecl
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

func TestLambdaActivation_Call(t *testing.T) {
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
			params: []any{int64(42)},
			want:   int64(42),
		},
		{
			name: "pcommon.Value argument is normalized",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("a"),
				&localIdentifierGetter[any]{
					identifier: &basePath[any]{name: "a"},
				},
				nil,
			),
			params: []any{pcommon.NewValueStr("value")},
			want:   "value",
		},
		{
			name: "int argument is normalized",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("a"),
				&localIdentifierGetter[any]{
					identifier: &basePath[any]{name: "a"},
				},
				nil,
			),
			params: []any{7},
			want:   int64(7),
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
							&baseKey[any]{s: new("name")},
							&baseKey[any]{i: new(int64(1))},
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
		{
			name: "too few arguments",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("a", "b"),
				newLiteral[any, any]("literal"),
				nil,
			),
			params:  []any{"a"},
			wantErr: "lambda should be defined with exactly 1 formal(s), but has 2",
		},
		{
			name: "too many arguments",
			expr: newLambdaExpression[any](
				makeLocalIdentifiers("a"),
				newLiteral[any, any]("literal"),
				nil,
			),
			params:  []any{"a", "b"},
			wantErr: "lambda should be defined with exactly 2 formal(s), but has 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx
			if ctx == nil {
				ctx = t.Context()
			}

			lb := tt.expr.Activate(ctx)
			defer lb.Close()

			got, err := lb.Call(nil, tt.params...)
			if tt.wantErr != "" {
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

	lb := expr.Activate(t.Context())
	defer lb.Close()

	got, err := lb.Call(nil, "first")
	require.NoError(t, err)
	assert.Equal(t, "first", got)

	got, err = lb.Call(nil, "second")
	require.NoError(t, err)
	assert.Equal(t, "second", got)

	// Each Activate yields independent state, so overlapping activations of the same expression do not
	// interfere with one another.
	lb1 := expr.Activate(t.Context())
	defer lb1.Close()
	lb2 := expr.Activate(t.Context())
	defer lb2.Close()

	got1, err := lb1.Call(nil, "one")
	require.NoError(t, err)
	assert.Equal(t, "one", got1)

	got2, err := lb2.Call(nil, "two")
	require.NoError(t, err)
	assert.Equal(t, "two", got2)

	assert.Equal(t, "one", lb1.state.activation.bindings["a"])
}

func TestLambdaExpression_ZeroValue(t *testing.T) {
	var expr LambdaExpression[any]
	require.NoError(t, expr.ValidateArity(0))

	lb := expr.Activate(t.Context())
	defer lb.Close()

	_, err := lb.Call(nil)
	require.EqualError(t, err, "invalid lambda: no body")
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

	outerLb := outerExpr.Activate(t.Context())
	defer outerLb.Close()
	_, err := outerLb.Call(nil, "from-outer")
	require.NoError(t, err)

	innerLb := innerExpr.Activate(outerLb.state.ctx)
	defer innerLb.Close()

	got, err := innerLb.Call(nil, "inner-val")
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
	parentCtx := context.WithValue(t.Context(), localActivationKey{}, &localActivation{bindings: map[string]any{}})

	lb := expr.Activate(parentCtx)
	got, err := lb.Call(nil, "a", "b")
	require.NoError(t, err)
	assert.Equal(t, "a", got)

	state := lb.state
	require.NotNil(t, state.activation.parent)
	lb.Close()
	assert.Nil(t, state.ctx)
	assert.Nil(t, state.activation.parent)
	assert.Empty(t, state.activation.bindings)

	_, err = lb.Call(nil, "a", "b")
	require.EqualError(t, err, "lambda activation is closed")

	// A second Close must not return the state to the pool again, otherwise two later
	// activations would share it.
	lb.Close()
	lb1 := expr.Activate(t.Context())
	defer lb1.Close()
	lb2 := expr.Activate(t.Context())
	defer lb2.Close()
	assert.NotSame(t, lb1.state, lb2.state)
}
