// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regex // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"

import (
	"context"
	"fmt"
	"regexp"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "regex_parser"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new regex parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new regex parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a regex parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`

	Regex string `mapstructure:"regex"`

	Cache struct {
		Size uint16 `mapstructure:"size"`
	} `mapstructure:"cache"`
}

// Build will build a regex parser operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.Regex == "" {
		return nil, fmt.Errorf("missing required field 'regex'")
	}

	r, err := regexp.Compile(c.Regex)
	if err != nil {
		return nil, fmt.Errorf("compiling regex: %w", err)
	}

	namedCaptureGroups := 0
	for _, groupName := range r.SubexpNames() {
		if groupName != "" {
			namedCaptureGroups++
		}
	}
	if namedCaptureGroups == 0 {
		return nil, errors.NewError(
			"no named capture groups in regex pattern",
			"use named capture groups like '^(?P<my_key>.*)$' to specify the key name for the parsed field",
		)
	}

	op := &Parser{
		ParserOperator: parserOperator,
		regexp:         r,
	}

	if c.Cache.Size > 0 {
		op.cache = newMemoryCache(c.Cache.Size, 0)
		logger.Debugf("configured %s with memory cache of size %d", op.ID(), op.cache.maxSize())
	}

	return op, nil
}

// Parser is an operator that parses regex in an entry.
type Parser struct {
	helper.ParserOperator
	regexp *regexp.Regexp
	cache  cache
}

// Process will parse an entry for regex.
func (r *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return r.ParserOperator.ProcessWith(ctx, entry, r.parse)
}

// parse will parse a value using the supplied regex.
func (r *Parser) parse(value any) (any, error) {
	var raw string
	switch m := value.(type) {
	case string:
		raw = m
	default:
		return nil, fmt.Errorf("type '%T' cannot be parsed as regex", value)
	}
	return r.match(raw)
}

func (r *Parser) match(value string) (any, error) {
	if r.cache != nil {
		if x := r.cache.get(value); x != nil {
			return x, nil
		}
	}

	matches := r.regexp.FindStringSubmatch(value)
	if matches == nil {
		return nil, fmt.Errorf("regex pattern does not match")
	}

	parsedValues := map[string]any{}
	for i, subexp := range r.regexp.SubexpNames() {
		if i == 0 {
			// Skip whole match
			continue
		}
		if subexp != "" {
			parsedValues[subexp] = matches[i]
		}
	}

	if r.cache != nil {
		r.cache.add(value, parsedValues)
	}

	return parsedValues, nil
}
