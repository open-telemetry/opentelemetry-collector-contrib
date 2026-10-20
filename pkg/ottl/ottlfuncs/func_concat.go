// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/slicegetter"
)

type concatArguments[K any] struct {
	Vals      slicegetter.SliceGetter[K, ottl.Getter[K]]
	Delimiter ottl.StringGetter[K]
}

// NewConcatFactory returns a factory for the Concat OTTL function.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md#concat
func NewConcatFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Concat", &concatArguments[K]{}, createConcatFunction[K])
}

func createConcatFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*concatArguments[K])

	if !ok {
		return nil, errors.New("ConcatFactory args must be of type *concatArguments[K]")
	}

	if _, ok := args.Vals.Len(); ok {
		index := 0
		var rejectErr error
		_, err := args.Vals.Range(context.Background(), *new(K), func(g ottl.Getter[K]) bool {
			if val, ok := ottl.GetLiteralValue(g); ok && ottlcommon.IsMapOrList(val) {
				rejectErr = fmt.Errorf("Concat: value at index %d is a map or list, which cannot be concatenated", index)
				return false
			}
			index++
			return true
		})
		if err != nil {
			return nil, err
		}
		if rejectErr != nil {
			return nil, rejectErr
		}
	}

	return concat(&args.Vals, args.Delimiter), nil
}

func concat[K any](vals *slicegetter.SliceGetter[K, ottl.Getter[K]], delimiter ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		builder := strings.Builder{}
		delimiterVal, err := delimiter.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		first := true
		var concatErr error
		_, err = vals.Range(ctx, tCtx, func(rv ottl.Getter[K]) bool {
			raw, getErr := rv.Get(ctx, tCtx)
			if getErr != nil {
				concatErr = getErr
				return false
			}
			if ottlcommon.IsMapOrList(raw) {
				concatErr = ottl.TypeError(fmt.Sprintf("Concat: cannot concatenate a map or list, got %T", raw))
				return false
			}
			val, ok, getErr := ottlcommon.ToString(raw)
			if getErr != nil {
				concatErr = getErr
				return false
			}
			if !first {
				builder.WriteString(delimiterVal)
			}
			first = false
			if !ok {
				builder.WriteString("<nil>")
			} else {
				builder.WriteString(val)
			}
			return true
		})
		if err != nil {
			return nil, err
		}
		if concatErr != nil {
			return nil, concatErr
		}
		return builder.String(), nil
	}
}
