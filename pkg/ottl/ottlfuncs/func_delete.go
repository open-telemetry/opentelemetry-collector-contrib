// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type DeleteArguments[K any] struct {
	Target ottl.GetSetter[K]
	Index  ottl.IntGetter[K]
	Length ottl.Optional[ottl.IntGetter[K]]
}

func NewDeleteFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("delete", &DeleteArguments[K]{}, createDeleteFunction[K])
}

func createDeleteFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DeleteArguments[K])
	if !ok {
		return nil, errors.New("DeleteFactory args must be of type *DeleteArguments[K]")
	}

	return deleteFrom(args.Target, args.Index, args.Length)
}

func deleteFrom[K any](target ottl.GetSetter[K], indexGetter ottl.IntGetter[K], lengthGetter ottl.Optional[ottl.IntGetter[K]]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		if target == nil {
			return nil, errors.New("target is nil")
		}

		t, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		index, err := indexGetter.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if index == -1 {
			// If we get -1 as index, do nothing and return nil.
			// Example: Calling delete with 'Index(log.attributes["tags"], "error")'
			// as index input value will result in -1 index, that means nothing to delete.
			return nil, target.Set(ctx, tCtx, t)
		}

		length := int64(1)
		if !lengthGetter.IsEmpty() {
			length, err = lengthGetter.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
		}

		var sourceSlice []any
		switch targetType := t.(type) {
		case pcommon.Slice:
			sourceSlice = targetType.AsRaw()
		case pcommon.Value:
			if targetType.Type() == pcommon.ValueTypeSlice {
				sourceSlice = targetType.Slice().AsRaw()
			} else {
				return nil, fmt.Errorf("target is not a slice, got pcommon.Value of type %q", targetType.Type())
			}
		default:
			return nil, fmt.Errorf("target must be a slice type, got %T", t)
		}

		sliceLen := int64(len(sourceSlice))
		if index < -1 || index >= sliceLen {
			return nil, fmt.Errorf("index %d out of bounds for slice of length %d", index, sliceLen)
		}

		if length <= 0 {
			return nil, fmt.Errorf("length must be positive, got %d", length)
		}

		endIndex := index + length
		if endIndex > sliceLen {
			endIndex = sliceLen
		}

		res := slices.Delete(slices.Clone(sourceSlice), int(index), int(endIndex))

		// Convert back to pcommon.Slice
		resSlice := pcommon.NewSlice()
		if err := resSlice.FromRaw(res); err != nil {
			return nil, err
		}

		return nil, target.Set(ctx, tCtx, resSlice)
	}, nil
}
