// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/parseutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	parseCSVModeStrict       = "strict"
	parseCSVModeLazyQuotes   = "lazyQuotes"
	parseCSVModeIgnoreQuotes = "ignoreQuotes"
)

const (
	parseCSVDefaultDelimiter = ','
	parseCSVDefaultMode      = parseCSVModeStrict
)

type ParseCSVArguments[K any] struct {
	Target          ottl.StringGetter[K]
	Header          ottl.StringGetter[K]
	Delimiter       ottl.Optional[string]
	HeaderDelimiter ottl.Optional[string]
	Mode            ottl.Optional[string]
}

func (p ParseCSVArguments[K]) validate() error {
	if !p.Delimiter.IsEmpty() {
		if len([]rune(p.Delimiter.Get())) != 1 {
			return errors.New("delimiter must be a single character")
		}
	}

	if !p.HeaderDelimiter.IsEmpty() {
		if len([]rune(p.HeaderDelimiter.Get())) != 1 {
			return errors.New("header_delimiter must be a single character")
		}
	}

	return nil
}

func NewParseCSVFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseCSV", &ParseCSVArguments[K]{}, createParseCSVFunction[K])
}

func createParseCSVFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseCSVArguments[K])
	if !ok {
		return nil, fmt.Errorf("ParseCSVFactory args must be of type *ParseCSVArguments[K]")
	}

	if err := args.validate(); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	delimiter := parseCSVDefaultDelimiter
	if !args.Delimiter.IsEmpty() {
		delimiter = []rune(args.Delimiter.Get())[0]
	}

	// headerDelimiter defaults to the chosen delimter,
	// since in most cases headerDelimiter == delmiter.
	headerDelimiter := delimiter
	if !args.HeaderDelimiter.IsEmpty() {
		headerDelimiter = []rune(args.HeaderDelimiter.Get())[0]
	}

	mode := parseCSVDefaultMode
	if !args.Mode.IsEmpty() {
		mode = args.Mode.Get()
	}

	switch mode {
	case parseCSVModeStrict:
		return parseCSV(args.Target, args.Header, delimiter, headerDelimiter, false), nil
	case parseCSVModeLazyQuotes:
		return parseCSV(args.Target, args.Header, delimiter, headerDelimiter, true), nil
	case parseCSVModeIgnoreQuotes:
		return parseCSVIgnoreQuotes(args.Target, args.Header, delimiter, headerDelimiter), nil
	}

	return nil, fmt.Errorf("unknown mode: %s", mode)
}

func parseCSV[K any](target, header ottl.StringGetter[K], delimiter, headerDelimiter rune, lazyQuotes bool) ottl.ExprFunc[K] {
	headerDelimiterString := string([]rune{headerDelimiter})

	return func(ctx context.Context, tCtx K) (any, error) {
		targetStr, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("get target: %w", err)
		}

		headerStr, err := header.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("get header: %w", err)
		}

		if headerStr == "" {
			return nil, errors.New("headers must not be an empty string")
		}

		headers := strings.Split(headerStr, headerDelimiterString)
		fields, err := parseutils.ReadCSVRow(targetStr, delimiter, headers, lazyQuotes)
		if err != nil {
			return nil, err
		}

		headersToFields, err := parseutils.MapCSVHeaders(headers, fields)
		if err != nil {
			return nil, fmt.Errorf("map csv headers: %w", err)
		}

		pMap := pcommon.NewMap()
		err = pMap.FromRaw(headersToFields)
		if err != nil {
			return nil, fmt.Errorf("create pcommon.Map: %w", err)
		}

		return pMap, nil
	}
}

func parseCSVIgnoreQuotes[K any](target, header ottl.StringGetter[K], delimiter, headerDelimiter rune) ottl.ExprFunc[K] {
	headerDelimiterString := string([]rune{headerDelimiter})
	delimiterString := string([]rune{delimiter})

	return func(ctx context.Context, tCtx K) (any, error) {
		targetStr, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("get target: %w", err)
		}

		headerStr, err := header.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("get header: %w", err)
		}

		if headerStr == "" {
			return nil, errors.New("headers must not be an empty string")
		}

		headers := strings.Split(headerStr, headerDelimiterString)

		// Ignoring quotes makes CSV parseable with just string.Split
		fields := strings.Split(targetStr, delimiterString)

		headersToFields, err := parseutils.MapCSVHeaders(headers, fields)
		if err != nil {
			return nil, fmt.Errorf("map csv headers: %w", err)
		}

		pMap := pcommon.NewMap()
		err = pMap.FromRaw(headersToFields)
		if err != nil {
			return nil, fmt.Errorf("create pcommon.Map: %w", err)
		}

		return pMap, nil
	}
}
