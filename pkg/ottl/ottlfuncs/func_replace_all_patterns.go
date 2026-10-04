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
					updatedString, err := applyOptReplaceFunction(ctx, tCtx, cp, fn, value.Str(), replacementVal, replacementFormat)
					if err != nil {
						continue
					}
					value.SetStr(updatedString)
				} else {
					value.SetStr(cp.ReplaceAllString(value.Str(), replacementVal))
				}
			}
		case modeKey:
			// Because we are changing the keys we cannot do in-place update, but we can move values to the
			// updated map and then move back the updated map to the initial map to avoid a copy in the target.Set,
			// because the pcommon.Map.CopyTo will not do a copy if it is the same object in this case val.
			// The updated map is assembled through an xpdata.MapBuilder, which appends without the per-key
			// duplicate check of pcommon.Map.PutEmpty (a linear scan), keeping this function linear in the size
			// of the map; seenKeys deduplicates renamed keys, preserving the previous last-writer-wins collision
			// behavior where a later entry overwrites the value of an earlier one under the same final key.
			var updated xpdata.MapBuilder
			updated.EnsureCapacity(val.Len())
			seenKeys := make(map[string]pcommon.Value, val.Len())
			for key, value := range val.All() {
				newKey := key
				if cp.MatchString(key) {
					if !fn.IsEmpty() {
						var err error
						if newKey, err = applyOptReplaceFunction(ctx, tCtx, cp, fn, key, replacementVal, replacementFormat); err != nil {
							continue
						}
					} else {
						newKey = cp.ReplaceAllString(key, replacementVal)
					}
				}
				if seenValue, ok := seenKeys[newKey]; ok {
					value.MoveTo(seenValue)
					continue
				}
				newValue := updated.AppendEmpty(newKey)
				value.MoveTo(newValue)
				seenKeys[newKey] = newValue
			}
			updated.UnsafeIntoMap(val)
		}
		return nil, target.Set(ctx, tCtx, val)
	}, nil
}
