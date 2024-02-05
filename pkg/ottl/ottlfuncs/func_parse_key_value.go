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

	pair_delimiter := " "
	if !p.IsEmpty() {
		pair_delimiter = p.Get()
	}

	if pair_delimiter == delimiter {
		return nil, fmt.Errorf("pair delimiter \"%s\" cannot be equal to delimiter \"%s\"", pair_delimiter, delimiter)
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		source, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if source == "" {
			return nil, fmt.Errorf("cannot parse from empty target")
		}

		pairs, err := splitString(source, pair_delimiter)
		if err != nil {
			return nil, fmt.Errorf("splitting pairs failed: %w", err)
		}

		parsed := make(map[string]any)
		for _, p := range pairs {
			pair := strings.SplitN(p, delimiter, 2)
			if len(pair) != 2 {
				return nil, fmt.Errorf("cannot split '%s' into 2 items, got %d", p, len(pair))
			}
			key := strings.TrimSpace(strings.Trim(pair[0], "\"'"))
			value := strings.TrimSpace(strings.Trim(pair[1], "\"'"))
			parsed[key] = value
		}

		result := pcommon.NewMap()
		err = result.FromRaw(parsed)
		return result, err
	}, nil
}

// splitString will split the input on the delimiter and return the resulting slice.
// `strings.Split` is not used because it does not respect quotes and will split if the delimiter appears in a quoted value
func splitString(input, delimiter string) ([]string, error) {
	var result []string
	quoteType := ""
	inQuotes := false
	currentPair := ""
	delimiterLength := len(delimiter)

	i := 0
	for i < len(input) {
		if !inQuotes && i+delimiterLength <= len(input) && input[i:i+delimiterLength] == delimiter {
			if currentPair == "" {
				i++
				continue
			}
			result = append(result, currentPair)
			currentPair = ""
			i += delimiterLength
			continue
		} else if input[i] == '"' || input[i] == '\'' {
			if inQuotes {
				if quoteType == string(input[i]) {
					inQuotes = !inQuotes
				}
			} else {
				quoteType = string(input[i])
				inQuotes = !inQuotes
			}
		}
		currentPair += string(input[i])
		i++
	}

	if inQuotes {
		return nil, fmt.Errorf("never reached end of a quoted value, failed to parse input")
	}

	if currentPair != "" {
		result = append(result, currentPair)
	}

	return result, nil
}
