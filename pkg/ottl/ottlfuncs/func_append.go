// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type AppendArguments[K any] struct {
	Target ottl.GetSetter[K]
	Value  ottl.Optional[ottl.Getter[K]]
	Values ottl.Optional[[]ottl.Getter[K]]
}

func NewAppendFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("append", &AppendArguments[K]{}, createAppendFunction[K])
}
func createAppendFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*AppendArguments[K])
	if !ok {
		return nil, fmt.Errorf("AppendFactory args must be of type *Appendrguments[K]")
	}

	return Append(args.Target, args.Value, args.Values)
}

func Append[K any](target ottl.GetSetter[K], value ottl.Optional[ottl.Getter[K]], values ottl.Optional[[]ottl.Getter[K]]) (ottl.ExprFunc[K], error) {
	if value.IsEmpty() && values.IsEmpty() {
		return nil, fmt.Errorf("one of optional arguments needs to be provided")
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		t, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if t == nil {
			// target does not exist, init a slice
			t = make([]string, 0)
		}

		// init res with target values
		var res []any

		switch targetType := t.(type) {
		case pcommon.Slice:
			res = append(res, targetType.AsRaw()...)
		case pcommon.Value:
			switch targetType.Type() {
			case pcommon.ValueTypeEmpty:
				res = append(res, targetType.Str())
			case pcommon.ValueTypeStr:
				res = append(res, targetType.Str())
			case pcommon.ValueTypeInt:
				res = append(res, targetType.Int())
			case pcommon.ValueTypeDouble:
				res = append(res, targetType.Double())
			case pcommon.ValueTypeBool:
				res = append(res, targetType.Bool())
			case pcommon.ValueTypeSlice:
				res = append(res, targetType.Slice().AsRaw()...)
			default:
				return nil, fmt.Errorf("unsupported type of target field")
			}

		case []string:
			res = appendMultiple(res, targetType)
		case []any:
			res = append(res, targetType...)
		case []int64:
			res = appendMultiple(res, targetType)
		case []bool:
			res = appendMultiple(res, targetType)
		case []float64:
			res = appendMultiple(res, targetType)

		case string:
			res = append(res, targetType)
		case int64:
			res = append(res, targetType)
		case bool:
			res = append(res, targetType)
		case float64:
			res = append(res, targetType)
		case any:
			res = append(res, targetType)
		default:
			return nil, fmt.Errorf("unsupported type of target field")
		}

		appendGetterFn := func(g ottl.Getter[K]) error {
			v, err := g.Get(ctx, tCtx)
			if err != nil {
				return err
			}
			res = append(res, v)
			return nil
		}

		if !value.IsEmpty() {
			getter := value.Get()
			if err := appendGetterFn(getter); err != nil {
				return nil, err
			}
		}
		if !values.IsEmpty() {
			getters := values.Get()
			for _, g := range getters {
				if err := appendGetterFn(g); err != nil {
					return nil, err
				}
			}
		}

		// retype []any to Slice, having []any sometimes misbehaves and nils pcommon.Value
		resSlice := pcommon.NewSlice()
		resSlice.FromRaw(res)
		return nil, target.Set(ctx, tCtx, resSlice)
	}, nil
}

func appendMultiple[K any](target []any, values []K) []any {
	for _, v := range values {
		target = append(target, v)
	}
	return target
}
