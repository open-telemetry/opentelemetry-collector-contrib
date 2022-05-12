// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper // import "github.com/open-telemetry/opentelemetry-log-collection/operator/helper"

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
)

// NewWriterConfig creates a new writer config
func NewWriterConfig(operatorID, operatorType string) WriterConfig {
	return WriterConfig{
		BasicConfig: NewBasicConfig(operatorID, operatorType),
	}
}

// WriterConfig is the configuration of a writer operator.
type WriterConfig struct {
	BasicConfig `mapstructure:",squash" yaml:",inline"`
	OutputIDs   OutputIDs `mapstructure:"output" json:"output" yaml:"output"`
}

// Build will build a writer operator from the config.
func (c WriterConfig) Build(logger *zap.SugaredLogger) (WriterOperator, error) {
	basicOperator, err := c.BasicConfig.Build(logger)
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
	OutputIDs       OutputIDs
	OutputOperators []operator.Operator
}

// Write will write an entry to the outputs of the operator.
func (w *WriterOperator) Write(ctx context.Context, e *entry.Entry) {
	for i, operator := range w.OutputOperators {
		if i == len(w.OutputOperators)-1 {
			_ = operator.Process(ctx, e)
			return
		}
		_ = operator.Process(ctx, e.Copy())
	}
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
	outputOperators := make([]operator.Operator, 0)

	for _, operatorID := range w.OutputIDs {
		operator, ok := w.findOperator(operators, operatorID)
		if !ok {
			return fmt.Errorf("operator '%s' does not exist", operatorID)
		}

		if !operator.CanProcess() {
			return fmt.Errorf("operator '%s' can not process entries", operatorID)
		}

		outputOperators = append(outputOperators, operator)
	}

	w.OutputOperators = outputOperators
	return nil
}

// SetOutputIDs will set the outputs of the operator.
func (w *WriterOperator) SetOutputIDs(opIds []string) {
	w.OutputIDs = opIds
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

// OutputIDs is a collection of operator IDs used as outputs.
type OutputIDs []string

// UnmarshalJSON will unmarshal a string or array of strings to OutputIDs.
func (o *OutputIDs) UnmarshalJSON(bytes []byte) error {
	var value interface{}
	err := json.Unmarshal(bytes, &value)
	if err != nil {
		return err
	}

	ids, err := NewOutputIDsFromInterface(value)
	if err != nil {
		return err
	}

	*o = ids
	return nil
}

// UnmarshalYAML will unmarshal a string or array of strings to OutputIDs.
func (o *OutputIDs) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value interface{}
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	ids, err := NewOutputIDsFromInterface(value)
	if err != nil {
		return err
	}

	*o = ids
	return nil
}

// NewOutputIDsFromInterface creates a new OutputIDs object from an interface
func NewOutputIDsFromInterface(value interface{}) (OutputIDs, error) {
	if str, ok := value.(string); ok {
		return OutputIDs{str}, nil
	}

	if array, ok := value.([]interface{}); ok {
		return NewOutputIDsFromArray(array)
	}

	return nil, fmt.Errorf("value is not of type string or string array")
}

// NewOutputIDsFromArray creates a new OutputIDs object from an array
func NewOutputIDsFromArray(array []interface{}) (OutputIDs, error) {
	ids := OutputIDs{}
	for _, rawValue := range array {
		strValue, ok := rawValue.(string)
		if !ok {
			return nil, fmt.Errorf("value in array is not of type string")
		}
		ids = append(ids, strValue)
	}
	return ids, nil
}
