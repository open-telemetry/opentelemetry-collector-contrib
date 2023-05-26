// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drop // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/drop"

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func init() {
	operator.Register("drop_output", func() operator.Builder { return NewConfig("") })
}

// NewConfig creates a new drop output config with default values
func NewConfig(operatorID string) *Config {
	return &Config{
		OutputConfig: helper.NewOutputConfig(operatorID, "drop_output"),
	}
}

// Config is the configuration of a drop output operator.
type Config struct {
	helper.OutputConfig `mapstructure:",squash"`
}

// Build will build a drop output operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	outputOperator, err := c.OutputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &Output{
		OutputOperator: outputOperator,
	}, nil
}

// Output is an operator that consumes and ignores incoming entries.
type Output struct {
	helper.OutputOperator
}

// Process will drop the incoming entry.
func (p *Output) Process(ctx context.Context, entry *entry.Entry) error {
	return nil
}
