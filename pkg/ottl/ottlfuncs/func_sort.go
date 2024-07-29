// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"cmp"
	"context"
	"fmt"
	"go.uber.org/multierr"
	"slices"
	"strconv"

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

	return sort(args.Target, order)
}

func sort[K any](target ottl.Getter[K], order string) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch v := val.(type) {
		case pcommon.Slice:
			return sortSlice(v, order)
		case pcommon.Value:
			if v.Type() == pcommon.ValueTypeSlice {
				return sortSlice(v.Slice(), order)
			}
			return nil, fmt.Errorf("sort with unsupported type: '%s'. Target is not a list", v.Type().String())
		case []any:
			// handle Sort([1,2,3])
			slice := pcommon.NewValueSlice().SetEmptySlice()
			if err := slice.FromRaw(v); err != nil {
				errs := multierr.Append(err, fmt.Errorf("sort with unsupported type: '%T'. Target is not a list of primitive types", v))
				return nil, errs
			}
			return sortSlice(slice, order)
		case []string:
			// handle value from Split()
			dup := makeCopy(v)
			return sortTypedSlice(dup, order), nil
		case []int64:
			dup := makeCopy(v)
			return sortTypedSlice(dup, order), nil
		case []float64:
			dup := makeCopy(v)
			return sortTypedSlice(dup, order), nil
		case []bool:
			var arr []string
			for _, b := range v {
				arr = append(arr, strconv.FormatBool(b))
			}
			return sortTypedSlice(arr, order), nil
		default:
			return nil, fmt.Errorf("sort with unsupported type: '%T'. Target is not a list", v)
		}
	}, nil
}

// sortSlice sorts a pcommon.Slice based on the specified order.
// It gets the common type for all elements in the slice and converts all elements to this common type, creating a new copy
// Parameters:
//   - slice: The pcommon.Slice to be sorted
//   - order: The sort order. "asc" for ascending, "desc" for descending
//
// Returns:
//   - A sorted slice as []any or the original pcommon.Slice
//   - An error if an unsupported type is encountered
func sortSlice(slice pcommon.Slice, order string) (any, error) {
	length := slice.Len()
	if length == 0 {
		return slice, nil
	}

	commonType, ok := findCommonValueType(slice)
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
			s := slice.At(idx)
			if s.Type() == pcommon.ValueTypeInt {
				return float64(s.Int())
			}

			return s.Double()
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

// findCommonValueType determines the most appropriate common type for all elements in a pcommon.Slice.
// It returns two values:
//   - A pcommon.ValueType representing the desired common type for all elements.
//     Mixed Numeric types return ValueTypeDouble. Integer type returns ValueTypeInt. Double type returns ValueTypeDouble.
//     String, Bool, Empty and mixed of the mentioned types return ValueTypeStr, as they require string conversion for comparison.
//   - A boolean indicating whether a common type could be determined (true) or not (false).
//     returns false for ValueTypeMap, ValueTypeSlice and ValueTypeBytes. They are unsupported types for sort.
func findCommonValueType(slice pcommon.Slice) (pcommon.ValueType, bool) {
	length := slice.Len()
	if length == 0 {
		return pcommon.ValueTypeEmpty, false
	}

	wantType := slice.At(0).Type()
	wantStr := false
	wantDouble := false

	for i := 0; i < length; i++ {
		value := slice.At(i)
		currType := value.Type()

		switch currType {
		case pcommon.ValueTypeInt:
			if wantType == pcommon.ValueTypeDouble {
				wantDouble = true
			}
		case pcommon.ValueTypeDouble:
			if wantType == pcommon.ValueTypeInt {
				wantDouble = true
			}
		case pcommon.ValueTypeStr, pcommon.ValueTypeBool, pcommon.ValueTypeEmpty:
			wantStr = true
		default:
			return pcommon.ValueTypeEmpty, false
		}
	}

	if wantStr {
		wantType = pcommon.ValueTypeStr
	} else if wantDouble {
		wantType = pcommon.ValueTypeDouble
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

func makeCopy[T TargetType](src []T) []T {
	dup := make([]T, len(src))
	copy(dup, src)
	return dup
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
