// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type ParseKeyValueArguments[K any] struct {
	Target        ottl.StringGetter[K]
	Delimiter     ottl.Optional[string]
	PairDelimiter ottl.Optional[string]
}

func NewParseKeyValueFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseKeyValue", &ParseKeyValueArguments[K]{}, createParseKeyValueFunction[K])
}

func createParseKeyValueFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseKeyValueArguments[K])

	if !ok {
		return nil, fmt.Errorf("ParseKeyValueFactory args must be of type *ParseKeyValueArguments[K]")
	}

	return parseKeyValue[K](args.Target, args.Delimiter, args.PairDelimiter)
}

func parseKeyValue[K any](target ottl.StringGetter[K], d ottl.Optional[string], p ottl.Optional[string]) (ottl.ExprFunc[K], error) {
	delimiter := "="
	if !d.IsEmpty() {
		delimiter = d.Get()
	}

	pairDelimiter := " "
	if !p.IsEmpty() {
		pairDelimiter = p.Get()
	}

	if pairDelimiter == delimiter {
		return nil, fmt.Errorf("pair delimiter \"%s\" cannot be equal to delimiter \"%s\"", pairDelimiter, delimiter)
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		source, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if source == "" {
			return nil, fmt.Errorf("cannot parse from empty target")
		}

		pairs, err := splitPairs(source, pairDelimiter)
		if err != nil {
			return nil, fmt.Errorf("splitting pairs failed: %w", err)
		}

		parsed := make(map[string]any)
		for _, p := range pairs {
			pair := strings.SplitN(p, delimiter, 2)
			if len(pair) != 2 {
				return nil, fmt.Errorf("cannot split '%s' into 2 items, got %d", p, len(pair))
			}
			key := strings.TrimSpace(pair[0])
			value := strings.TrimSpace(pair[1])
			parsed[key] = value
		}

		result := pcommon.NewMap()
		err = result.FromRaw(parsed)
		return result, err
	}, nil
}

// splitPairs will split the input on the pairDelimiter and return the resulting slice.
// `strings.Split` is not used because it does not respect quotes and will split if the delimiter appears in a quoted value
func splitPairs(input, pairDelimiter string) ([]string, error) {
	var result []string
	currentPair := ""
	delimiterLength := len(pairDelimiter)
	quoteChar := "" // "" means we are not in quotes

	for i := 0; i < len(input); i++ {
		if quoteChar == "" && i+delimiterLength <= len(input) && input[i:i+delimiterLength] == pairDelimiter { // delimiter
			if currentPair == "" { // leading || trailing delimiter; ignore
				continue
			}
			result = append(result, currentPair)
			currentPair = ""
			i += delimiterLength - 1
			continue
		}

		if quoteChar == "" && (input[i] == '"' || input[i] == '\'') { // start of quote
			quoteChar = string(input[i])
			continue
		}
		if string(input[i]) == quoteChar { // end of quote
			quoteChar = ""
			continue
		}

		currentPair += string(input[i])
	}

	if quoteChar != "" { // check for closed quotes
		return nil, fmt.Errorf("never reached end of a quoted value")
	}
	if currentPair != "" { // avoid adding empty value bc of a trailing delimiter
		return append(result, currentPair), nil
	}

	return result, nil
}
