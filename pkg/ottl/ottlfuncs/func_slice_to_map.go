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
	Target    ottl.Getter[K]
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

func getSliceToMapFunc[K any](target ottl.Getter[K], keyPath, valuePath ottl.Optional[[]string]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch v := val.(type) {
		case []any:
			return sliceToMap(v, keyPath, valuePath)
		case pcommon.Slice:
			return sliceToMap(v.AsRaw(), keyPath, valuePath)
		default:
			return nil, fmt.Errorf("unsupported type provided to SliceToMap function: %T", v)
		}
	}
}

func sliceToMap(v []any, keyPath, valuePath ottl.Optional[[]string]) (any, error) {
	m := pcommon.NewMap()
	m.EnsureCapacity(len(v))

	for i, elem := range v {
		// Default key is index
		key := strconv.Itoa(i)

		mapData, rawValue, err := normalizeElement(elem)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}

		value := rawValue

		if !keyPath.IsEmpty() {
			if mapData == nil {
				return nil, fmt.Errorf("slice elements must be maps when using `key_path`, but could not cast element '%v' to a map", elem)
			}
			extractedKey, err := extractValue(mapData, keyPath.Get())
			if err != nil {
				return nil, fmt.Errorf("element %d: could not extract key from element: %w", i, err)
			}
			strKey, ok := extractedKey.(string)
			if !ok {
				return nil, fmt.Errorf("element %d: extracted key attribute is not of type string", i)
			}
			key = strKey
		}

		if !valuePath.IsEmpty() {
			if mapData == nil {
				return nil, fmt.Errorf("element %d: cannot extract value, not a map-like structure", elem)
			}
			extractedValue, err := extractValue(mapData, valuePath.Get())
			if err != nil {
				return nil, fmt.Errorf("could not extract value from element: %w", err)
			}
			value = extractedValue
		}

		if err := m.PutEmpty(key).FromRaw(value); err != nil {
			return nil, fmt.Errorf("could not convert value from element: %w", err)
		}
	}

	return m, nil
}

func extractValue(v map[string]any, path []string) (any, error) {
	if len(path) == 0 {
		return nil, errors.New("must provide at least one path item")
	}
	obj, ok := v[path[0]]
	if !ok {
		return nil, fmt.Errorf("provided object does not contain the path %v", path)
	}
	if len(path) == 1 {
		return obj, nil
	}

	if o, ok := obj.(map[string]any); ok {
		return extractValue(o, path[1:])
	}
	return nil, fmt.Errorf("provided object does not contain the path %v", path)
}

func normalizeElement(elem any) (map[string]any, any, error) {
	switch e := elem.(type) {
	case map[string]any:
		return e, e, nil
	case pcommon.Map:
		raw := e.AsRaw()
		return raw, raw, nil

	case pcommon.Value:
		if e.Type() == pcommon.ValueTypeMap {
			raw := e.Map().AsRaw()
			return raw, raw, nil

		}
		return nil, e.AsString(), nil
	default:
		return nil, e, nil
	}
}
