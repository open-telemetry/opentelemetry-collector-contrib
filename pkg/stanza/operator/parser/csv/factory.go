// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package csv // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/csv"

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("csv_parser")

func init() {
	operator.RegisterFactory(NewFactory())
}

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return operator.NewFactory(operatorType, newDefaultConfig, createOperator)
}

func newDefaultConfig(operatorID string) component.Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType.String()),
	}
}

func createOperator(set component.TelemetrySettings, cfg component.Config) (operator.Operator, error) {
	c := cfg.(*Config)
	parserOperator, err := helper.NewParser(set, c.ParserConfig)
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
