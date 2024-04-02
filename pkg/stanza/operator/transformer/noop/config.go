// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package noop // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/noop"

import (
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "noop"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new noop operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new noop operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a noop operator.
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
}

// Build will build a noop operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &Transformer{
		TransformerOperator: transformerOperator,
	}, nil
}
