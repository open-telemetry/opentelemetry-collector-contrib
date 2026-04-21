// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/filter"

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "filter"

var (
	upperBound = big.NewInt(1000)
	randInt    = rand.Int // allow override for testing
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a filter operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a filter operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
		DropRatio:         1,
	}
}

// Config is the configuration of a filter operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	Expression               string  `mapstructure:"expr"`
	DropRatio                float64 `mapstructure:"drop_ratio"`
}

// Build will build a filter operator from the supplied configuration
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformer, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	compiledExpression, err := helper.ExprCompileBool(c.Expression)
	if err != nil {
		return nil, fmt.Errorf("failed to compile expression '%s': %w", c.Expression, err)
	}

	if c.DropRatio < 0.0 || c.DropRatio > 1.0 {
		return nil, errors.New("drop_ratio must be a number between 0 and 1")
	}

	return &Transformer{
		TransformerOperator: transformer,
		expression:          compiledExpression,
		dropCutoff:          big.NewInt(int64(c.DropRatio * 1000)),
	}, nil
}
