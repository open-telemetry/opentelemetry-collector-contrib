// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

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
		t, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		targetSlice, ok := t.(pcommon.Slice)
		if !ok {
			targetValue, ok := t.(pcommon.Value)
			if !ok || targetValue.Type() != pcommon.ValueTypeSlice {
				return nil, fmt.Errorf("target must be a slice type, got %T", t)
			}
			targetSlice = targetValue.Slice()
		}

		index, err := indexGetter.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if index == -1 {
			// If we get -1 as index, do nothing and return nil.
			// Example: If we execute 'delete(attributes["tags"], Index(attributes["tags"], "error"))' statement
			// and 'error' is not found in tags, Index func will return -1, To avoid errors on this scenario,
			//  in this case delete function will be a no-op.
			return nil, target.Set(ctx, tCtx, t)
		}

		length := int64(1)
		if !lengthGetter.IsEmpty() {
			length, err = lengthGetter.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
		}

		sliceLen := int64(targetSlice.Len())
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

		resSlice := pcommon.NewSlice()
		resSlice.EnsureCapacity(int(sliceLen - (endIndex - index)))
		for i := int64(0); i < sliceLen; i++ {
			if i < index || i >= endIndex {
				targetSlice.At(int(i)).MoveTo(resSlice.AppendEmpty())
			}
		}

		return nil, target.Set(ctx, tCtx, resSlice)
	}, nil
}
