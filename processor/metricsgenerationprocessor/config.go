// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricsgenerationprocessor

import (
	"fmt"
	"sort"

	"go.opentelemetry.io/collector/config"
)

const (
	// NewMetricFieldName is the mapstructure field name for NewMetricName field
	NewMetricFieldName = "new_metric_name"

	// GenerationTypeFieldName is the mapstructure field name for Type field
	GenerationTypeFieldName = "generation_type"

	// Operand1MetricFieldName is the mapstructure field name for Operand1Metric field
	Operand1MetricFieldName = "operand1_metric"

	// Operand2MetricFieldName is the mapstructure field name for Operand2Metric field
	Operand2MetricFieldName = "operand2_metric"

	// ScaleByFieldName is the mapstructure field name for ScaleBy field
	ScaleByFieldName = "scale_by"

	// OperationFieldName is the mapstructure field name for Operation field
	OperationFieldName = "operation"
)

// Config defines the configuration for the processor.
type Config struct {
	*config.ProcessorSettings `mapstructure:"-"`

	// Set of rules for generating new metrics
	Rules []Rule `mapstructure:"rules"`
}

type Rule struct {
	// Name of the new metric being generated. This is a required field.
	NewMetricName string `mapstructure:"new_metric_name"`

	// The rule type following which the new metric will be generated. This is a required field.
	Type GenerationType `mapstructure:"generation_type"`

	// First operand metric to use in the calculation. This is a required field.
	Operand1Metric string `mapstructure:"operand1_metric"`

	// Second operand metric to use in the calculation. A required field if the generation_type is calculate.
	Operand2Metric string `mapstructure:"operand2_metric"`

	// The arithmetic operation to apply for the calculation. This is a required field.
	Operation OperationType `mapstructure:"operation"`

	// A constant number by which the first operand will be scaled. A required field if the generation_type is scale.
	ScaleBy float64 `mapstructure:"scale_by"`
}

type GenerationType string

const (

	// Generates a new metric applying an arithmatic operation with two operands
	Calculate GenerationType = "calculate"

	// Generates a new metric scaling the value of s given metric with a provided constant
	Scale GenerationType = "scale"
)

var generationTypes = map[GenerationType]struct{}{Calculate: {}, Scale: {}}

func (gt GenerationType) isValid() bool {
	_, ok := generationTypes[gt]
	return ok
}

var generationTypeKeys = func() []string {
	ret := make([]string, len(generationTypes))
	i := 0
	for k := range generationTypes {
		ret[i] = string(k)
		i++
	}
	sort.Strings(ret)
	return ret
}

type OperationType string

const (

	// Adds two operands
	Add OperationType = "add"

	// Subtract the second operand from the first operand
	Subtract OperationType = "subtract"

	// Multiply two operands
	Multiply OperationType = "multiply"

	// Divides the first operand with the second operand
	Divide OperationType = "divide"

	// Calculates the percentage: (Operand1 / Operand2) * 100
	Percent OperationType = "percent"
)

var operationTypes = map[OperationType]struct{}{
	Add:      {},
	Subtract: {},
	Multiply: {},
	Divide:   {},
	Percent:  {},
}

func (ot OperationType) isValid() bool {
	_, ok := operationTypes[ot]
	return ok
}

var operationTypeKeys = func() []string {
	ret := make([]string, len(operationTypes))
	i := 0
	for k := range operationTypes {
		ret[i] = string(k)
		i++
	}
	sort.Strings(ret)
	return ret
}

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (config *Config) Validate() error {
	for _, rule := range config.Rules {
		if rule.NewMetricName == "" {
			return fmt.Errorf("missing required field %q", NewMetricFieldName)
		}

		if rule.Type == "" {
			return fmt.Errorf("missing required field %q", GenerationTypeFieldName)
		}

		if !rule.Type.isValid() {
			return fmt.Errorf("%q must be in %q", GenerationTypeFieldName, generationTypeKeys())
		}

		if rule.Operand1Metric == "" {
			return fmt.Errorf("missing required field %q", Operand1MetricFieldName)
		}

		if rule.Type == Calculate && rule.Operand2Metric == "" {
			return fmt.Errorf("missing required field %q for generation type %q", Operand2MetricFieldName, Calculate)
		}

		if rule.Type == Scale && rule.ScaleBy <= 0 {
			return fmt.Errorf("field %q required to be greater than 0 for generation type %q", ScaleByFieldName, Scale)
		}

		if rule.Operation != "" && !rule.Operation.isValid() {
			return fmt.Errorf("%q must be in %q", OperationFieldName, operationTypeKeys())
		}
	}
	return nil
}
