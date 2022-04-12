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

func init() {
	operator.Register("csv_parser", func() operator.Builder { return NewParserConfig("") })
}

// NewParserConfig creates a new csv parser config with default values
func NewParserConfig(operatorID string) *ParserConfig {
	return &ParserConfig{
		ParserConfig: helper.NewParserConfig(operatorID, "csv_parser"),
	}
}

// ParserConfig is the configuration of a csv parser operator.
type ParserConfig struct {
	helper.ParserConfig `yaml:",inline"`

	Header          string `json:"header" yaml:"header"`
	HeaderAttribute string `json:"header_attribute" yaml:"header_attribute"`
	FieldDelimiter  string `json:"delimiter,omitempty" yaml:"delimiter,omitempty"`
	LazyQuotes      bool   `json:"lazy_quotes,omitempty" yaml:"lazy_quotes,omitempty"`
}

// Build will build a csv parser operator.
func (c ParserConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
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

	return &Parser{
		ParserOperator:  parserOperator,
		header:          headers,
		headerAttribute: c.HeaderAttribute,
		fieldDelimiter:  fieldDelimiter,
		lazyQuotes:      c.LazyQuotes,

		parse: generateParseFunc(headers, fieldDelimiter, c.LazyQuotes),
	}, nil
}

// Parser is an operator that parses csv in an entry.
type Parser struct {
	helper.ParserOperator
	fieldDelimiter  rune
	header          []string
	headerAttribute string
	lazyQuotes      bool
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

		// Typically only need one
		lines := make([][]string, 0, 1)
		for {
			line, err := reader.Read()
			if err == io.EOF {
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

		parsedValues := make(map[string]interface{})

		if len(joinedLine) != len(headers) {
			return nil, errors.New("wrong number of fields")
		}

		for i, val := range joinedLine {
			parsedValues[headers[i]] = val
		}
		return parsedValues, nil
	}
}
