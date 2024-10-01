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
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(set)
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
