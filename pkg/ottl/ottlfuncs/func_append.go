// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type Appendrguments[K any] struct {
	Target ottl.GetSetter[K]
	Value  ottl.Optional[string]
	Values ottl.Optional[[]string]
}

func NewAppendFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("append", &Appendrguments[K]{}, createAppendFunction[K])
}
func createAppendFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*Appendrguments[K])
	if !ok {
		return nil, fmt.Errorf("AppendFactory args must be of type *Appendrguments[K]")
	}

	return Append(args.Target, args.Value, args.Values)
}

func Append[K any](target ottl.GetSetter[K], value ottl.Optional[string], values ottl.Optional[[]string]) (ottl.ExprFunc[K], error) {
	if value.IsEmpty() && values.IsEmpty() {
		return nil, fmt.Errorf("one of optional arguments needs to be provided")
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		t, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		var res []string

		// init res with target values
		if t == nil {
			// target does not exist, init a slice
			t = make([]string, 0)
		}

		switch targetType := t.(type) {
		case pcommon.Slice:
			res = appendSlice(res, targetType)
		case pcommon.Value:
			switch targetType.Type() {
			case pcommon.ValueTypeStr:
				res = append(res, targetType.Str())
			case pcommon.ValueTypeSlice:
				res = appendSlice(res, targetType.Slice())
			default:
				return nil, fmt.Errorf("unsupported type of target field")
			}
		case []string:
			res = appendStrings(res, targetType)
		case []any:
			res = appendAny(res, targetType)
		case string:
			res = append(res, targetType)
		case any:
			res = append(res, fmt.Sprint(targetType))
		default:
			return nil, fmt.Errorf("unsupported type of target field")
		}

		if !value.IsEmpty() {
			res = append(res, value.Get())
		}
		if !values.IsEmpty() {
			res = append(res, values.Get()...)
		}

		return nil, target.Set(ctx, tCtx, res)
	}, nil
}

func appendSlice(target []string, values pcommon.Slice) []string {
	for i := 0; i < values.Len(); i++ {
		v := values.At(i)
		target = append(target, v.AsString())
	}

	return target
}

func appendStrings(target []string, values []string) []string {
	return append(target, values...)
}

func appendAny(target []string, values []any) []string {
	for _, v := range values {
		target = append(target, fmt.Sprint(v))
	}

	return target
}
