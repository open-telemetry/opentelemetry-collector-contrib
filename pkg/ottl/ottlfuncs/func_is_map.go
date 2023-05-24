// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsMapArguments[K any] struct {
	Target ottl.Getter[K] `ottlarg:"0"`
}

func NewIsMapFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsMap", &IsMapArguments[K]{}, createIsMapFunction[K])
}

func createIsMapFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsMapArguments[K])

	if !ok {
		return nil, fmt.Errorf("IsMapFactory args must be of type *IsMapArguments[K]")
	}

	return isMap(args.Target), nil
}

func isMap[K any](target ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		value, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if value == nil {
			return false, nil
		}
		val, ok := value.(pcommon.Value)
		if ok {
			return val.Type() == pcommon.ValueTypeMap, nil
		}
		_, ok = value.(map[string]any)
		return ok, nil
	}
}
