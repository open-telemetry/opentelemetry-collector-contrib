// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package time // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/time"

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "time_parser"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new time parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new time parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
		TimeParser:        helper.NewTimeParser(),
	}
}

// Config is the configuration of a time parser operator.
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	helper.TimeParser        `mapstructure:",omitempty,squash"`
}

// Build will build a time parser operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if err := c.TimeParser.Validate(); err != nil {
		return nil, err
	}

	return &Parser{
		TransformerOperator: transformerOperator,
		TimeParser:          c.TimeParser,
	}, nil
}

// Parser is an operator that parses time from a field to an entry.
type Parser struct {
	helper.TransformerOperator
	helper.TimeParser
}

// Process will parse time from an entry.
func (t *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.TimeParser.Parse)
}
