// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// LambdaExpression is a parsed OTTL lambda expression. OTTL functions may accept it as an argument.
// Implementations that run the lambda should call [LambdaExpression.Eval] with params in declaration
// order where params[i] is the value for the i-th parameter (see [LambdaExpression.Formals]).
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
type LambdaExpression[K any] struct {
	paramNames []LocalIdentifier
	body       Getter[K] // mutually exclusive with bodyExpr
	bodyExpr   boolExpr[K]
}

// Formals returns a copy of the lambda's formal parameters in declaration order (left to right).
// Blank ("_") placeholders are included; see [LambdaExpression.Eval] for binding behavior.
// Calling [LambdaExpression.Eval] with len(params) != len(Formals()) returns an error.
func (e *LambdaExpression[K]) Formals() []LocalIdentifier {
	return slices.Clone(e.paramNames)
}

// Eval runs the lambda with positional arguments bound to formals ($name) in declaration
// order (left to right). len(params) must equal len(Formals()).
// The result type follows the lambda body (value or boolean sub-expression) evaluation and
// may be nil if the body evaluates to nil.
func (e *LambdaExpression[K]) Eval(ctx context.Context, tCtx K, params []any) (any, error) {
	if len(e.paramNames) != len(params) {
		return nil, fmt.Errorf("lambda expects exactly %d argument(s), got %d", len(params), len(e.paramNames))
	}

	// Literal bodies are always evaluated as-is.
	if e.body != nil {
		if literalValue, ok := GetLiteralValue(e.body); ok {
			return literalValue, nil
		}
	}

	bindings := make(map[string]any, len(e.paramNames))
	for i, param := range e.paramNames {
		if param.IsBlank() {
			continue
		}
		bindings[param.Name()] = params[i]
	}

	return withLocalBindings(ctx, bindings, func(ctx context.Context) (any, error) {
		switch {
		case e.bodyExpr != nil:
			return e.bodyExpr.Eval(ctx, tCtx)
		case e.body != nil:
			return e.body.Get(ctx, tCtx)
		default:
			return nil, errors.New("invalid lambda: no body")
		}
	})
}
