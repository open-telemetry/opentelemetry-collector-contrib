// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drop // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/drop"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("drop_output")

func init() {
	operator.RegisterFactory(NewFactory())
}

// Deprecated [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfig(operatorID string) *Config {
	return NewFactory().NewDefaultConfig(operatorID).(*Config)
}

// Config is the configuration of a drop output operator.
type Config struct {
	helper.OutputConfig `mapstructure:",squash"`
}

// Deprecated [v0.97.0] Use NewFactory.CreateOperator instead.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	return NewFactory().CreateOperator(&c, component.TelemetrySettings{Logger: logger.Desugar()})
}

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return operator.NewFactory(operatorType, newDefaultConfig, createOperator)
}

func newDefaultConfig(operatorID string) component.Config {
	return &Config{
		OutputConfig: helper.NewOutputConfig(operatorID, operatorType.String()),
	}
}

func createOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	outputOperator, err := helper.NewOutputOperator(c.OutputConfig, set)
	if err != nil {
		return nil, err
	}
	return &Output{OutputOperator: outputOperator}, nil
}

// Output is an operator that consumes and ignores incoming entries.
type Output struct {
	helper.OutputOperator
}

// Process will drop the incoming entry.
func (p *Output) Process(_ context.Context, _ *entry.Entry) error {
	return nil
}
