// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
)

// lambdaBindingsKey is a context key used for storing lambda parameter bindings.
type lambdaBindingsKey struct{}

// lambdaScopeStack tracks parameter $names in scope while building lambda bodies (parse time only).
type lambdaScopeStack []map[string]struct{}

func (s *lambdaScopeStack) push(allowed map[string]struct{}) {
	*s = append(*s, allowed)
}

func (s *lambdaScopeStack) pop() {
	*s = (*s)[:len(*s)-1]
}

func (s *lambdaScopeStack) empty() bool {
	return len(*s) == 0
}

func (s *lambdaScopeStack) allows(name string) bool {
	if s.empty() {
		return false
	}
	// Start from the innermost scope and check if the name is allowed.
	for _, v := range slices.Backward(*s) {
		if _, ok := v[name]; ok {
			return true
		}
	}
	return false
}

// LambdaExpression is a parsed OTTL lambda expression. OTTL functions may accept it as an argument.
// Implementations that run the lambda should call [LambdaExpression.Eval] with params in declaration
// order where params[i] is the value for the i-th parameter (see [LambdaExpression.NumParams]).
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
type LambdaExpression[K any] struct {
	paramNames []string
	body       Getter[K] // mutually exclusive with bodyExpr
	bodyExpr   boolExpr[K]
}

// NumParams returns the number of formal parameters declared by the lambda expression call.
func (e *LambdaExpression[K]) NumParams() int {
	return len(e.paramNames)
}

// Eval runs the lambda with positional arguments bound to formals ($name) in declaration
// order (left to right). Callers MUST pass at least [LambdaExpression.NumParams] entries,
// extra arguments are ignored.
// The result type follows the lambda body (value or boolean sub-expression) and may be nil
// if the body evaluates to nil.
func (e *LambdaExpression[K]) Eval(ctx context.Context, tCtx K, params []any) (any, error) {
	if len(e.paramNames) > len(params) {
		return nil, fmt.Errorf("lambda expected at least %d argument(s), got %d", len(e.paramNames), len(params))
	}

	// Literal bodies are always evaluated as-is.
	if e.body != nil {
		if literalValue, ok := GetLiteralValue(e.body); ok {
			return literalValue, nil
		}
	}

	bindings := make(map[string]any)
	if parentBindings := ctx.Value(lambdaBindingsKey{}); parentBindings != nil {
		if v, ok := parentBindings.(map[string]any); ok {
			maps.Copy(bindings, v)
		}
	}

	for i, name := range e.paramNames {
		bindings[name] = params[i]
	}

	ctx = context.WithValue(ctx, lambdaBindingsKey{}, bindings)
	switch {
	case e.bodyExpr != nil:
		return e.bodyExpr.Eval(ctx, tCtx)
	case e.body != nil:
		return e.body.Get(ctx, tCtx)
	default:
		return nil, errors.New("invalid lambda: no body")
	}
}

type lambdaBindingGetter[K any] struct {
	paramPath *lambdaParamPath
}

func (g *lambdaBindingGetter[K]) Get(ctx context.Context, tCtx K) (any, error) {
	bindings, ok := ctx.Value(lambdaBindingsKey{}).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("lambda parameter $%s evaluated outside of LambdaExpression.Eval", g.paramPath.Name)
	}
	v, ok := bindings[string(g.paramPath.Name)]
	if !ok {
		return nil, fmt.Errorf("missing value for lambda parameter $%s", g.paramPath.Name)
	}
	if len(g.paramPath.Keys) > 0 {
		return g.getIndexableValue(ctx, tCtx, v)
	}
	return v, nil
}

func (g *lambdaBindingGetter[K]) getIndexableValue(ctx context.Context, tCtx K, val any) (any, error) {
	getter := &exprGetter[K]{
		expr: Expr[K]{exprFunc: func(context.Context, K) (any, error) {
			return val, nil
		}},
		keys: g.paramPath.Keys,
	}

	result, err := getter.Get(ctx, tCtx)
	if err != nil {
		return nil, fmt.Errorf("cannot index lambda parameter $%s: %w", g.paramPath.Name, err)
	}

	return result, nil
}
