// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// NewWriterConfig creates a new writer config
func NewWriterConfig(operatorID, operatorType string) WriterConfig {
	return WriterConfig{
		BasicConfig: NewBasicConfig(operatorID, operatorType),
	}
}

// WriterConfig is the configuration of a writer operator.
type WriterConfig struct {
	BasicConfig `mapstructure:",squash"`
	OutputIDs   []string `mapstructure:"output"`
}

// Build will build a writer operator from the config.
func (c WriterConfig) Build(set component.TelemetrySettings) (WriterOperator, error) {
	basicOperator, err := c.BasicConfig.Build(set)
	if err != nil {
		return WriterOperator{}, err
	}

	return WriterOperator{
		OutputIDs:     c.OutputIDs,
		BasicOperator: basicOperator,
	}, nil
}

// WriterOperator is an operator that can write to other operators.
type WriterOperator struct {
	BasicOperator
	OutputIDs       []string
	OutputOperators []operator.Operator
}

// Write will write an entry to the outputs of the operator.
func (w *WriterOperator) Write(ctx context.Context, e *entry.Entry) error {
	for i, op := range w.OutputOperators {
		if i == len(w.OutputOperators)-1 {
			return op.Process(ctx, e)
		}
		err := op.Process(ctx, e.Copy())
		if err != nil {
			w.Logger().Error("Failed to process entry", zap.Error(err))
		}
	}
	return nil
}

// CanOutput always returns true for a writer operator.
func (w *WriterOperator) CanOutput() bool {
	return true
}

// Outputs returns the outputs of the writer operator.
func (w *WriterOperator) Outputs() []operator.Operator {
	return w.OutputOperators
}

// GetOutputIDs returns the output IDs of the writer operator.
func (w *WriterOperator) GetOutputIDs() []string {
	return w.OutputIDs
}

// SetOutputs will set the outputs of the operator.
func (w *WriterOperator) SetOutputs(operators []operator.Operator) error {
	outputOperators := make([]operator.Operator, len(w.OutputIDs))

	for i, operatorID := range w.OutputIDs {
		operator, ok := w.findOperator(operators, operatorID)
		if !ok {
			return fmt.Errorf("operator '%s' does not exist", operatorID)
		}

		if !operator.CanProcess() {
			return fmt.Errorf("operator '%s' can not process entries", operatorID)
		}

		outputOperators[i] = operator
	}

	w.OutputOperators = outputOperators
	return nil
}

// SetOutputIDs will set the outputs of the operator.
func (w *WriterOperator) SetOutputIDs(opIDs []string) {
	w.OutputIDs = opIDs
}

// FindOperator will find an operator matching the supplied id.
func (w *WriterOperator) findOperator(operators []operator.Operator, operatorID string) (operator.Operator, bool) {
	for _, operator := range operators {
		if operator.ID() == operatorID {
			return operator, true
		}
	}
	return nil, false
}
