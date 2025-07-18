// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IndexArguments[K any] struct {
	Target ottl.Getter[K]
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

	return index(ottl.NewValueComparator(), args.Target, args.Value), nil
}

func index[K any](valueComparator ottl.ValueComparator, target ottl.Getter[K], value ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		sourceVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		valueVal, err := value.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch s := sourceVal.(type) {
		case string:
			v, ok := valueVal.(string)
			if !ok {
				return nil, errors.New("invalid value type for Index function, value must be a string")
			}
			return int64(strings.Index(s, v)), nil
		case pcommon.Slice:
			return findIndexInSlice(valueComparator, s, valueVal), nil
		case pcommon.Value:
			if s.Type() != pcommon.ValueTypeSlice {
				return nil, errors.New("when source is pcommon.Value, only pcommon.ValueTypeSlice is supported")
			}
			return findIndexInSlice(valueComparator, s.Slice(), valueVal), nil
		case []any:
			// for the []any case, we try to utilize ValueComparator's power
			slice := pcommon.NewSlice()
			err = slice.FromRaw(s)
			if err != nil {
				return nil, err
			}
			return findIndexInSlice(valueComparator, slice, valueVal), nil
		case []string:
			return findIndexInStrings(s, valueVal), nil
		case []int:
			return findIndexInIntegers(valueComparator, s, valueVal), nil
		case []int16:
			return findIndexInIntegers(valueComparator, s, valueVal), nil
		case []int32:
			return findIndexInIntegers(valueComparator, s, valueVal), nil
		case []int64:
			return findIndexInIntegers(valueComparator, s, valueVal), nil
		case []uint:
			return findIndexInIntegers(valueComparator, s, valueVal), nil
		case []uint16:
			return findIndexInIntegers(valueComparator, s, valueVal), nil
		case []uint32:
			return findIndexInIntegers(valueComparator, s, valueVal), nil
		case []uint64:
			return findIndexInIntegers(valueComparator, s, valueVal), nil
		case []float32:
			return findIndexInFloats(valueComparator, s, valueVal), nil
		case []float64:
			return findIndexInFloats(valueComparator, s, valueVal), nil
		case []bool:
			return findIndexInBooleans(s, valueVal), nil
		default:
			return nil, errors.New("unsupported `" + reflect.TypeOf(s).String() + "` type")
		}
	}
}

func findIndexInSlice(valueComparator ottl.ValueComparator, slice pcommon.Slice, value any) int64 {
	for i := 0; i < slice.Len(); i++ {
		sliceValue := slice.At(i).AsRaw()
		if valueComparator.Equal(sliceValue, value) {
			return int64(i)
		}
	}
	return -1
}

func findIndexInStrings(slice []string, value any) int64 {
	strValue, ok := value.(string)
	if !ok {
		return -1
	}
	for i, v := range slice {
		if v == strValue {
			return int64(i)
		}
	}
	return -1
}

func findIndexInIntegers[T ~int | ~int16 | ~int32 | ~int64 | ~uint | ~uint16 | ~uint32 | ~uint64](valueComparator ottl.ValueComparator, slice []T, value any) int64 {
	for i, v := range slice {
		if valueComparator.Equal(int64(v), value) {
			return int64(i)
		}
	}
	return -1
}

func findIndexInFloats[T ~float32 | ~float64](valueComparator ottl.ValueComparator, slice []T, value any) int64 {
	for i, v := range slice {
		if valueComparator.Equal(float64(v), value) {
			return int64(i)
		}
	}
	return -1
}

func findIndexInBooleans(slice []bool, value any) int64 {
	boolValue, ok := value.(bool)
	if !ok {
		return -1
	}
	for i, v := range slice {
		if v == boolValue {
			return int64(i)
		}
	}
	return -1
}
