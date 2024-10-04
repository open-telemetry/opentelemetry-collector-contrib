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
		mapObject, err := object.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		stringPattern, err := pattern.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		res := pcommon.NewMap()
		var dataMap map[string]any

		switch r := mapObject.(type) {
		case pcommon.Map:
			dataMap = r.AsRaw()
		case map[string]any:
			dataMap = r
		default:
			return res, fmt.Errorf("type, %T, not supported for compileMap function", r)
		}

		tmpResult, err := compileTarget(dataMap, stringPattern)
		if err != nil {
			return res, err
		}

		if err := res.FromRaw(tmpResult); err != nil {
			return res, err
		}

		return res, nil
	}
}

func compileTarget(object map[string]any, pattern string) (map[string]any, error) {
	result := make(map[string]any, len(object))
	for k, v := range object {
		matched, err := regexp.MatchString(pattern, k)
		if err != nil {
			return result, err
		}
		if matched {
			switch r := v.(type) {
			case pcommon.Map:
				m, err := compileTarget(r.AsRaw(), pattern)
				if err != nil {
					return result, err
				}
				result[k] = m
			case map[string]any:
				m, err := compileTarget(r, pattern)
				if err != nil {
					return result, err
				}
				result[k] = m
			default:
				result[k] = v
			}
		}
	}

	return result, nil
}
