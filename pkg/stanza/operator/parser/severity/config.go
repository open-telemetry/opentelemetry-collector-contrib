// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package severity // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/severity"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "severity_parser"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new severity parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new severity parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
		SeverityConfig:    helper.NewSeverityConfig(),
	}
}

// Config is the configuration of a severity parser operator.
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	helper.SeverityConfig    `mapstructure:",omitempty,squash"`
}

// Build will build a severity parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	severityParser, err := c.SeverityConfig.Build(set)
	if err != nil {
		return nil, err
	}

	return &Parser{
		TransformerOperator: transformerOperator,
		SeverityParser:      severityParser,
	}, nil
}
