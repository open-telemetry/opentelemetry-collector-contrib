// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/sawmillsfuncs"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type EndsWithArguments[K any] struct {
	Target        ottl.StringGetter[K]
	Suffixes      []string
	CaseSensitive bool
}

func NewEndsWithFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory(
		"EndsWith",
		&EndsWithArguments[K]{},
		createEndsWithFunction[K],
	)
}

func createEndsWithFunction[K any](
	_ ottl.FunctionContext,
	oArgs ottl.Arguments,
) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*EndsWithArguments[K])

	if !ok {
		return nil, fmt.Errorf(
			"NewEndsWithFactory args must be of type *EndsWithArguments[K]",
		)
	}

	return endsWith(args.Target, args.Suffixes, args.CaseSensitive)
}

func endsWithAny(s string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

func endsWith[K any](
	target ottl.StringGetter[K],
	suffixes []string,
	caseSensitive bool,
) (ottl.ExprFunc[K], error) {
	var processedSuffixes []string
	if !caseSensitive {
		processedSuffixes = make([]string, len(suffixes))
		for i, suffix := range suffixes {
			processedSuffixes[i] = strings.ToLower(suffix)
		}
	} else {
		processedSuffixes = suffixes
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
		return endsWithAny(val, processedSuffixes), nil
	}, nil
}
