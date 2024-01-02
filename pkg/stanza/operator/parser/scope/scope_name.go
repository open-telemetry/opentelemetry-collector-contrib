// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scope // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/scope"

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "scope_name_parser"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new logger name parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new logger name parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
		ScopeNameParser:   helper.NewScopeNameParser(),
	}
}

// Config is the configuration of a logger name parser operator.
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	helper.ScopeNameParser   `mapstructure:",omitempty,squash"`
}

// Build will build a logger name parser operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &Parser{
		TransformerOperator: transformerOperator,
		ScopeNameParser:     c.ScopeNameParser,
	}, nil
}

// Parser is an operator that parses logger name from a field to an entry.
type Parser struct {
	helper.TransformerOperator
	helper.ScopeNameParser
}

// Process will parse logger name from an entry.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Parse)
}
