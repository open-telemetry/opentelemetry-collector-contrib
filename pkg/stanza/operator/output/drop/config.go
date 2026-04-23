// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drop // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/drop"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "drop_output"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig("") })
}

// NewConfig creates a new drop output config with default values
func NewConfig(operatorID string) *Config {
	return &Config{
		OutputConfig: helper.NewOutputConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a drop output operator.
type Config struct {
	helper.OutputConfig `mapstructure:",squash"`
}

// Build will build a drop output operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	outputOperator, err := c.OutputConfig.Build(set)
	if err != nil {
		return nil, err
	}

	return &Output{
		OutputOperator: outputOperator,
	}, nil
}
