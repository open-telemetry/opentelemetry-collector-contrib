// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package csv

import (
	"context"
	csvparser "encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("csv_parser", func() operator.Builder { return NewCSVParserConfig("") })
}

// NewCSVParserConfig creates a new csv parser config with default values
func NewCSVParserConfig(operatorID string) *CSVParserConfig {
	return &CSVParserConfig{
		ParserConfig: helper.NewParserConfig(operatorID, "csv_parser"),
	}
}

// CSVParserConfig is the configuration of a csv parser operator.
type CSVParserConfig struct {
	helper.ParserConfig `yaml:",inline"`

	Header          string `json:"header" yaml:"header"`
	HeaderAttribute string `json:"header_attribute" yaml:"header_attribute"`
	FieldDelimiter  string `json:"delimiter,omitempty" yaml:"delimiter,omitempty"`
	LazyQuotes      bool   `json:"lazy_quotes,omitempty" yaml:"lazy_quotes,omitempty"`
}

// Build will build a csv parser operator.
func (c CSVParserConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.FieldDelimiter == "" {
		c.FieldDelimiter = ","
	}

	fieldDelimiter := []rune(c.FieldDelimiter)[0]

	if len([]rune(c.FieldDelimiter)) != 1 {
		return nil, fmt.Errorf("invalid 'delimiter': '%s'", c.FieldDelimiter)
	}

	headers := make([]string, 0)
	switch {
	case c.Header == "" && c.HeaderAttribute == "":
		return nil, errors.New("missing required field 'header' or 'header_attribute'")
	case c.Header != "" && c.HeaderAttribute != "":
		return nil, errors.New("only one header parameter can be set: 'header' or 'header_attribute'")
	case c.Header != "" && !strings.Contains(c.Header, c.FieldDelimiter):
		return nil, errors.New("missing field delimiter in header")
	case c.Header != "":
		headers = strings.Split(c.Header, c.FieldDelimiter)
	}

	return &CSVParser{
		ParserOperator:  parserOperator,
		header:          headers,
		headerAttribute: c.HeaderAttribute,
		fieldDelimiter:  fieldDelimiter,
		lazyQuotes:      c.LazyQuotes,

		parse: generateParseFunc(headers, fieldDelimiter, c.LazyQuotes),
	}, nil
}

// CSVParser is an operator that parses csv in an entry.
type CSVParser struct {
	helper.ParserOperator
	fieldDelimiter  rune
	header          []string
	headerAttribute string
	lazyQuotes      bool
	parse           parseFunc
}

type parseFunc func(interface{}) (interface{}, error)

// Process will parse an entry for csv.
func (r *CSVParser) Process(ctx context.Context, e *entry.Entry) error {
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
		headers := strings.Split(headerString, string([]rune{r.fieldDelimiter}))
		parse = generateParseFunc(headers, r.fieldDelimiter, r.lazyQuotes)
	}

	return r.ParserOperator.ProcessWith(ctx, e, parse)
}

// generateParseFunc returns a parse function for a given header, allowing
// each entry to have a potentially unique set of fields when using dynamic
// field names retrieved from an entry's attribute
func generateParseFunc(headers []string, fieldDelimiter rune, lazyQuotes bool) parseFunc {
	return func(value interface{}) (interface{}, error) {
		var csvLine string
		switch t := value.(type) {
		case string:
			csvLine += t
		case []byte:
			csvLine += string(t)
		default:
			return nil, fmt.Errorf("type '%T' cannot be parsed as csv", value)
		}

		reader := csvparser.NewReader(strings.NewReader(csvLine))
		reader.Comma = fieldDelimiter
		reader.FieldsPerRecord = len(headers)
		reader.LazyQuotes = lazyQuotes
		parsedValues := make(map[string]interface{})

		for {
			body, err := reader.Read()
			if err == io.EOF {
				break
			}

			if err != nil {
				return nil, err
			}

			for i, key := range headers {
				parsedValues[key] = body[i]
			}
		}

		return parsedValues, nil
	}
}
