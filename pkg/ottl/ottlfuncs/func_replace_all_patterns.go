// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/xpdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	modeKey   = "key"
	modeValue = "value"
)

type replaceAllPatternsArguments[K any] struct {
	Target            ottl.PMapGetSetter[K]
	Mode              string
	RegexPattern      ottl.StringGetter[K]
	Replacement       ottl.StringGetter[K]
	Function          ottl.Optional[ottl.FunctionGetter[K]]
	ReplacementFormat ottl.Optional[ottl.StringGetter[K]]
}

// NewReplaceAllPatternsFactory returns a factory for the replace_all_patterns OTTL function.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md#replace_all_patterns
func NewReplaceAllPatternsFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("replace_all_patterns", &replaceAllPatternsArguments[K]{}, createReplaceAllPatternsFunction[K])
}

func createReplaceAllPatternsFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*replaceAllPatternsArguments[K])

	if !ok {
		return nil, errors.New("ReplaceAllPatternsFactory args must be of type *replaceAllPatternsArguments[K]")
	}

	return replaceAllPatterns(args.Target, args.Mode, args.RegexPattern, args.Replacement, args.Function, args.ReplacementFormat)
}

func replaceAllPatterns[K any](target ottl.PMapGetSetter[K], mode string, regexPattern, replacement ottl.StringGetter[K], fn ottl.Optional[ottl.FunctionGetter[K]], replacementFormat ottl.Optional[ottl.StringGetter[K]]) (ottl.ExprFunc[K], error) {
	compiledPattern, err := newDynamicRegex("replace_all_patterns", regexPattern)
	if err != nil {
		return nil, err
	}
	if mode != modeValue && mode != modeKey {
		return nil, fmt.Errorf("invalid mode %v, must be either 'key' or 'value'", mode)
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		var replacementVal string
		replacementVal, err = replacement.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		cp, err := compiledPattern.compile(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch mode {
		case modeValue:
			for _, value := range val.All() {
				if value.Type() != pcommon.ValueTypeStr || !cp.MatchString(value.Str()) {
					continue
				}
				if !fn.IsEmpty() {
					updatedString, applyErr := applyOptReplaceFunction(ctx, tCtx, cp, fn, value.Str(), replacementVal, replacementFormat)
					if applyErr != nil {
						continue
					}
					value.SetStr(updatedString)
				} else {
					value.SetStr(cp.ReplaceAllString(value.Str(), replacementVal))
				}
			}
		case modeKey:
			hasMatch := false
			for key := range val.All() {
				if cp.MatchString(key) {
					hasMatch = true
					break
				}
			}
			if !hasMatch {
				break
			}

			// Append each unique key without pcommon.Map.PutEmpty's linear duplicate check. Keeping the
			// first destination value for each key preserves entry order while later values overwrite it.
			var updated xpdata.MapBuilder
			updated.EnsureCapacity(val.Len())
			valuesByKey := make(map[string]pcommon.Value, val.Len())
			for key, value := range val.All() {
				updatedKey := key
				if cp.MatchString(key) {
					if !fn.IsEmpty() {
						transformedKey, applyErr := applyOptReplaceFunction(ctx, tCtx, cp, fn, key, replacementVal, replacementFormat)
						if applyErr != nil {
							continue
						}
						updatedKey = transformedKey
					} else {
						updatedKey = cp.ReplaceAllString(key, replacementVal)
					}
				}

				if updatedValue, ok := valuesByKey[updatedKey]; ok {
					value.MoveTo(updatedValue)
					continue
				}

				updatedValue := updated.AppendEmpty(updatedKey)
				value.MoveTo(updatedValue)
				valuesByKey[updatedKey] = updatedValue
			}
			updated.UnsafeIntoMap(val)
		}
		return nil, target.Set(ctx, tCtx, val)
	}, nil
}
