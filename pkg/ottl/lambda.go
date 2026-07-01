// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

// LambdaExpression is a parsed OTTL lambda expression. OTTL functions may accept it as an argument.
// For each outer invocation, call [LambdaExpression.Activate] with the evaluation context and the
// number of arguments to bind, so the [LambdaExpression.Formals] length must match it. Use
// [LambdaActivation.SetArg] to bind positional arguments, [LambdaActivation.Eval] to run the body
// (possibly multiple times with different arguments), and [LambdaActivation.Close] when finished.
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
type LambdaExpression[K any] struct {
	formals        []LocalIdentifierDecl
	body           Getter[K] // mutually exclusive with bodyExpr
	bodyExpr       boolExpr[K]
	activationPool *sync.Pool
}

// newLambdaExpression creates a new LambdaExpression. It must either have a body or a bodyExpr, but not both.
func newLambdaExpression[K any](formals []LocalIdentifierDecl, body Getter[K], bodyExpr boolExpr[K]) *LambdaExpression[K] {
	v := &LambdaExpression[K]{
		formals:  formals,
		body:     body,
		bodyExpr: bodyExpr,
	}
	nonBlankFormals := countNonBlankIdentifiers(formals)
	v.activationPool = &sync.Pool{
		New: func() any {
			return &LambdaActivation[K]{
				expr:       v,
				argValues:  make([]any, len(formals)),
				activation: &localActivation{bindings: make(map[string]any, nonBlankFormals)},
			}
		},
	}
	return v
}

// Formals returns a copy of the lambda's formal parameters in declaration order (left to right).
// Blank ("_") placeholders are included.
func (l *LambdaExpression[K]) Formals() []LocalIdentifierDecl {
	return slices.Clone(l.formals)
}

// Activate creates a [LambdaActivation] for a single outer function invocation, allocating
// its own activation and argument storage, linking the resulting activation to the given ctx.
// The arity is the number of formals the caller must pass, and their values are set via
// [LambdaActivation.SetArg]. If the lambda's declared formal count differs from arity, an
// error is returned.
// Call [LambdaActivation.SetArg] for every index in (0...arity) before each [LambdaActivation.Eval],
// and then [LambdaActivation.Close] on the returned activation when it is no longer needed.
func (l *LambdaExpression[K]) Activate(ctx context.Context, arity int) (*LambdaActivation[K], error) {
	if len(l.formals) != arity {
		return nil, fmt.Errorf("lambda expects exactly %d argument(s), got %d", arity, len(l.formals))
	}
	v := l.activationPool.Get().(*LambdaActivation[K])
	v.ctx = pushLocalActivation(ctx, v.activation)
	return v, nil
}

// LambdaActivation is a local activation of a [LambdaExpression] produced by [LambdaExpression.Activate].
type LambdaActivation[K any] struct {
	expr       *LambdaExpression[K]
	ctx        context.Context
	argValues  []any
	activation *localActivation
}

// SetArg sets the i-th positional argument for the next [LambdaActivation.Eval] call.
func (l *LambdaActivation[K]) SetArg(i int, v any) error {
	if i < 0 || i >= len(l.argValues) {
		return fmt.Errorf("argument index %d out of range (len=%d)", i, len(l.argValues))
	}
	l.argValues[i] = v
	return nil
}

// Eval runs the lambda with positional arguments set via [LambdaActivation.SetArg].
// The result type follows the lambda body (value or boolean sub-expression) evaluation and
// may be nil if the body evaluates to nil.
func (l *LambdaActivation[K]) Eval(tCtx K) (any, error) {
	if v, ok := l.expr.getLiteralValue(); ok {
		return v, nil
	}
	l.bindArguments()
	return l.evalBody(tCtx)
}

func (l *LambdaActivation[K]) bindArguments() {
	for i, formal := range l.expr.formals {
		if formal.IsBlank() {
			continue
		}
		l.activation.bindings[formal.Name()] = l.argValues[i]
	}
}

func (l *LambdaActivation[K]) evalBody(tCtx K) (any, error) {
	switch {
	case l.expr.bodyExpr != nil:
		return l.expr.bodyExpr.Eval(l.ctx, tCtx)
	case l.expr.body != nil:
		return l.expr.body.Get(l.ctx, tCtx)
	default:
		return nil, errors.New("invalid lambda: no body")
	}
}

func (l *LambdaExpression[K]) getLiteralValue() (any, bool) {
	if l.body != nil {
		if literalValue, ok := GetLiteralValue(l.body); ok {
			return literalValue, true
		}
	}
	if l.bodyExpr != nil {
		if litExp, ok := l.bodyExpr.(*literalBoolExpr[K]); ok {
			return litExp.getValue(), true
		}
	}
	return nil, false
}

// Close releases the activation's resources. Call it when the activation is no longer needed.
func (l *LambdaActivation[K]) Close() {
	l.ctx = nil
	l.activation.parent = nil
	clear(l.activation.bindings)
	clear(l.argValues)
	l.expr.activationPool.Put(l)
}

// NewTestingLambdaExpression creates a LambdaExpression with a value body for use in tests.
// eval is called with resolveBinding to resolve local identifier values from the active scope.
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
func NewTestingLambdaExpression[K any](
	params []string,
	eval func(ctx context.Context, tCtx K, resolveBinding func(string) any) (any, error),
) *LambdaExpression[K] {
	getter := exprGetter[K]{
		expr: Expr[K]{exprFunc: func(ctx context.Context, tCtx K) (any, error) {
			resolveBinding := func(name string) any {
				v, err := resolveLocalIdentifierBinding(ctx, name)
				if err != nil {
					return nil
				}
				return v
			}
			return eval(ctx, tCtx, resolveBinding)
		}},
	}
	return newLambdaExpression(makeLocalIdentifiers(params...), &getter, nil)
}
