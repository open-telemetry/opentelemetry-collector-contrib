// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ConcatArguments[K any] struct {
	Vals      []ottl.StringLikeGetter[K] `ottlarg:"0"`
	Delimiter string                     `ottlarg:"1"`
	IgnoreNil bool                       `ottlarg:"2"`
}

func NewConcatFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Concat", &ConcatArguments[K]{}, createConcatFunction[K])
}

func createConcatFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ConcatArguments[K])

	if !ok {
		return nil, fmt.Errorf("ConcatFactory args must be of type *ConcatArguments[K]")
	}

	return concat(args.Vals, args.Delimiter, args.IgnoreNil), nil
}

func concat[K any](vals []ottl.StringLikeGetter[K], delimiter string, ignoreNil bool) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		builder := strings.Builder{}
		for i, rv := range vals {
			val, err := rv.Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			if val == nil && !ignoreNil {
				builder.WriteString(fmt.Sprint(val))
			} else {
				builder.WriteString(*val)
			}
			if !ignoreNil {
				if i != len(vals)-1 {
					builder.WriteString(delimiter)
				}
			}
		}
		return builder.String(), nil
	}
}
