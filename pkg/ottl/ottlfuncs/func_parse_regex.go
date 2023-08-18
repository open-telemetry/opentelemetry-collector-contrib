package ottlfuncs

import (
	"context"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type ParseRegexArguments[K any] struct {
	Target  ottl.StringGetter[K] `ottlarg:"0"`
	Pattern string               `ottlarg:"1"`
}

func NewParseRegexFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseRegex", &ParseRegexArguments[K]{}, createParseRegexFunction[K])
}

func createParseRegexFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseRegexArguments[K])

	if !ok {
		return nil, fmt.Errorf("ParseRegexFactory args must be of type *ParseRegexArguments[K]")
	}

	return parseRegex(args.Target, args.Pattern)
}

func parseRegex[K any](target ottl.StringGetter[K], pattern string) (ottl.ExprFunc[K], error) {
	r, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("the pattern supplied to ParseRegex is not a valid pattern: %w", err)
	}

	namedCaptureGroups := 0
	for _, groupName := range r.SubexpNames() {
		if groupName != "" {
			namedCaptureGroups++
		}
	}

	if namedCaptureGroups == 0 {
		return nil, fmt.Errorf("no named capture groups in regex pattern")
	}

	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		matches := r.FindStringSubmatch(val)
		if matches == nil {
			return nil, fmt.Errorf("regex pattern does not match")
		}

		parsedValues := map[string]interface{}{}
		for i, subexp := range r.SubexpNames() {
			if i == 0 {
				// Skip whole match
				continue
			}
			if subexp != "" {
				parsedValues[subexp] = matches[i]
			}
		}

		result := pcommon.NewMap()
		err = result.FromRaw(parsedValues)
		return result, err
	}, nil
}
