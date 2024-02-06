// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

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

		headers := strings.Split(headerStr, headerDelimiterString)

		csvReader := csv.NewReader(strings.NewReader(targetStr))
		csvReader.Comma = delimiter
		csvReader.FieldsPerRecord = len(headers)
		csvReader.LazyQuotes = lazyQuotes

		fields, err := csvReadLine(csvReader)
		if err != nil {
			return nil, fmt.Errorf("read csv line: %w", err)
		}

		return csvHeadersMap(headers, fields)
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

		headers := strings.Split(headerStr, headerDelimiterString)

		// Ignoring quotes makes CSV parseable with just string.Split
		fields := strings.Split(targetStr, delimiterString)
		return csvHeadersMap(headers, fields)
	}
}

// csvHeadersMap creates a map of headers[i] -> fields[i].
func csvHeadersMap(headers []string, fields []string) (pcommon.Map, error) {
	pMap := pcommon.NewMap()
	parsedValues := make(map[string]any)

	if len(fields) != len(headers) {
		return pMap, fmt.Errorf("wrong number of fields: expected %d, found %d", len(headers), len(fields))
	}

	for i, val := range fields {
		parsedValues[headers[i]] = val
	}

	err := pMap.FromRaw(parsedValues)
	if err != nil {
		return pMap, fmt.Errorf("create pcommon.Map: %w", err)
	}

	return pMap, nil
}

// csvReadLine reads a CSV line from the csv reader, returning the fields parsed from the line.
// We make the assumption that the payload we are reading is a single row, so we allow newline characters in fields.
// However, the csv package does not support newlines in a CSV field (it assumes rows are newline separated),
// so in order to support parsing newlines in a field, we need to stitch together the results of multiple Read calls.
func csvReadLine(csvReader *csv.Reader) ([]string, error) {
	lines := make([][]string, 0, 1)
	for {
		line, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil && len(line) == 0 {
			return nil, fmt.Errorf("read csv line: %w", err)
		}

		lines = append(lines, line)
	}

	/*
		This parser is parsing a single value, which came from a single log entry.
		Therefore, if there are multiple lines here, it should be assumed that each
		subsequent line contains a continuation of the last field in the previous line.

		Given a file w/ headers "A,B,C,D,E" and contents "aa,b\nb,cc,d\nd,ee",
		expect reader.Read() to return bodies:
		- ["aa","b"]
		- ["b","cc","d"]
		- ["d","ee"]
	*/

	joinedLine := lines[0]
	for i := 1; i < len(lines); i++ {
		nextLine := lines[i]

		// The first element of the next line is a continuation of the previous line's last element
		joinedLine[len(joinedLine)-1] += "\n" + nextLine[0]

		// The remainder are separate elements
		for n := 1; n < len(nextLine); n++ {
			joinedLine = append(joinedLine, nextLine[n])
		}
	}

	return joinedLine, nil
}
