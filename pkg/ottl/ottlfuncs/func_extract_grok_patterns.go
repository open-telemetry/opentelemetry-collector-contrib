// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"strings"

	"github.com/elastic/go-grok"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ExtractGrokPatternsArguments[K any] struct {
	Target             ottl.StringGetter[K]
	Pattern            string
	NamedCapturesOnly  ottl.Optional[bool]
	PatternDefinitions ottl.Optional[[]ottl.StringGetter[K]]
}

func NewExtractGrokPatternsFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ExtractGrokPatterns", &ExtractGrokPatternsArguments[K]{}, createExtractGrokPatternsFunction[K])
}

func createExtractGrokPatternsFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ExtractGrokPatternsArguments[K])

	if !ok {
		return nil, fmt.Errorf("ExtractGrokPatternsFactory args must be of type *ExtractGrokPatternsArguments[K]")
	}

	return extractGrokPatterns(args.Target, args.Pattern, args.NamedCapturesOnly, args.PatternDefinitions)
}

func extractGrokPatterns[K any](target ottl.StringGetter[K], pattern string, nco ottl.Optional[bool], patternDefinitions ottl.Optional[[]ottl.StringGetter[K]]) (ottl.ExprFunc[K], error) {
	g := grok.NewComplete()
	namedCapturesOnly := !nco.IsEmpty() && nco.Get()

	return func(ctx context.Context, tCtx K) (any, error) {
		if !patternDefinitions.IsEmpty() {
			getters := patternDefinitions.Get()

			for i, getter := range getters {
				val, err := getter.Get(ctx, tCtx)
				if err != nil {
					return nil, fmt.Errorf("failed to get pattern supplied to ExtractGrokPatterns at index %d: %w", i, err)
				}
				// split pattern in format key=val
				parts := strings.SplitN(val, "=", 2)
				if len(parts) == 1 {
					return nil, fmt.Errorf("pattern supplied to ExtractGrokPatterns at index %d has incorrect format, expecting PATTERNNAME=pattern definition", i)
				}

				if strings.ContainsRune(parts[0], ':') {
					return nil, fmt.Errorf("pattern ID %q should not contain ':'", parts[0])
				}

				g.AddPattern(parts[0], parts[1])
			}
		}

		err := g.Compile(pattern, namedCapturesOnly)
		if err != nil {
			return nil, fmt.Errorf("the pattern supplied to ExtractGrokPatterns is not a valid pattern: %w", err)
		}

		if namedCapturesOnly && !g.HasCaptureGroups() {
			return nil, fmt.Errorf("at least 1 named capture group must be supplied in the given regex")
		}

		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		matches, err := g.ParseTypedString(val)
		if err != nil {
			return nil, err
		}

		result := pcommon.NewMap()
		for k, v := range matches {
			switch val := v.(type) {
			case bool:
				result.PutBool(k, val)
			case float64:
				result.PutDouble(k, val)
			case int64:
				result.PutInt(k, val)
			case int:
				result.PutInt(k, int64(val))
			case string:
				result.PutStr(k, val)
			}
		}

		return result, err
	}, nil
}
