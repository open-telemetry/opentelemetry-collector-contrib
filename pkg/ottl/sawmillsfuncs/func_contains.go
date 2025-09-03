package sawmillsfuncs

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ContainsArguments[K any] struct {
	Target        ottl.StringGetter[K]
	Patterns      []string
	CaseSensitive bool
}

func NewContainsFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory(
		"Contains",
		&ContainsArguments[K]{},
		createContainsFunction[K],
	)
}

func createContainsFunction[K any](
	_ ottl.FunctionContext,
	oArgs ottl.Arguments,
) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ContainsArguments[K])

	if !ok {
		return nil, fmt.Errorf(
			"NewContainsFactory args must be of type *ContainsArguments[K]",
		)
	}

	return contains(args.Target, args.Patterns, args.CaseSensitive)
}

func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func contains[K any](
	target ottl.StringGetter[K],
	patterns []string,
	caseSensitive bool,
) (ottl.ExprFunc[K], error) {
	var processedPatterns []string
	if !caseSensitive {
		processedPatterns = make([]string, len(patterns))
		for i, pattern := range patterns {
			processedPatterns[i] = strings.ToLower(pattern)
		}
	} else {
		processedPatterns = patterns
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
		return containsAny(val, processedPatterns), nil
	}, nil
}
