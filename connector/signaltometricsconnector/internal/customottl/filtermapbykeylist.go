// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package customottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/customottl"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// FilterMapByKeyListArguments defines the OTTL function signature:
//
//	FilterMapByKeyList(source, key_list, prefixes...)
//
// source   – map to filter (e.g. resource.attributes)
// key_list – comma-separated allow-list; accepts a string, pcommon.Slice
//
//	whose first element is a string, or nil (returns empty map).
//	The special value "*" accepts all prefix-matched keys.
//
// prefixes – one or more prefix strings; only keys starting with a listed
//
//	prefix are subject to filtering (non-matching keys are dropped)
type FilterMapByKeyListArguments[K any] struct {
	Source   ottl.PMapGetter[K]
	KeyList  ottl.Getter[K]
	Prefixes []ottl.StringGetter[K]
}

func NewFilterMapByKeyListFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory(
		"FilterMapByKeyList",
		&FilterMapByKeyListArguments[K]{},
		createFilterMapByKeyListFunction[K],
	)
}

func createFilterMapByKeyListFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*FilterMapByKeyListArguments[K])
	if !ok {
		return nil, errors.New("FilterMapByKeyList args must be of type *FilterMapByKeyListArguments[K]")
	}
	if len(args.Prefixes) == 0 {
		return nil, errors.New("FilterMapByKeyList requires at least one prefix")
	}
	return filterMapByKeyList(args.Source, args.KeyList, args.Prefixes), nil
}

func filterMapByKeyList[K any](
	source ottl.PMapGetter[K],
	keyList ottl.Getter[K],
	prefixGetters []ottl.StringGetter[K],
) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		src, err := source.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		rawVal, err := keyList.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		raw := resolveKeyListString(rawVal)
		allowKeys := parseKeyList(raw)

		prefixes := make([]string, 0, len(prefixGetters))
		for _, pg := range prefixGetters {
			p, err := pg.Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			prefixes = append(prefixes, p)
		}

		_, isWildcard := allowKeys["*"]

		result := pcommon.NewMap()
		src.Range(func(k string, v pcommon.Value) bool {
			for _, prefix := range prefixes {
				if base, ok := strings.CutPrefix(k, prefix); ok {
					if isWildcard {
						// Wildcard: accept all prefix-matched keys
						v.CopyTo(result.PutEmpty(k))
					} else if _, found := allowKeys[base]; found {
						// Allow-list: accept only listed base keys
						v.CopyTo(result.PutEmpty(k))
					}
					return true
				}
			}
			return true
		})
		return result, nil
	}
}

// resolveKeyListString extracts a string from the raw OTTL getter
// result. Handles string, pcommon.Slice (takes first element), and nil.
func resolveKeyListString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case pcommon.Slice:
		if val.Len() > 0 {
			return val.At(0).AsString()
		}
		return ""
	case nil:
		return ""
	default:
		// Defensive fallback; in practice the OTTL context should only
		// produce string, pcommon.Slice, or nil.
		return fmt.Sprintf("%v", val)
	}
}

// parseKeyList splits a comma-separated string into a set of keys.
// Returns nil when raw is empty; callers rely on nil-map reads
// returning the zero value (false) to reject all candidates.
// The special value "*" returns a sentinel map that signals
// "accept all prefix-matched keys" (see filterMapByKeyList).
func parseKeyList(raw string) map[string]struct{} {
	if raw == "" {
		return nil
	}
	if raw == "*" {
		return map[string]struct{}{"*": {}}
	}
	parts := strings.Split(raw, ",")
	keys := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		if k := strings.TrimSpace(p); k != "" {
			keys[k] = struct{}{}
		}
	}
	return keys
}
