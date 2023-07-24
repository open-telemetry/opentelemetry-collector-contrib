// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type LenArguments[K any] struct {
	Target ottl.Getter[K] `ottlarg:"0"`
}

func NewLenFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Len", &LenArguments[K]{}, createLenFunction[K])
}

func createLenFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*LenArguments[K])

	if !ok {
		return nil, fmt.Errorf("LenFactory args must be of type *LenArguments[K]")
	}

	return computeLen(args.Target), nil
}

func computeLen[K any](target ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		v := reflect.ValueOf(val)
		if v.Kind() == reflect.String {
			return int64(len(v.String())), nil
		} else if v.Kind() == reflect.Slice {
			return int64(v.Len()), nil
		}

		if pcommonVal, ok := val.(pcommon.Value); ok {
			if pcommonVal.Type() == pcommon.ValueTypeStr {
				return int64(len(pcommonVal.Str())), nil
			}
		}
		if pcommonSlice, ok := val.(pcommon.Slice); ok {
			return int64(pcommonSlice.Len()), nil
		}

		return nil, fmt.Errorf("target arg must be of type string, []any, or pcommon.Slice")
	}
}
