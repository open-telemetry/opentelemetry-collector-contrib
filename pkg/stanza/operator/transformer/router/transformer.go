// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package router // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/router"

import (
	"context"
	"errors"
	"fmt"

	"github.com/expr-lang/expr/vm"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer is an operator that routes entries based on matching expressions
type Transformer struct {
	helper.BasicOperator
	routes []*Route
}

// Route is a route on a router operator
type Route struct {
	helper.Attributer
	Expression      *vm.Program
	OutputIDs       []string
	OutputOperators []operator.Operator
}

// CanProcess will always return true for a router operator
func (t *Transformer) CanProcess() bool {
	return true
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	var errs []error
	for i := range entries {
		errs = append(errs, t.Process(ctx, entries[i]))
	}
	return errors.Join(errs...)
}

// Process will route incoming entries based on matching expressions
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	if entry == nil {
		return errors.New("got a nil entry, this should not happen and is potentially a bug")
	}

	env := helper.GetExprEnv(entry)
	defer helper.PutExprEnv(env)

	logFields := []zap.Field{
		zap.Any("entry.timestamp", entry.Timestamp),
	}
	for attrName, attrValue := range entry.Attributes {
		logFields = append(logFields, zap.Any(attrName, attrValue))
	}

	for _, route := range t.routes {
		matches, err := vm.Run(route.Expression, env)
		if err != nil {
			logFields = append(logFields, zap.Any("error", err))
			t.Logger().Warn("Running expression returned an error", logFields...)
			continue
		}

		// we compile the expression with "AsBool", so this should be safe
		if matches.(bool) {
			if err = route.Attribute(entry); err != nil {
				logFields = append(logFields, zap.Any("error", err))
				t.Logger().Error("Failed to label entry", logFields...)
				return err
			}

			for _, output := range route.OutputOperators {
				err = output.Process(ctx, entry)
				logFields = append(logFields, zap.Any("error", err))
				if err != nil {
					t.Logger().Error("Failed to process entry", logFields...)
				}
			}
			break
		}
	}

	return nil
}

// CanOutput will always return true for a router operator
func (t *Transformer) CanOutput() bool {
	return true
}

// Outputs will return all connected operators.
func (t *Transformer) Outputs() []operator.Operator {
	outputs := make([]operator.Operator, 0, len(t.routes))
	for _, route := range t.routes {
		outputs = append(outputs, route.OutputOperators...)
	}
	return outputs
}

// GetOutputIDs will return all connected operators.
func (t *Transformer) GetOutputIDs() []string {
	outputs := make([]string, 0, len(t.routes))
	for _, route := range t.routes {
		outputs = append(outputs, route.OutputIDs...)
	}
	return outputs
}

// SetOutputs will set the outputs of the router operator.
func (t *Transformer) SetOutputs(operators []operator.Operator) error {
	for _, route := range t.routes {
		outputOperators, err := t.findOperators(operators, route.OutputIDs)
		if err != nil {
			return fmt.Errorf("failed to set outputs on route: %w", err)
		}
		route.OutputOperators = outputOperators
	}

	return nil
}

// SetOutputIDs will do nothing.
func (t *Transformer) SetOutputIDs(_ []string) {}

// findOperators will find a subset of operators from a collection.
func (t *Transformer) findOperators(operators []operator.Operator, operatorIDs []string) ([]operator.Operator, error) {
	result := make([]operator.Operator, len(operatorIDs))
	for i, operatorID := range operatorIDs {
		operator, err := t.findOperator(operators, operatorID)
		if err != nil {
			return nil, err
		}
		result[i] = operator
	}
	return result, nil
}

// findOperator will find an operator from a collection.
func (t *Transformer) findOperator(operators []operator.Operator, operatorID string) (operator.Operator, error) {
	for _, operator := range operators {
		if operator.ID() == operatorID {
			return operator, nil
		}
	}
	return nil, fmt.Errorf("operator %s does not exist", operatorID)
}
