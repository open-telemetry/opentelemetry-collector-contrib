// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package json // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/json"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/goccy/go-json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses JSON.
type Parser struct {
	helper.ParserOperator

	useNumber bool
}

// Process will parse an entry for JSON.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ParserOperator.ProcessWith(ctx, entry, p.parse)
}

// parse will parse a value as JSON.
func (p *Parser) parse(value any) (any, error) {
	var parsedValue map[string]any
	switch m := value.(type) {
	case string:
		err := json.Unmarshal([]byte(m), &parsedValue)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as JSON", value)
	}

	if p.useNumber {
		p.convertNumbers(parsedValue)
	}
	return parsedValue, nil
}

func (p *Parser) convertNumbers(parsedValue map[string]any) {
	for k, v := range parsedValue {
		switch t := v.(type) {
		case json.Number:
			parsedValue[k] = p.convertNumber(t)
		case map[string]any:
			p.convertNumbers(t)
		case []any:
			p.convertNumbersArray(t)
		}
	}
}

func (p *Parser) convertNumbersArray(arr []any) {
	for i, v := range arr {
		switch t := v.(type) {
		case json.Number:
			arr[i] = p.convertNumber(t)
		case map[string]any:
			p.convertNumbers(t)
		case []any:
			p.convertNumbersArray(t)
		}
	}
}

func (p *Parser) convertNumber(value json.Number) any {
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
