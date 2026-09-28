// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/slicegetter"
)

var errDynamicSliceArgumentsDisabled = fmt.Errorf(
	"must be a list; passing a path or converter that resolves to a slice requires the `%s` feature gate to be enabled",
	metadata.PkgOttlFunctionsEnableDynamicSliceArgumentsFeatureGate.ID(),
)

// sliceElementCoercer is a generic type for handling SliceGetter element coercion operations.
// It stores metadata and provides functionality to coerce and iterate over slice elements.
type sliceElementCoercer[K any] struct {
	sliceItemType        reflect.Type
	sliceItemTypeName    string
	buildSliceItemGetter func(string, Getter[K]) (any, error)
}

func newSliceElementCoercer[K any](
	sliceItemType reflect.Type,
	buildSliceItemGetter func(string, Getter[K]) (any, error),
) *sliceElementCoercer[K] {
	return &sliceElementCoercer[K]{
		sliceItemType:        sliceItemType,
		sliceItemTypeName:    sliceItemType.Name(),
		buildSliceItemGetter: buildSliceItemGetter,
	}
}

func isLiteralSliceElementType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String, reflect.Uint8, reflect.Float64, reflect.Int64:
		return true
	default:
		return false
	}
}

type runtimeSliceSource[K any] struct {
	Getter[K]
	*sliceElementCoercer[K]
}

var _ slicegetter.Source[any] = runtimeSliceSource[any]{}

func (s runtimeSliceSource[K]) IsLiteral() bool {
	return isLiteralGetter(s.Getter)
}

func buildSliceGetterValue[K any](
	val value,
	sliceItemType reflect.Type,
	allowDynamic bool,
	buildSliceArg func(value, reflect.Type) (any, error),
	buildSliceItemGetter func(string, Getter[K]) (any, error),
	buildGetter func(value) (Getter[K], error),
) (any, error) {
	if val.List != nil || isLiteralSliceElementType(sliceItemType) {
		return buildSliceArg(val, reflect.SliceOf(sliceItemType))
	}
	if !allowDynamic {
		return nil, errDynamicSliceArgumentsDisabled
	}

	valueGetter, err := buildGetter(val)
	if err != nil {
		return nil, err
	}

	return runtimeSliceSource[K]{
		Getter:              valueGetter,
		sliceElementCoercer: newSliceElementCoercer(sliceItemType, buildSliceItemGetter),
	}, nil
}

// Len returns the length of a slice and a boolean indicating if it is a valid slice.
// If the value is not a slice, it returns 0 and false.
func (*sliceElementCoercer[K]) Len(slice any) (int, bool) {
	switch typedVal := slice.(type) {
	case pcommon.Slice:
		return typedVal.Len(), true
	case pcommon.Value:
		if typedVal.Type() != pcommon.ValueTypeSlice {
			return 0, false
		}
		return typedVal.Slice().Len(), true
	default:
		values := reflect.ValueOf(slice)
		if values.Kind() != reflect.Slice {
			return 0, false
		}
		return values.Len(), true
	}
}

// Range iterates over the slice and applies the yield function to each item after
// coercing the item to the slice item type. Yield true to continue iterating, false to stop.
// The returned boolean reports whether the slice is non-nil.
func (c *sliceElementCoercer[K]) Range(slice any, yield func(val any) bool) (bool, error) {
	switch typedVal := slice.(type) {
	case pcommon.Slice:
		for _, item := range typedVal.All() {
			itemGetter, err := c.buildItem(newLiteral[K, any](item))
			if err != nil {
				return false, err
			}
			if !yield(itemGetter) {
				return true, nil
			}
		}
	case pcommon.Value:
		if typedVal.Type() != pcommon.ValueTypeSlice {
			return false, fmt.Errorf("expected a slice, got %q", typedVal.Type())
		}
		return c.Range(typedVal.Slice(), yield)
	default:
		values := reflect.ValueOf(slice)
		if values.Kind() != reflect.Slice {
			return false, fmt.Errorf("expected a slice, got %T", slice)
		}
		if values.IsNil() {
			return false, nil
		}
		for i := 0; i < values.Len(); i++ {
			item := values.Index(i)
			if item.Type() == c.sliceItemType {
				if !yield(item.Interface()) {
					return true, nil
				}
			} else {
				var itemGetter any
				var err error
				rawValue := values.Index(i).Interface()
				if getter, ok := rawValue.(Getter[K]); ok {
					itemGetter, err = c.buildItem(getter)
				} else {
					itemGetter, err = c.buildItem(newLiteral[K, any](rawValue))
				}
				if err != nil {
					return false, err
				}
				if !yield(itemGetter) {
					return true, nil
				}
			}
		}
	}
	return true, nil
}

func (c *sliceElementCoercer[K]) buildItem(getter Getter[K]) (any, error) {
	item, err := c.buildSliceItemGetter(c.sliceItemTypeName, getter)
	if err != nil {
		return nil, err
	}
	if item == nil || !reflect.TypeOf(item).AssignableTo(c.sliceItemType) {
		return nil, TypeError(fmt.Sprintf("expected slice item of type %s, got %s", c.sliceItemType, reflect.TypeOf(item)))
	}
	return item, nil
}
