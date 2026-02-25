// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonparser // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/jsonparser"

import (
	"context"
	"fmt"
	"strings"

	"github.com/goccy/go-json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses JSON.
type Parser struct {
	helper.ParserOperator

	parseInts bool
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.parse)
}

// Process will parse an entry for JSON.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.parse)
}

// parse will parse a value as JSON.
func (p *Parser) parse(value any) (any, error) {
	var parsedValue map[string]any
	switch m := value.(type) {
	case string:
		// when parseInts is disabled, `int` and `float` will be parsed as `float64`.
		// when it is enabled, they will be parsed as `json.Number`, later the parser
		// will convert them to `int` or `float64` according to the field type.
		if p.parseInts {
			d := json.NewDecoder(strings.NewReader(m))
			d.UseNumber()
			err := d.Decode(&parsedValue)
			if err != nil {
				return nil, err
			}
			convertNumbers(parsedValue)
		} else {
			err := json.Unmarshal([]byte(m), &parsedValue)
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as JSON", value)
	}

	return parsedValue, nil
}

func convertNumbers(parsedValue map[string]any) {
	for k, v := range parsedValue {
		switch t := v.(type) {
		case json.Number:
			parsedValue[k] = convertNumber(t)
		case map[string]any:
			convertNumbers(t)
		case []any:
			convertNumbersArray(t)
		}
	}
}

func convertNumbersArray(arr []any) {
	for i, v := range arr {
		switch t := v.(type) {
		case json.Number:
			arr[i] = convertNumber(t)
		case map[string]any:
			convertNumbers(t)
		case []any:
			convertNumbersArray(t)
		}
	}
}

func convertNumber(value json.Number) any {
	i64, err := value.Int64()
	if err == nil {
		return i64
	}
	f64, err := value.Float64()
	if err == nil {
		return f64
	}
	return value.String()
}
