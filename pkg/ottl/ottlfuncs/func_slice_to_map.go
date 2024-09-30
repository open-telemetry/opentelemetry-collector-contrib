// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"golang.org/x/net/context"
)

type SliceToMapArguments[K any] struct {
	Target    ottl.Getter[K]
	KeyPath   []string
	ValuePath ottl.Optional[[]string]
}

func NewSliceToMapFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("SliceToMap", &SliceToMapArguments[K]{}, sliceToMapFunction[K])
}

func sliceToMapFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SliceToMapArguments[K])
	if !ok {
		return nil, fmt.Errorf("AssociateFactory args must be of type *SliceToMapArguments[K")
	}

	return SliceToMap(args.Target, args.KeyPath, args.ValuePath)
}

func SliceToMap[K any](target ottl.Getter[K], keyPath []string, valuePath ottl.Optional[[]string]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		if len(keyPath) == 0 {
			return nil, fmt.Errorf("key path must contain at least one element")
		}
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
	}, nil
}

func sliceToMap(v []any, keyPath []string, valuePath ottl.Optional[[]string]) (any, error) {
	result := make(map[string]any, len(v))
	for _, elem := range v {
		switch e := elem.(type) {
		case map[string]any:
			obj, err := extractValue(e, keyPath)
			if err != nil {
				continue
			}

			var key string
			switch k := obj.(type) {
			case string:
				key = k
			default:
				continue
			}

			if valuePath.IsEmpty() {
				result[key] = e
				continue
			}
			obj, err = extractValue(e, valuePath.Get())
			if err != nil {
				continue
			}
			result[key] = obj
		default:
			continue
		}
	}
	m := pcommon.NewMap()
	if err := m.FromRaw(result); err != nil {
		return nil, fmt.Errorf("could not create pcommon.Map from result: %w", err)
	}

	return m, nil
}

func extractValue(v map[string]any, path []string) (any, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("must provide at least one path item")
	}
	obj, ok := v[path[0]]
	if !ok {
		return nil, fmt.Errorf("provided object does not contain the path %v", path)
	}
	if len(path) == 1 {
		return obj, nil
	}

	switch o := obj.(type) {
	case map[string]any:
		return extractValue(o, path[1:])
	default:
		return nil, fmt.Errorf("provided object does not contain the path %v", path)
	}
}
