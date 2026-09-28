// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"
)

// LambdaExpression is a parsed OTTL lambda expression. OTTL functions may accept it as an argument.
// Call [LambdaExpression.ValidateArity] once in the function factory with the number of arguments
// the function passes to the lambda, then for each outer invocation call [LambdaExpression.Activate]
// with the evaluation context. Use [LambdaActivation.Call] to run the body (possibly multiple times
// with different arguments), and [LambdaActivation.Close] when finished.
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
type LambdaExpression[K any] struct {
	formals        []localIdentifierDecl
	body           Getter[K] // mutually exclusive with bodyExpr
	bodyExpr       boolExpr[K]
	activationPool *sync.Pool
}

// newLambdaExpression creates a new LambdaExpression. It must either have a body or a bodyExpr, but not both.
func newLambdaExpression[K any](formals []localIdentifierDecl, body Getter[K], bodyExpr boolExpr[K]) *LambdaExpression[K] {
	v := &LambdaExpression[K]{
		formals:  formals,
		body:     body,
		bodyExpr: bodyExpr,
	}
	v.activationPool = &sync.Pool{
		New: func() any {
			return newLambdaActivationState(v)
		},
	}
	return v
}

// ValidateArity returns an error if the lambda is not defined with exactly arity formals, including
// blank ("_") ones. Call it in the OTTL function factory, i.e. outside the closure the factory
// returns, so a lambda with the wrong number of formals is rejected when the statement is parsed
// instead of when [LambdaActivation.Call] is evaluated.
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
func (l *LambdaExpression[K]) ValidateArity(arity int) error {
	if len(l.formals) != arity {
		return lambdaArityError(arity, len(l.formals))
	}
	return nil
}

func lambdaArityError(arity, formals int) error {
	return fmt.Errorf("lambda should be defined with exactly %d formal(s), but has %d", arity, formals)
}

// Activate creates a [LambdaActivation] for a single outer function invocation. The lambda body is
// evaluated with ctx, which also gives it access to the formals of enclosing lambdas. Call
// [LambdaActivation.Close] on the returned activation when it is no longer needed.
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
func (l *LambdaExpression[K]) Activate(ctx context.Context) *LambdaActivation[K] {
	var state *lambdaActivationState[K]
	if l.activationPool != nil {
		state = l.activationPool.Get().(*lambdaActivationState[K])
	} else {
		state = newLambdaActivationState(l)
	}
	state.ctx = pushLocalActivation(ctx, state.activation)
	return &LambdaActivation[K]{state: state}
}

// LambdaActivation is a local activation of a [LambdaExpression] produced by [LambdaExpression.Activate].
// It must not be used by multiple goroutines at the same time.
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
type LambdaActivation[K any] struct {
	state *lambdaActivationState[K]
}

// lambdaActivationState is pooled apart from LambdaActivation so a closed handle can't touch a reused state.
type lambdaActivationState[K any] struct {
	expr       *LambdaExpression[K]
	ctx        context.Context
	activation *localActivation
}

func newLambdaActivationState[K any](expr *LambdaExpression[K]) *lambdaActivationState[K] {
	return &lambdaActivationState[K]{
		expr:       expr,
		activation: &localActivation{bindings: make(map[string]any, countNonBlankIdentifiers(expr.formals))},
	}
}

// Call binds args to the lambda's formals in declaration order and evaluates the body. The number
// of args must match the number of formals; args for blank ("_") formals are discarded. Each arg is
// converted to the value an OTTL path would produce, e.g. a pcommon.Value holding an int becomes an
// int64. The result follows the lambda body (value or boolean sub-expression) evaluation and may be
// nil if the body evaluates to nil.
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
func (a *LambdaActivation[K]) Call(tCtx K, args ...any) (any, error) {
	state := a.state
	if state == nil {
		return nil, errors.New("lambda activation is closed")
	}
	formals := state.expr.formals
	if len(args) != len(formals) {
		return nil, lambdaArityError(len(args), len(formals))
	}
	if v, ok := state.expr.getLiteralValue(); ok {
		return v, nil
	}
	for i, formal := range formals {
		if formal.IsBlank() {
			continue
		}
		state.activation.bindings[formal.Name()] = ottlcommon.NormalizeValue(args[i])
	}
	return state.expr.evalBody(state.ctx, tCtx)
}

func (l *LambdaExpression[K]) evalBody(ctx context.Context, tCtx K) (any, error) {
	switch {
	case l.bodyExpr != nil:
		return l.bodyExpr.Eval(ctx, tCtx)
	case l.body != nil:
		return l.body.Get(ctx, tCtx)
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

// Close releases the activation's resources. Calling Close more than once is a no-op, and
// [LambdaActivation.Call] returns an error once the activation is closed.
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
func (a *LambdaActivation[K]) Close() {
	state := a.state
	if state == nil {
		return
	}
	a.state = nil
	state.ctx = nil
	state.activation.parent = nil
	clear(state.activation.bindings)
	if pool := state.expr.activationPool; pool != nil {
		pool.Put(state)
	}
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
