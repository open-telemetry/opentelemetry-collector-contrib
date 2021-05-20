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
	"fmt"
	"strings"

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

	Header         string `json:"header" yaml:"header"`
	FieldDelimiter string `json:"delimiter,omitempty" yaml:"delimiter,omitempty"`
}

// Build will build a csv parser operator.
func (c CSVParserConfig) Build(context operator.BuildContext) ([]operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(context)
	if err != nil {
		return nil, err
	}

	if c.Header == "" {
		return nil, fmt.Errorf("Missing required field 'header'")
	}

	if c.FieldDelimiter == "" {
		c.FieldDelimiter = ","
	}

	if len([]rune(c.FieldDelimiter)) != 1 {
		return nil, fmt.Errorf("Invalid 'delimiter': '%s'", c.FieldDelimiter)
	}

	fieldDelimiter := []rune(c.FieldDelimiter)[0]

	if !strings.Contains(c.Header, c.FieldDelimiter) {
		return nil, fmt.Errorf("missing field delimiter in header")
	}

	numFields := len(strings.Split(c.Header, c.FieldDelimiter))

	delimiterStr := string([]rune{fieldDelimiter})
	csvParser := &CSVParser{
		ParserOperator: parserOperator,
		header:         strings.Split(c.Header, delimiterStr),
		fieldDelimiter: fieldDelimiter,
		numFields:      numFields,
	}

	return []operator.Operator{csvParser}, nil
}

// CSVParser is an operator that parses csv in an entry.
type CSVParser struct {
	helper.ParserOperator
	header         []string
	fieldDelimiter rune
	numFields      int
}

// Process will parse an entry for csv.
func (r *CSVParser) Process(ctx context.Context, entry *entry.Entry) error {
	return r.ParserOperator.ProcessWith(ctx, entry, r.parse)
}

// parse will parse a value using the supplied csv header.
func (r *CSVParser) parse(value interface{}) (interface{}, error) {
	var csvLine string
	switch val := value.(type) {
	case string:
		csvLine = val
	default:
		return nil, fmt.Errorf("type '%T' cannot be parsed as csv", value)
	}

	reader := csvparser.NewReader(strings.NewReader(csvLine))
	reader.Comma = r.fieldDelimiter
	reader.FieldsPerRecord = r.numFields
	parsedValues := make(map[string]interface{})

	record, err := reader.Read()

	if err != nil {
		return nil, err
	}

	for i, key := range r.header {
		parsedValues[key] = record[i]
	}

	return parsedValues, nil
}
