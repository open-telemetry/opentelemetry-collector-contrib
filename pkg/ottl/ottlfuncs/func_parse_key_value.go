// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/parseutils"
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
	if !d.IsEmpty() && d.Get() != "" {
		delimiter = d.Get()
	}

	pairDelimiter := " "
	if !p.IsEmpty() && p.Get() != "" {
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

		if source == "" {
			return nil, fmt.Errorf("cannot parse from empty target")
		}

		pairs, err := parseutils.SplitString(source, pairDelimiter)
		if err != nil {
			return nil, fmt.Errorf("splitting source %q into pairs failed: %w", source, err)
		}

		parsed := make(map[string]any)
		for _, p := range pairs {
			pair := strings.SplitN(p, delimiter, 2)
			if len(pair) != 2 {
				return nil, fmt.Errorf("cannot split %q into 2 items, got %d item(s)", p, len(pair))
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
