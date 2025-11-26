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
	Target ottl.PSliceGetSetter[K]
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

	return deleteFrom(args.Target, args.Index, args.Length), nil
}

func deleteFrom[K any](target ottl.PSliceGetSetter[K], indexGetter ottl.IntGetter[K], lengthGetter ottl.Optional[ottl.IntGetter[K]]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		t, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		index, err := indexGetter.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		length := int64(1)
		if !lengthGetter.IsEmpty() {
			length, err = lengthGetter.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
		}

		sliceLen := int64(t.Len())
		endIndex, err := validateBounds(index, length, sliceLen)
		if err != nil {
			return nil, err
		}

		if index == 0 && endIndex == sliceLen {
			// If deleting all elements, return an empty slice without looping
			return nil, target.Set(ctx, tCtx, pcommon.NewSlice())
		}

		// ToDo: This would be better with RemoveAt if it was supported.
		resSlice := pcommon.NewSlice()
		resSlice.EnsureCapacity(int(sliceLen - (endIndex - index)))
		for i := int64(0); i < sliceLen; i++ {
			if i < index || i >= endIndex {
				t.At(int(i)).MoveTo(resSlice.AppendEmpty())
			}
		}

		return nil, target.Set(ctx, tCtx, resSlice)
	}
}

func validateBounds(index, length, sliceLen int64) (endIndex int64, err error) {
	if index < 0 || index >= sliceLen {
		return 0, fmt.Errorf("index %d out of bounds for slice of length %d", index, sliceLen)
	}

	if length <= 0 {
		return 0, fmt.Errorf("length must be positive, got %d", length)
	}

	endIndex = index + length
	if endIndex > sliceLen {
		return 0, fmt.Errorf("deletion range [%d:%d] out of bounds for slice of length %d", index, endIndex, sliceLen)
	}
	return endIndex, nil
}
