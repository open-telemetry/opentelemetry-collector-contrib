// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type CompileMapArguments[K any] struct {
	Object  ottl.Getter[K]
	Pattern ottl.StringGetter[K]
}

func NewCompileMapFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("compileMap", &CompileMapArguments[K]{}, createCompileMapFunction[K])
}

func createCompileMapFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*CompileMapArguments[K])

	if !ok {
		return nil, fmt.Errorf("CompileFactory args must be of type *CompileArguments[K]")
	}

	return compileMap(args.Object, args.Pattern), nil
}

func compileMap[K any](object ottl.Getter[K], pattern ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := object.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		val2, err := pattern.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		var result map[string]any
		switch r := val.(type) {
		case pcommon.Map:
			result = r.AsRaw()
		case map[string]any:
			result = r
		default:
			return nil, fmt.Errorf("type, %T, not supported for compileMap function", r)
		}
		return compileTarget(result, val2)
	}
}

func compileTarget(object map[string]any, pattern string) (pcommon.Map, error) {
	tmpMap := make(map[string]any, len(object))
	res := pcommon.NewMap()
	for k, v := range object {
		matched, err := regexp.MatchString(pattern, k)
		if err != nil {
			return res, err
		}
		if matched {
			tmpMap[k] = v
		}
	}

	if err := res.FromRaw(tmpMap); err != nil {
		return res, err
	}
	return res, nil
}
