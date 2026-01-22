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
func (*Transformer) CanProcess() bool {
	return true
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	// Group entries by the route they match
	routeEntries := make(map[int][]*entry.Entry)

	for _, ent := range entries {
		if ent == nil {
			return errors.New("got a nil entry, this should not happen and is potentially a bug")
		}

		env := helper.GetExprEnv(ent)
		routeIdx := -1 // Track which route matched

		for idx, route := range t.routes {
			matches, err := vm.Run(route.Expression, env)
			if err != nil {
				t.Logger().Warn("Running expression returned an error", zapAttributes(ent, err)...)
				helper.PutExprEnv(env)
				continue
			}

			// we compile the expression with "AsBool", so this should be safe
			if matches.(bool) {
				if err = route.Attribute(ent); err != nil {
					t.Logger().Error("Failed to label entry", zapAttributes(ent, err)...)
					helper.PutExprEnv(env)
					return err
				}
				routeIdx = idx
				break
			}
		}
		helper.PutExprEnv(env)

		// Group entries by their matching route
		if routeIdx >= 0 {
			routeEntries[routeIdx] = append(routeEntries[routeIdx], ent)
		}
	}

	// Process batches for each route
	for routeIdx, batch := range routeEntries {
		route := t.routes[routeIdx]
		for _, output := range route.OutputOperators {
			if err := output.ProcessBatch(ctx, batch); err != nil {
				t.Logger().Error("Failed to process batch", zap.Int("batch_size", len(batch)), zap.Error(err))
			}
		}
	}

	return nil
}

// Process will route incoming entries based on matching expressions
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	if entry == nil {
		return errors.New("got a nil entry, this should not happen and is potentially a bug")
	}

	env := helper.GetExprEnv(entry)
	defer helper.PutExprEnv(env)

	for _, route := range t.routes {
		matches, err := vm.Run(route.Expression, env)
		if err != nil {
			t.Logger().Warn("Running expression returned an error", zapAttributes(entry, err)...)
			continue
		}

		// we compile the expression with "AsBool", so this should be safe
		if matches.(bool) {
			if err = route.Attribute(entry); err != nil {
				t.Logger().Error("Failed to label entry", zapAttributes(entry, err)...)
				return err
			}

			for _, output := range route.OutputOperators {
				if err = output.Process(ctx, entry); err != nil {
					t.Logger().Error("Failed to process entry", zapAttributes(entry, err)...)
				}
			}
			break
		}
	}

	return nil
}

// CanOutput will always return true for a router operator
func (*Transformer) CanOutput() bool {
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
func (*Transformer) SetOutputIDs(_ []string) {}

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
func (*Transformer) findOperator(operators []operator.Operator, operatorID string) (operator.Operator, error) {
	for _, operator := range operators {
		if operator.ID() == operatorID {
			return operator, nil
		}
	}
	return nil, fmt.Errorf("operator %s does not exist", operatorID)
}

func zapAttributes(entry *entry.Entry, err error) []zap.Field {
	logFields := make([]zap.Field, 0, 2+len(entry.Attributes))
	logFields = append(logFields, zap.Time("entry.timestamp", entry.Timestamp))
	for attrName, attrValue := range entry.Attributes {
		logFields = append(logFields, zap.Any(attrName, attrValue))
	}
	logFields = append(logFields, zap.Error(err))
	return logFields
}
