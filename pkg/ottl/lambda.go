// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/lambda"
)

// newLambdaExpression creates a new LambdaExpression. It must either have a body or a bodyExpr, but not both.
func newLambdaExpression[K any](formals []string, body Getter[K], bodyExpr boolExpr[K]) *lambda.LambdaExpression[K] {
	switch {
	case body != nil:
		if literal, ok := GetLiteralValue(body); ok {
			return lambda.NewLiteral[K](formals, literal)
		}
		return lambda.New(formals, body.Get)
	case bodyExpr != nil:
		if literal, ok := bodyExpr.(*literalBoolExpr[K]); ok {
			return lambda.NewLiteral[K](formals, literal.getValue())
		}
		return lambda.New(formals, func(ctx context.Context, tCtx K) (any, error) {
			return bodyExpr.Eval(ctx, tCtx)
		})
	default:
		return lambda.New[K](formals, nil)
	}
}
