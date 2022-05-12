// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package keyvalue // import "github.com/open-telemetry/opentelemetry-log-collection/operator/parser/keyvalue"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("key_value_parser", func() operator.Builder { return NewKVParserConfig("") })
}

// NewKVParserConfig creates a new key value parser config with default values
func NewKVParserConfig(operatorID string) *KVParserConfig {
	return &KVParserConfig{
		ParserConfig: helper.NewParserConfig(operatorID, "key_value_parser"),
		Delimiter:    "=",
	}
}

// KVParserConfig is the configuration of a key value parser operator.
type KVParserConfig struct {
	helper.ParserConfig `mapstructure:",squash" yaml:",inline"`

	Delimiter     string `mapstructure:"delimiter" yaml:"delimiter"`
	PairDelimiter string `mapstructure:"pair_delimiter" yaml:"pair_delimiter"`
}

// Build will build a key value parser operator.
func (c KVParserConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.Delimiter == c.PairDelimiter {
		return nil, errors.New("delimiter and pair_delimiter cannot be the same value")
	}

	if c.Delimiter == "" {
		return nil, errors.New("delimiter is a required parameter")
	}

	// split on whitespace by default, if pair delimiter is set, use
	// strings.Split()
	pairSplitFunc := splitStringByWhitespace
	if c.PairDelimiter != "" {
		pairSplitFunc = func(input string) []string {
			return strings.Split(input, c.PairDelimiter)
		}
	}

	return &KVParser{
		ParserOperator: parserOperator,
		delimiter:      c.Delimiter,
		pairSplitFunc:  pairSplitFunc,
	}, nil
}

// KVParser is an operator that parses key value pairs.
type KVParser struct {
	helper.ParserOperator
	delimiter     string
	pairSplitFunc func(input string) []string
}

// Process will parse an entry for key value pairs.
func (kv *KVParser) Process(ctx context.Context, entry *entry.Entry) error {
	return kv.ParserOperator.ProcessWith(ctx, entry, kv.parse)
}

// parse will parse a value as key values.
func (kv *KVParser) parse(value interface{}) (interface{}, error) {
	switch m := value.(type) {
	case string:
		return kv.parser(m, kv.delimiter)
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as key value pairs", value)
	}
}

func (kv *KVParser) parser(input string, delimiter string) (map[string]interface{}, error) {
	if input == "" {
		return nil, fmt.Errorf("parse from field %s is empty", kv.ParseFrom.String())
	}

	parsed := make(map[string]interface{})

	var err error
	for _, raw := range kv.pairSplitFunc(input) {
		m := strings.Split(raw, delimiter)
		if len(m) != 2 {
			e := fmt.Errorf("expected '%s' to split by '%s' into two items, got %d", raw, delimiter, len(m))
			err = multierror.Append(err, e)
			continue
		}

		key := strings.TrimSpace(strings.Trim(m[0], "\"'"))
		value := strings.TrimSpace(strings.Trim(m[1], "\"'"))

		parsed[key] = value
	}

	return parsed, err
}

// split on whitespace and preserve quoted text
func splitStringByWhitespace(input string) []string {
	quoted := false
	raw := strings.FieldsFunc(input, func(r rune) bool {
		if r == '"' || r == '\'' {
			quoted = !quoted
		}
		return !quoted && r == ' '
	})
	return raw
}
