// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsgenerationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor"

import (
	"fmt"
	"sort"
)

const (
	// nameFieldName is the mapstructure field name for name field
	nameFieldName = "name"

	// typeFieldName is the mapstructure field name for Type field
	typeFieldName = "type"

	// metric1FieldName is the mapstructure field name for Metric1 field
	metric1FieldName = "metric1"

	// metric2FieldName is the mapstructure field name for Metric2 field
	metric2FieldName = "metric2"

	// scaleByFieldName is the mapstructure field name for ScaleBy field
	scaleByFieldName = "scale_by"

	// operationFieldName is the mapstructure field name for Operation field
	operationFieldName = "operation"
)

// Config defines the configuration for the processor.
type Config struct {
	// Set of rules for generating new metrics
	Rules []Rule `mapstructure:"rules"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type Rule struct {
	// Name of the new metric being generated. This is a required field.
	Name string `mapstructure:"name"`

	// Unit for the new metric being generated.
	Unit string `mapstructure:"unit"`

	// The rule type following which the new metric will be generated. This is a required field.
	Type GenerationType `mapstructure:"type"`

	// First operand metric to use in the calculation. This is a required field.
	Metric1 string `mapstructure:"metric1"`

	// Second operand metric to use in the calculation. A required field if the type is calculate.
	Metric2 string `mapstructure:"metric2"`

	// The arithmetic operation to apply for the calculation. This is a required field.
	Operation OperationType `mapstructure:"operation"`

	// A constant number by which the first operand will be scaled. A required field if the type is scale.
	ScaleBy float64 `mapstructure:"scale_by"`
}

type GenerationType string

const (

	// Generates a new metric applying an arithmetic operation with two operands
	calculate GenerationType = "calculate"

	// Generates a new metric scaling the value of s given metric with a provided constant
	scale GenerationType = "scale"
)

var generationTypes = map[GenerationType]struct{}{calculate: {}, scale: {}}

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
	add OperationType = "add"

	// subtract the second operand from the first operand
	subtract OperationType = "subtract"

	// multiply two operands
	multiply OperationType = "multiply"

	// Divides the first operand with the second operand
	divide OperationType = "divide"

	// Calculates the percentage: (Metric1 / Metric2) * 100
	percent OperationType = "percent"
)

var operationTypes = map[OperationType]struct{}{
	add:      {},
	subtract: {},
	multiply: {},
	divide:   {},
	percent:  {},
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
		if rule.Name == "" {
			return fmt.Errorf("missing required field %q", nameFieldName)
		}

		if rule.Type == "" {
			return fmt.Errorf("missing required field %q", typeFieldName)
		}

		if !rule.Type.isValid() {
			return fmt.Errorf("%q must be in %q", typeFieldName, generationTypeKeys())
		}

		if rule.Metric1 == "" {
			return fmt.Errorf("missing required field %q", metric1FieldName)
		}

		if rule.Type == calculate && rule.Metric2 == "" {
			return fmt.Errorf("missing required field %q for generation type %q", metric2FieldName, calculate)
		}

		if rule.Type == scale && rule.ScaleBy <= 0 {
			return fmt.Errorf("field %q required to be greater than 0 for generation type %q", scaleByFieldName, scale)
		}

		if rule.Operation != "" && !rule.Operation.isValid() {
			return fmt.Errorf("%q must be in %q", operationFieldName, operationTypeKeys())
		}

		switch rule.Name {
		case rule.Metric1:
			return fmt.Errorf("value of field %q may not match value of field %q", nameFieldName, metric1FieldName)
		case rule.Metric2:
			return fmt.Errorf("value of field %q may not match value of field %q", nameFieldName, metric2FieldName)
		}
	}
	return nil
}
