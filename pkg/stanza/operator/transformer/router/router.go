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

package router // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/router"

import (
	"context"
	"fmt"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func init() {
	operator.Register("router", func() operator.Builder { return NewOperatorConfig("") })
}

// NewOperatorConfig config creates a new router operator config with default values
func NewOperatorConfig(operatorID string) *OperatorConfig {
	return &OperatorConfig{
		BasicConfig: helper.NewBasicConfig(operatorID, "router"),
	}
}

// OperatorConfig is the configuration of a router operator
type OperatorConfig struct {
	helper.BasicConfig `mapstructure:",squash" yaml:",inline"`
	Routes             []*RouteConfig   `mapstructure:"routes" json:"routes" yaml:"routes"`
	Default            helper.OutputIDs `mapstructure:"default" json:"default" yaml:"default"`
}

// RouteConfig is the configuration of a route on a router operator
type RouteConfig struct {
	helper.AttributerConfig `mapstructure:",squash" yaml:",inline"`
	Expression              string           `mapstructure:"expr" json:"expr"   yaml:"expr"`
	OutputIDs               helper.OutputIDs `mapstructure:"output" json:"output" yaml:"output"`
}

// Build will build a router operator from the supplied configuration
func (c OperatorConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	basicOperator, err := c.BasicConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.Default != nil {
		defaultRoute := &RouteConfig{
			Expression: "true",
			OutputIDs:  c.Default,
		}
		c.Routes = append(c.Routes, defaultRoute)
	}

	routes := make([]*Route, 0, len(c.Routes))
	for _, routeConfig := range c.Routes {
		compiled, err := expr.Compile(routeConfig.Expression, expr.AsBool(), expr.AllowUndefinedVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to compile expression '%s': %w", routeConfig.Expression, err)
		}

		attributer, err := routeConfig.AttributerConfig.Build()
		if err != nil {
			return nil, fmt.Errorf("failed to build attributer for route '%s': %w", routeConfig.Expression, err)
		}

		route := Route{
			Attributer: attributer,
			Expression: compiled,
			OutputIDs:  routeConfig.OutputIDs,
		}
		routes = append(routes, &route)
	}

	return &Operator{
		BasicOperator: basicOperator,
		routes:        routes,
	}, nil
}

// Operator is an operator that routes entries based on matching expressions
type Operator struct {
	helper.BasicOperator
	routes []*Route
}

// Route is a route on a router operator
type Route struct {
	helper.Attributer
	Expression      *vm.Program
	OutputIDs       helper.OutputIDs
	OutputOperators []operator.Operator
}

// CanProcess will always return true for a router operator
func (p *Operator) CanProcess() bool {
	return true
}

// Process will route incoming entries based on matching expressions
func (p *Operator) Process(ctx context.Context, entry *entry.Entry) error {
	env := helper.GetExprEnv(entry)
	defer helper.PutExprEnv(env)

	for _, route := range p.routes {
		matches, err := vm.Run(route.Expression, env)
		if err != nil {
			p.Warnw("Running expression returned an error", zap.Error(err))
			continue
		}

		// we compile the expression with "AsBool", so this should be safe
		if matches.(bool) {
			if err := route.Attribute(entry); err != nil {
				p.Errorf("Failed to label entry: %s", err)
				return err
			}

			for _, output := range route.OutputOperators {
				_ = output.Process(ctx, entry)
			}
			break
		}
	}

	return nil
}

// CanOutput will always return true for a router operator
func (p *Operator) CanOutput() bool {
	return true
}

// Outputs will return all connected operators.
func (p *Operator) Outputs() []operator.Operator {
	outputs := make([]operator.Operator, 0, len(p.routes))
	for _, route := range p.routes {
		outputs = append(outputs, route.OutputOperators...)
	}
	return outputs
}

// GetOutputIDs will return all connected operators.
func (p *Operator) GetOutputIDs() []string {
	outputs := make([]string, 0, len(p.routes))
	for _, route := range p.routes {
		outputs = append(outputs, route.OutputIDs...)
	}
	return outputs
}

// SetOutputs will set the outputs of the router operator.
func (p *Operator) SetOutputs(operators []operator.Operator) error {
	for _, route := range p.routes {
		outputOperators, err := p.findOperators(operators, route.OutputIDs)
		if err != nil {
			return fmt.Errorf("failed to set outputs on route: %s", err)
		}
		route.OutputOperators = outputOperators
	}

	return nil
}

// SetOutputIDs will do nothing.
func (p *Operator) SetOutputIDs(opIDs []string) {}

// findOperators will find a subset of operators from a collection.
func (p *Operator) findOperators(operators []operator.Operator, operatorIDs []string) ([]operator.Operator, error) {
	result := make([]operator.Operator, 0)
	for _, operatorID := range operatorIDs {
		operator, err := p.findOperator(operators, operatorID)
		if err != nil {
			return nil, err
		}
		result = append(result, operator)
	}
	return result, nil
}

// findOperator will find an operator from a collection.
func (p *Operator) findOperator(operators []operator.Operator, operatorID string) (operator.Operator, error) {
	for _, operator := range operators {
		if operator.ID() == operatorID {
			return operator, nil
		}
	}
	return nil, fmt.Errorf("operator %s does not exist", operatorID)
}
