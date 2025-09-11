// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regexreplace // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/regexreplace"

import (
	"errors"
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "regex_replace"

// derived from https://en.wikipedia.org/wiki/ANSI_escape_code#CSIsection
var ansiCsiEscapeRegex = regexp.MustCompile(`\x1B\[[\x30-\x3F]*[\x20-\x2F]*[\x40-\x7E]`)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new ansi_control_sequences config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new ansi_control_sequences config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of an ansi_control_sequences operator.
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	RegexName                string      `mapstructure:"regex_name"`
	Regex                    string      `mapstructure:"regex"`
	ReplaceWith              string      `mapstructure:"replace_with"`
	Field                    entry.Field `mapstructure:"field"`
}

func (c *Config) getRegexp() (*regexp.Regexp, error) {
	switch c.RegexName {
	case "ansi_control_sequences":
		return ansiCsiEscapeRegex, nil
	case "":
		return regexp.Compile(c.Regex)
	default:
		return nil, fmt.Errorf("regex_name %s is unknown", c.RegexName)
	}
}

// Build will build an ansi_control_sequences operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	if (c.RegexName == "") == (c.Regex == "") {
		return nil, errors.New("either regex or regex_name must be set")
	}

	regexp, err := c.getRegexp()
	if err != nil {
		return nil, err
	}

	return &Transformer{
		TransformerOperator: transformerOperator,
		field:               c.Field,
		regexp:              regexp,
		replaceWith:         c.ReplaceWith,
	}, nil
}
