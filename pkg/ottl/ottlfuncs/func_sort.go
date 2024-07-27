// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	sortAsc  = "asc"
	sortDesc = "desc"
)

type SortArguments[K any] struct {
	Target ottl.Getter[K]
	Order  ottl.Optional[string]
}

func NewSortFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Sort", &SortArguments[K]{}, createSortFunction[K])
}

func createSortFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SortArguments[K])

	if !ok {
		return nil, fmt.Errorf("SortFactory args must be of type *SortArguments[K]")
	}

	order := sortAsc
	if !args.Order.IsEmpty() {
		o := args.Order.Get()
		switch o {
		case sortAsc, sortDesc:
			order = o
		default:
			return nil, fmt.Errorf("invalid arguments: %s. Order should be either \"%s\" or \"%s\"", o, sortAsc, sortDesc)
		}
	}

	return Sort(args.Target, order)
}

func Sort[K any](target ottl.Getter[K], order string) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if order != sortAsc && order != sortDesc {
			return nil, fmt.Errorf("invalid arguments: %s. Order should be either \"%s\" or \"%s\"", order, sortAsc, sortDesc)
		}

		switch v := val.(type) {
		case pcommon.Slice:
			return sortSlice(v, order)
		case []any:
			// handle Sort([1,2,3])
			slice := pcommon.NewValueSlice().SetEmptySlice()
			if err := slice.FromRaw(v); err != nil {
				return v, nil
			}
			return sortSlice(slice, order)
		default:
			return v, nil
		}
	}, nil
}

func sortSlice(slice pcommon.Slice, order string) (any, error) {
	length := slice.Len()
	if length == 0 {
		return slice, nil
	}

	commonType, ok := getCommonValueType(slice)
	if !ok {
		return slice, nil
	}

	switch commonType {
	case pcommon.ValueTypeInt:
		arr := makeTypedCopy(length, func(idx int) int64 {
			return slice.At(idx).Int()
		})
		return sortTypedSlice(arr, order), nil
	case pcommon.ValueTypeDouble:
		arr := makeTypedCopy(length, func(idx int) float64 {
			return slice.At(idx).Double()
		})
		return sortTypedSlice(arr, order), nil
	case pcommon.ValueTypeStr:
		arr := makeTypedCopy(length, func(idx int) string {
			return slice.At(idx).AsString()
		})
		return sortTypedSlice(arr, order), nil
	default:
		return nil, fmt.Errorf("sort with unsupported type: '%T'", commonType)
	}
}

type TargetType interface {
	~int64 | ~float64 | ~string
}

// getCommonValueType determines the most appropriate common type for all elements in a pcommon.Slice.
// It returns two values:
//   - A pcommon.ValueType representing the desired common type for all elements.
//     Mixed types, String, Bool, and Empty types return ValueTypeStr as they require string conversion for comparison.
//     Numeric types return either ValueTypeInt or ValueTypeDouble.
//   - A boolean indicating whether a common type could be determined (true) or not (false).
//     returns false for ValueTypeMap, ValueTypeSlice and ValueTypeBytes. They are unsupported types for sort.
func getCommonValueType(slice pcommon.Slice) (pcommon.ValueType, bool) {
	length := slice.Len()
	if length == 0 {
		return pcommon.ValueTypeEmpty, false
	}

	wantType := slice.At(0).Type()
	wantStr := false

	for i := 0; i < length; i++ {
		value := slice.At(i)
		currType := value.Type()

		if currType != wantType {
			wantStr = true
		}

		switch currType {
		case pcommon.ValueTypeInt, pcommon.ValueTypeDouble:
		case pcommon.ValueTypeStr, pcommon.ValueTypeBool, pcommon.ValueTypeEmpty:
			wantStr = true
		default:
			return pcommon.ValueTypeEmpty, false
		}
	}

	if wantStr {
		wantType = pcommon.ValueTypeStr
	}

	return wantType, true
}

func makeTypedCopy[T TargetType](length int, converter func(idx int) T) []T {
	var arr []T
	for i := 0; i < length; i++ {
		arr = append(arr, converter(i))
	}
	return arr
}

func sortTypedSlice[T TargetType](arr []T, order string) []T {
	if len(arr) == 0 {
		return arr
	}

	slices.SortFunc(arr, func(a, b T) int {
		if order == sortDesc {
			return cmp.Compare(b, a)
		}
		return cmp.Compare(a, b)
	})

	return arr
}
