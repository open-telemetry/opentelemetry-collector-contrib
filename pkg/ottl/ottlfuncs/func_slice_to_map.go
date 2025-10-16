// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"errors"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"golang.org/x/net/context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type SliceToMapArguments[K any] struct {
	Target    ottl.PSliceGetter[K]
	KeyPath   ottl.Optional[[]string]
	ValuePath ottl.Optional[[]string]
}

func NewSliceToMapFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("SliceToMap", &SliceToMapArguments[K]{}, sliceToMapFunction[K])
}

func sliceToMapFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SliceToMapArguments[K])
	if !ok {
		return nil, errors.New("SliceToMapFactory args must be of type *SliceToMapArguments[K")
	}

	return getSliceToMapFunc(args.Target, args.KeyPath, args.ValuePath), nil
}

func getSliceToMapFunc[K any](target ottl.PSliceGetter[K], keyPath, valuePath ottl.Optional[[]string]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		return sliceToMapFromPcommon(val, keyPath, valuePath)

	}
}

func sliceToMapFromPcommon(v pcommon.Slice, keyPath, valuePath ottl.Optional[[]string]) (pcommon.Map, error) {
	m := pcommon.NewMap()
	m.EnsureCapacity(v.Len())

	useKeyPath := !keyPath.IsEmpty()
	useValuePath := !valuePath.IsEmpty()

	for i := 0; i < v.Len(); i++ {
		elem := v.At(i)

		// If key_path is not set, key is the index
		key := strconv.Itoa(i)
		// If value_path is not set, value is the whole element
		value := elem

		// If using key_path or value_path, element must be a map
		if useKeyPath || useValuePath {
			if elem.Type() != pcommon.ValueTypeMap {
				return pcommon.Map{}, fmt.Errorf("slice elements must be maps when using `key_path` or 'value_path', but could not cast element '%s' to a map", elem.Str())
			}
		}

		if useKeyPath {
			extractedKey, err := extractPcommonValue(elem.Map(), keyPath.Get())
			if err != nil {
				return pcommon.Map{}, fmt.Errorf("element %d: could not extract key from element: %w", i, err)
			}
			if extractedKey.Type() != pcommon.ValueTypeStr {
				return pcommon.Map{}, fmt.Errorf("element %d: extracted key attribute is not of type string", i)
			}
			key = extractedKey.Str()
		}

		if useValuePath {
			extractedValue, err := extractPcommonValue(elem.Map(), valuePath.Get())
			if err != nil {
				return pcommon.Map{}, fmt.Errorf("could not extract value from element: %w", err)
			}
			value = extractedValue
		}

		err := putValue(value, m, key)
		if err != nil {
			return pcommon.Map{}, err
		}

	}
	return m, nil
}

func putValue(value pcommon.Value, m pcommon.Map, key string) error {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		m.PutStr(key, value.Str())
	case pcommon.ValueTypeInt:
		m.PutInt(key, value.Int())
	case pcommon.ValueTypeDouble:
		m.PutDouble(key, value.Double())
	case pcommon.ValueTypeBool:
		m.PutBool(key, value.Bool())
	case pcommon.ValueTypeBytes:
		m.PutEmptyBytes(key).FromRaw(value.Bytes().AsRaw())
	case pcommon.ValueTypeMap:
		value.Map().CopyTo(m.PutEmptyMap(key))
	case pcommon.ValueTypeSlice:
		value.Slice().CopyTo(m.PutEmptySlice(key))
	case pcommon.ValueTypeEmpty:
		m.PutEmpty(key)
	default:
		return fmt.Errorf("unsupported value type %s for key %q", value.Type().String(), key)
	}
	return nil
}

func extractPcommonValue(m pcommon.Map, path []string) (pcommon.Value, error) {
	if len(path) == 0 {
		return pcommon.NewValueEmpty(), errors.New("must provide at least one path item")
	}

	val, ok := m.Get(path[0])
	if !ok {
		return pcommon.NewValueEmpty(), fmt.Errorf("provided object does not contain the path %v", path)
	}

	if len(path) == 1 {
		return val, nil
	}

	if val.Type() != pcommon.ValueTypeMap {
		return pcommon.NewValueEmpty(), fmt.Errorf("provided object does not contain the path %v", path)
	}

	return extractPcommonValue(val.Map(), path[1:])
}
