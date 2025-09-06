// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type StartsWithArguments[K any] struct {
	Target        ottl.StringGetter[K]
	Prefixes      []string
	CaseSensitive bool
}

func NewStartsWithFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory(
		"StartsWith",
		&StartsWithArguments[K]{},
		createStartsWithFunction[K],
	)
}

func createStartsWithFunction[K any](
	_ ottl.FunctionContext,
	oArgs ottl.Arguments,
) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*StartsWithArguments[K])

	if !ok {
		return nil, fmt.Errorf(
			"NewStartsWithFactory args must be of type *StartsWithArguments[K]",
		)
	}

	return startsWith(args.Target, args.Prefixes, args.CaseSensitive)
}

func startsWithAny(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func startsWith[K any](
	target ottl.StringGetter[K],
	prefixes []string,
	caseSensitive bool,
) (ottl.ExprFunc[K], error) {
	var processedPrefixes []string
	if !caseSensitive {
		processedPrefixes = make([]string, len(prefixes))
		for i, prefix := range prefixes {
			processedPrefixes[i] = strings.ToLower(prefix)
		}
	} else {
		processedPrefixes = prefixes
	}
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			var typeError ottl.TypeError
			switch {
			case errors.As(err, &typeError):
				return false, nil
			default:
				return false, err
			}
		}
		if val == "" {
			return false, nil
		}
		if !caseSensitive {
			val = strings.ToLower(val)
		}
		return startsWithAny(val, processedPrefixes), nil
	}, nil
}
