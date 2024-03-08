// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// NewOutputConfig creates a new output config
func NewOutputConfig(operatorID, operatorType string) OutputConfig {
	return OutputConfig{
		BasicConfig: NewBasicConfig(operatorID, operatorType),
	}
}

// OutputConfig provides a basic implementation of an output operator config.
type OutputConfig struct {
	BasicConfig `mapstructure:",squash"`
}

// Deprecated [v0.97.0] Use NewOutputOperator instead.
func (c OutputConfig) Build(logger *zap.SugaredLogger) (OutputOperator, error) {
	return NewOutputOperator(c, component.TelemetrySettings{Logger: logger.Desugar()})
}

// NewOutputOperator creates a new output operator.
func NewOutputOperator(c OutputConfig, set component.TelemetrySettings) (OutputOperator, error) {
	basicOperator, err := NewBasicOperator(c.BasicConfig, set)
	if err != nil {
		return OutputOperator{}, err
	}
	return OutputOperator{BasicOperator: basicOperator}, nil
}

// OutputOperator provides a basic implementation of an output operator.
type OutputOperator struct {
	BasicOperator
}

// CanProcess will always return true for an output operator.
func (o *OutputOperator) CanProcess() bool {
	return true
}

// CanOutput will always return false for an output operator.
func (o *OutputOperator) CanOutput() bool {
	return false
}

// Outputs will always return an empty array for an output operator.
func (o *OutputOperator) Outputs() []operator.Operator {
	return []operator.Operator{}
}

// GetOutputIDs will always return an empty array for an output ID.
func (o *OutputOperator) GetOutputIDs() []string {
	return []string{}
}

// SetOutputs will return an error if called.
func (o *OutputOperator) SetOutputs(_ []operator.Operator) error {
	return errors.NewError(
		"Operator can not output, but is attempting to set an output.",
		"This is an unexpected internal error. Please submit a bug/issue.",
	)
}

// SetOutputIDs will return nothing and does nothing.
func (o *OutputOperator) SetOutputIDs(_ []string) {
}
