// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package keyvalue // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/keyvalue"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "key_value_parser"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new key value parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new key value parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType),
		Delimiter:    "=",
	}
}

// Config is the configuration of a key value parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`

	Delimiter     string `mapstructure:"delimiter"`
	PairDelimiter string `mapstructure:"pair_delimiter"`
}

// Build will build a key value parser operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.Delimiter == "" {
		return nil, errors.New("delimiter is a required parameter")
	}

	pairDelimiter := " "
	if c.PairDelimiter != "" {
		pairDelimiter = c.PairDelimiter
	}

	if c.Delimiter == pairDelimiter {
		return nil, errors.New("delimiter and pair_delimiter cannot be the same value")
	}

	return &Parser{
		ParserOperator: parserOperator,
		delimiter:      c.Delimiter,
		pairDelimiter:  pairDelimiter,
	}, nil
}

// Parser is an operator that parses key value pairs.
type Parser struct {
	helper.ParserOperator
	delimiter     string
	pairDelimiter string
}

// Process will parse an entry for key value pairs.
func (kv *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return kv.ParserOperator.ProcessWith(ctx, entry, kv.parse)
}

// parse will parse a value as key values.
func (kv *Parser) parse(value any) (any, error) {
	switch m := value.(type) {
	case string:
		return kv.parser(m, kv.delimiter, kv.pairDelimiter)
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as key value pairs", value)
	}
}

func (kv *Parser) parser(input string, delimiter string, pairDelimiter string) (map[string]any, error) {
	if input == "" {
		return nil, fmt.Errorf("parse from field %s is empty", kv.ParseFrom.String())
	}

	pairs, err := splitPairs(input, pairDelimiter)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pairs from input: %w", err)
	}

	parsed := make(map[string]any)

	for _, raw := range pairs {
		m := strings.SplitN(raw, delimiter, 2)
		if len(m) != 2 {
			e := fmt.Errorf("expected '%s' to split by '%s' into two items, got %d", raw, delimiter, len(m))
			err = multierr.Append(err, e)
			continue
		}

		key := strings.TrimSpace(m[0])
		value := strings.TrimSpace(m[1])

		parsed[key] = value
	}

	return parsed, err
}

// splitPairs will split the input on the pairDelimiter and return the resulting slice.
// `strings.Split` is not used because it does not respect quotes and will split if the delimiter appears in a quoted value
func splitPairs(input, pairDelimiter string) ([]string, error) {
	var result []string
	currentPair := ""
	delimiterLength := len(pairDelimiter)
	quoteChar := "" // "" means we are not in quotes

	for i := 0; i < len(input); i++ {
		if quoteChar == "" && i+delimiterLength <= len(input) && input[i:i+delimiterLength] == pairDelimiter { // delimiter
			if currentPair == "" { // leading || trailing delimiter; ignore
				continue
			}
			result = append(result, currentPair)
			currentPair = ""
			i += delimiterLength - 1
			continue
		}

		if quoteChar == "" && (input[i] == '"' || input[i] == '\'') { // start of quote
			quoteChar = string(input[i])
			continue
		}
		if string(input[i]) == quoteChar { // end of quote
			quoteChar = ""
			continue
		}

		currentPair += string(input[i])
	}

	if quoteChar != "" { // check for closed quotes
		return nil, fmt.Errorf("never reached end of a quoted value")
	}
	if currentPair != "" { // avoid adding empty value bc of a trailing delimiter
		return append(result, currentPair), nil
	}

	return result, nil
}
