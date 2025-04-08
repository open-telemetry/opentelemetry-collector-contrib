// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package jsonarray // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/jsonarray"

import (
	"context"
	"errors"
	"fmt"

	"github.com/valyala/fastjson"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses json array in an entry.
type Parser struct {
	helper.ParserOperator
	parse parseFunc
}

type parseFunc func(any) (any, error)

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.Process)
}

// Process will parse an entry for json array.
func (p *Parser) Process(ctx context.Context, e *entry.Entry) error {
	return p.ProcessWith(ctx, e, p.parse)
}

func generateParseToArrayFunc(pool *fastjson.ParserPool) parseFunc {
	return func(value any) (any, error) {
		jArrayLine, err := valueAsString(value)
		if err != nil {
			return nil, err
		}

		p := pool.Get()
		v, err := p.Parse(jArrayLine)
		pool.Put(p)
		if err != nil {
			return nil, errors.New("failed to parse entry")
		}

		jArray := v.GetArray() // a is a []*Value slice
		parsedValues := make([]any, len(jArray))
		for i := range jArray {
			switch jArray[i].Type() {
			case fastjson.TypeNumber:
				parsedValues[i] = jArray[i].GetInt64()
			case fastjson.TypeString:
				parsedValues[i] = string(jArray[i].GetStringBytes())
			case fastjson.TypeTrue:
				parsedValues[i] = true
			case fastjson.TypeFalse:
				parsedValues[i] = false
			case fastjson.TypeNull:
				parsedValues[i] = nil
			case fastjson.TypeObject:
				// Nested objects handled as a string since this parser doesn't support nested headers
				parsedValues[i] = jArray[i].String()
			default:
				return nil, errors.New("failed to parse entry")
			}
		}

		return parsedValues, nil
	}
}

func generateParseToMapFunc(pool *fastjson.ParserPool, header []string) parseFunc {
	return func(value any) (any, error) {
		jArrayLine, err := valueAsString(value)
		if err != nil {
			return nil, err
		}

		p := pool.Get()
		v, err := p.Parse(jArrayLine)
		pool.Put(p)
		if err != nil {
			return nil, errors.New("failed to parse entry")
		}

		jArray := v.GetArray() // a is a []*Value slice
		if len(header) != len(jArray) {
			return nil, fmt.Errorf("wrong number of fields: expected %d, found %d", len(header), len(jArray))
		}
		parsedValues := make(map[string]any, len(jArray))
		for i := range jArray {
			switch jArray[i].Type() {
			case fastjson.TypeNumber:
				parsedValues[header[i]] = jArray[i].GetInt64()
			case fastjson.TypeString:
				parsedValues[header[i]] = string(jArray[i].GetStringBytes())
			case fastjson.TypeTrue:
				parsedValues[header[i]] = true
			case fastjson.TypeFalse:
				parsedValues[header[i]] = false
			case fastjson.TypeNull:
				parsedValues[header[i]] = nil
			case fastjson.TypeObject:
				// Nested objects handled as a string since this parser doesn't support nested headers
				parsedValues[header[i]] = jArray[i].String()
			default:
				return nil, errors.New("failed to parse entry")
			}
		}

		return parsedValues, nil
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
		return s, fmt.Errorf("type '%T' cannot be parsed as json array", value)
	}

	return s, nil
}
