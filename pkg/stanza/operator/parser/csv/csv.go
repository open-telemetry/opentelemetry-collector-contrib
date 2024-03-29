// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package csv // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/csv"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/parseutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "csv_parser"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new csv parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new csv parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a csv parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`

	Header          string `mapstructure:"header"`
	HeaderDelimiter string `mapstructure:"header_delimiter"`
	HeaderAttribute string `mapstructure:"header_attribute"`
	FieldDelimiter  string `mapstructure:"delimiter"`
	LazyQuotes      bool   `mapstructure:"lazy_quotes"`
	IgnoreQuotes    bool   `mapstructure:"ignore_quotes"`
}

// Build will build a csv parser operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.FieldDelimiter == "" {
		c.FieldDelimiter = ","
	}

	if c.HeaderDelimiter == "" {
		c.HeaderDelimiter = c.FieldDelimiter
	}

	if c.IgnoreQuotes && c.LazyQuotes {
		return nil, errors.New("only one of 'ignore_quotes' or 'lazy_quotes' can be true")
	}

	if len([]rune(c.FieldDelimiter)) != 1 {
		return nil, fmt.Errorf("invalid 'delimiter': '%s'", c.FieldDelimiter)
	}

	if len([]rune(c.HeaderDelimiter)) != 1 {
		return nil, fmt.Errorf("invalid 'header_delimiter': '%s'", c.HeaderDelimiter)
	}

	if c.Header == "" && c.HeaderAttribute == "" {
		return nil, errors.New("missing required field 'header' or 'header_attribute'")
	}

	if c.Header != "" && c.HeaderAttribute != "" {
		return nil, errors.New("only one header parameter can be set: 'header' or 'header_attribute'")
	}

	if c.Header != "" && !strings.Contains(c.Header, c.HeaderDelimiter) {
		return nil, errors.New("missing field delimiter in header")
	}

	p := &Parser{
		ParserOperator:  parserOperator,
		headerAttribute: c.HeaderAttribute,
		fieldDelimiter:  []rune(c.FieldDelimiter)[0],
		headerDelimiter: []rune(c.HeaderDelimiter)[0],
		lazyQuotes:      c.LazyQuotes,
		ignoreQuotes:    c.IgnoreQuotes,
	}

	if c.Header != "" {
		p.parse = generateParseFunc(strings.Split(c.Header, c.HeaderDelimiter), p.fieldDelimiter, c.LazyQuotes, c.IgnoreQuotes)
	}

	return p, nil
}

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

// Process will parse an entry for csv.
func (r *Parser) Process(ctx context.Context, e *entry.Entry) error {
	// Static parse function
	if r.parse != nil {
		return r.ParserOperator.ProcessWith(ctx, e, r.parse)
	}

	// Dynamically generate the parse function based on a header attribute
	h, ok := e.Attributes[r.headerAttribute]
	if !ok {
		err := fmt.Errorf("failed to read dynamic header attribute %s", r.headerAttribute)
		r.Error(err)
		return err
	}
	headerString, ok := h.(string)
	if !ok {
		err := fmt.Errorf("header is expected to be a string but is %T", h)
		r.Error(err)
		return err
	}
	headers := strings.Split(headerString, string([]rune{r.headerDelimiter}))
	parse := generateParseFunc(headers, r.fieldDelimiter, r.lazyQuotes, r.ignoreQuotes)
	return r.ParserOperator.ProcessWith(ctx, e, parse)
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
