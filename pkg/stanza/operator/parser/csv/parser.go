// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package csv // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/csv"

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/parseutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses csv in an entry.
type Parser struct {
	helper.ParserOperator
	fieldDelimiter  rune
	headerDelimiter rune
	headerAttribute string
	lazyQuotes      bool
	ignoreQuotes    bool
	parse           parseFunc
}

type parseFunc func(any) (any, error)

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.Process)
}

// Process will parse an entry for csv.
func (p *Parser) Process(ctx context.Context, e *entry.Entry) error {
	// Static parse function
	if p.parse != nil {
		return p.ProcessWith(ctx, e, p.parse)
	}

	// Dynamically generate the parse function based on a header attribute
	h, ok := e.Attributes[p.headerAttribute]
	if !ok {
		p.Logger().Error("read dynamic header attribute", zap.String("attribute", p.headerAttribute))
		return fmt.Errorf("failed to read dynamic header attribute %s", p.headerAttribute)
	}
	headerString, ok := h.(string)
	if !ok {
		p.Logger().Error("header must be string", zap.String("type", fmt.Sprintf("%T", h)))
		return fmt.Errorf("header is expected to be a string but is %T", h)
	}
	headers := strings.Split(headerString, string([]rune{p.headerDelimiter}))
	parse := generateParseFunc(headers, p.fieldDelimiter, p.lazyQuotes, p.ignoreQuotes)
	return p.ProcessWith(ctx, e, parse)
}

// generateParseFunc returns a parse function for a given header, allowing
// each entry to have a potentially unique set of fields when using dynamic
// field names retrieved from an entry's attribute
func generateParseFunc(headers []string, fieldDelimiter rune, lazyQuotes bool, ignoreQuotes bool) parseFunc {
	if ignoreQuotes {
		return generateSplitParseFunc(headers, fieldDelimiter)
	}
	return generateCSVParseFunc(headers, fieldDelimiter, lazyQuotes)
}

// generateCSVParseFunc returns a parse function for a given header and field delimiter, which parses a line of CSV text.
func generateCSVParseFunc(headers []string, fieldDelimiter rune, lazyQuotes bool) parseFunc {
	return func(value any) (any, error) {
		csvLine, err := valueAsString(value)
		if err != nil {
			return nil, err
		}

		joinedLine, err := parseutils.ReadCSVRow(csvLine, fieldDelimiter, lazyQuotes)
		if err != nil {
			return nil, err
		}

		return parseutils.MapCSVHeaders(headers, joinedLine)
	}
}

// generateSplitParseFunc returns a parse function (which ignores quotes) for a given header and field delimiter.
func generateSplitParseFunc(headers []string, fieldDelimiter rune) parseFunc {
	return func(value any) (any, error) {
		csvLine, err := valueAsString(value)
		if err != nil {
			return nil, err
		}

		// This parse function does not do any special quote handling; Splitting on the delimiter is sufficient.
		fields := strings.Split(csvLine, string(fieldDelimiter))
		return parseutils.MapCSVHeaders(headers, fields)
	}
}

// valueAsString interprets the given value as a string.
func valueAsString(value any) (string, error) {
	var s string
	switch t := value.(type) {
	case string:
		s += t
	case []byte:
		s += string(t)
	default:
		return s, fmt.Errorf("type '%T' cannot be parsed as csv", value)
	}

	return s, nil
}
