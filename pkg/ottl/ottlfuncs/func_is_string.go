// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsStringArguments[K any] struct {
	Target ottl.Getter[K] `ottlarg:"0"`
}

func NewIsStringFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsString", &IsStringArguments[K]{}, createIsStringFunction[K])
}

func createIsStringFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsStringArguments[K])

	if !ok {
		return nil, fmt.Errorf("IsStringFactory args must be of type *IsStringArguments[K]")
	}

	return isStringFunc(args.Target), nil
}

func isStringFunc[K any](target ottl.Getter[K]) ottl.ExprFunc[K] {
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
			return val.Type() == pcommon.ValueTypeStr, nil
		}
		_, ok = value.(string)
		return ok, nil
	}
}
