// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package csv // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/csv"

import (
	"context"
	csvparser "encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.uber.org/zap"

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

	fieldDelimiter := []rune(c.FieldDelimiter)[0]
	headerDelimiter := []rune(c.HeaderDelimiter)[0]

	if len([]rune(c.FieldDelimiter)) != 1 {
		return nil, fmt.Errorf("invalid 'delimiter': '%s'", c.FieldDelimiter)
	}

	if len([]rune(c.HeaderDelimiter)) != 1 {
		return nil, fmt.Errorf("invalid 'header_delimiter': '%s'", c.HeaderDelimiter)
	}

	var headers []string
	switch {
	case c.Header == "" && c.HeaderAttribute == "":
		return nil, errors.New("missing required field 'header' or 'header_attribute'")
	case c.Header != "" && c.HeaderAttribute != "":
		return nil, errors.New("only one header parameter can be set: 'header' or 'header_attribute'")
	case c.Header != "" && !strings.Contains(c.Header, c.HeaderDelimiter):
		return nil, errors.New("missing field delimiter in header")
	case c.Header != "":
		headers = strings.Split(c.Header, c.HeaderDelimiter)
	}

	return &Parser{
		ParserOperator:  parserOperator,
		header:          headers,
		headerAttribute: c.HeaderAttribute,
		fieldDelimiter:  fieldDelimiter,
		headerDelimiter: headerDelimiter,
		lazyQuotes:      c.LazyQuotes,
		ignoreQuotes:    c.IgnoreQuotes,
		parse:           generateParseFunc(headers, fieldDelimiter, c.LazyQuotes, c.IgnoreQuotes),
	}, nil
}

// Parser is an operator that parses csv in an entry.
type Parser struct {
	helper.ParserOperator
	fieldDelimiter  rune
	headerDelimiter rune
	header          []string
	headerAttribute string
	lazyQuotes      bool
	ignoreQuotes    bool
	parse           parseFunc
}

type parseFunc func(interface{}) (interface{}, error)

// Process will parse an entry for csv.
func (r *Parser) Process(ctx context.Context, e *entry.Entry) error {
	parse := r.parse

	// If we have a headerAttribute set we need to dynamically generate our parser function
	if r.headerAttribute != "" {
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
		parse = generateParseFunc(headers, r.fieldDelimiter, r.lazyQuotes, r.ignoreQuotes)
	}

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
	return func(value interface{}) (interface{}, error) {
		csvLine, err := valueAsString(value)
		if err != nil {
			return nil, err
		}

		reader := csvparser.NewReader(strings.NewReader(csvLine))
		reader.Comma = fieldDelimiter
		reader.FieldsPerRecord = len(headers)
		reader.LazyQuotes = lazyQuotes

		// Typically only need one
		lines := make([][]string, 0, 1)
		for {
			line, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			}

			if err != nil && len(line) == 0 {
				return nil, errors.New("failed to parse entry")
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

		return headersMap(headers, joinedLine)
	}
}

// generateSplitParseFunc returns a parse function (which ignores quotes) for a given header and field delimiter.
func generateSplitParseFunc(headers []string, fieldDelimiter rune) parseFunc {
	return func(value interface{}) (interface{}, error) {
		csvLine, err := valueAsString(value)
		if err != nil {
			return nil, err
		}

		// This parse function does not do any special quote handling; Splitting on the delimiter is sufficient.
		fields := strings.Split(csvLine, string(fieldDelimiter))
		return headersMap(headers, fields)
	}
}

// valueAsString interprets the given value as a string.
func valueAsString(value interface{}) (string, error) {
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

// headersMap creates a map of headers[i] -> fields[i].
func headersMap(headers []string, fields []string) (map[string]interface{}, error) {
	parsedValues := make(map[string]interface{})

	if len(fields) != len(headers) {
		return nil, fmt.Errorf("wrong number of fields: expected %d, found %d", len(headers), len(fields))
	}

	for i, val := range fields {
		parsedValues[headers[i]] = val
	}
	return parsedValues, nil
}
