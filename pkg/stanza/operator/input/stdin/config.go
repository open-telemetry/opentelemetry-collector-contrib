// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stdin // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/stdin"

import (
	"os"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "stdin"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig("") })
}

// NewConfig creates a new stdin input config with default values
func NewConfig(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a stdin input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
}

// Build will build a stdin input operator.
func (c *Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &Input{
		InputOperator: inputOperator,
		stdin:         os.Stdin,
	}, nil
}
