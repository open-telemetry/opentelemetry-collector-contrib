// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IndexArguments[K any] struct {
	Source ottl.Getter[K]
	Value  ottl.Getter[K]
}

func NewIndexFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Index", &IndexArguments[K]{}, createIndexFunction[K])
}

func createIndexFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IndexArguments[K])

	if !ok {
		return nil, errors.New("IndexFactory args must be of type *IndexArguments[K]")
	}
	return index(args.Source, args.Value), nil
}

func index[K any](source ottl.Getter[K], value ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		sourceVal, err := source.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		valueVal, err := value.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch s := sourceVal.(type) {
		case string:
			// For string source, the value must be string
			v, ok := valueVal.(string)
			if !ok {
				return nil, errors.New("when source is string, value must also be string")
			}
			return int64(strings.Index(s, v)), nil
		case pcommon.Slice:
			return findIndexInSlice(s, valueVal), nil
		default:
			return nil, errors.New("source must be string or slice type")
		}
	}
}

func findIndexInSlice(slice pcommon.Slice, value any) int64 {
	comparator := ottl.NewValueComparator()
	for i := 0; i < slice.Len(); i++ {
		sliceValue := slice.At(i).AsRaw()
		if comparator.Equal(sliceValue, value) {
			return int64(i)
		}
	}
	return -1
}
