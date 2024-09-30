// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	gosort "sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ToKeyValueStringArguments[K any] struct {
	Target        ottl.PMapGetter[K]
	Delimiter     ottl.Optional[string]
	PairDelimiter ottl.Optional[string]
}

func NewToKeyValueStringFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ToKeyValueString", &ToKeyValueStringArguments[K]{}, createToKeyValueStringFunction[K])
}

func createToKeyValueStringFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ToKeyValueStringArguments[K])

	if !ok {
		return nil, fmt.Errorf("ToKeyValueStringFactory args must be of type *ToKeyValueStringArguments[K]")
	}

	return toKeyValueString[K](args.Target, args.Delimiter, args.PairDelimiter)
}

func toKeyValueString[K any](target ottl.PMapGetter[K], d ottl.Optional[string], p ottl.Optional[string]) (ottl.ExprFunc[K], error) {
	delimiter := "="
	if !d.IsEmpty() {
		if d.Get() == "" {
			return nil, fmt.Errorf("delimiter cannot be set to an empty string")
		}
		delimiter = d.Get()
	}

	pairDelimiter := " "
	if !p.IsEmpty() {
		if p.Get() == "" {
			return nil, fmt.Errorf("pair delimiter cannot be set to an empty string")
		}
		pairDelimiter = p.Get()
	}

	if pairDelimiter == delimiter {
		return nil, fmt.Errorf("pair delimiter %q cannot be equal to delimiter %q", pairDelimiter, delimiter)
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		source, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		kvString := convertMapToKV(source, delimiter, pairDelimiter)

		return kvString, nil
	}, nil
}

// convertMapToKV converts a pcommon.Map to a key value string
func convertMapToKV(target pcommon.Map, delimiter string, pairDelimiter string) string {
	var kvStrings []string
	var keyValues []struct {
		key string
		val pcommon.Value
	}

	// Sort by keys
	target.Range(func(k string, v pcommon.Value) bool {
		keyValues = append(keyValues, struct {
			key string
			val pcommon.Value
		}{key: k, val: v})
		return true
	})
	gosort.Slice(keyValues, func(i, j int) bool {
		return keyValues[i].key < keyValues[j].key
	})

	// Convert KV pairs
	for _, kv := range keyValues {
		k := escapeAndQuoteKV(kv.key, delimiter, pairDelimiter)
		vStr := escapeAndQuoteKV(kv.val.AsString(), delimiter, pairDelimiter)
		kvStrings = append(kvStrings, fmt.Sprintf("%s%s%v", k, delimiter, vStr))
	}

	return strings.Join(kvStrings, pairDelimiter)
}

func escapeAndQuoteKV(s string, delimiter string, pairDelimiter string) string {
	s = strings.ReplaceAll(s, `"`, `\"`)
	if strings.Contains(s, pairDelimiter) || strings.Contains(s, delimiter) {
		s = `"` + s + `"`
	}
	return s
}
