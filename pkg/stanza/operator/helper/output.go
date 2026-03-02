// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/stanzaerrors"
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
	// prevent unkeyed literal initialization
	_ struct{}
}

// Build will build an output operator.
func (c OutputConfig) Build(set component.TelemetrySettings) (OutputOperator, error) {
	basicOperator, err := c.BasicConfig.Build(set)
	if err != nil {
		return OutputOperator{}, err
	}

	outputOperator := OutputOperator{
		BasicOperator: basicOperator,
	}

	return outputOperator, nil
}

// OutputOperator provides a basic implementation of an output operator.
type OutputOperator struct {
	BasicOperator
	// prevent unkeyed literal initialization
	_ struct{}
}

// CanProcess will always return true for an output operator.
func (*OutputOperator) CanProcess() bool {
	return true
}

// CanOutput will always return false for an output operator.
func (*OutputOperator) CanOutput() bool {
	return false
}

// Outputs will always return an empty array for an output operator.
func (*OutputOperator) Outputs() []operator.Operator {
	return []operator.Operator{}
}

// GetOutputIDs will always return an empty array for an output ID.
func (*OutputOperator) GetOutputIDs() []string {
	return []string{}
}

// SetOutputs will return an error if called.
func (*OutputOperator) SetOutputs(_ []operator.Operator) error {
	return stanzaerrors.NewError(
		"Operator can not output, but is attempting to set an output.",
		"This is an unexpected internal error. Please submit a bug/issue.",
	)
}

// SetOutputIDs will return nothing and does nothing.
func (*OutputOperator) SetOutputIDs([]string) {
}
