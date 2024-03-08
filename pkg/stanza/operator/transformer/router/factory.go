// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package router // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/router"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("router")

func init() {
	operator.RegisterFactory(NewFactory())
}

type factory struct{}

// NewFactory creates a factory
func NewFactory() operator.Factory {
	return &factory{}
}

// Type gets the type of the operator
func (f *factory) Type() component.Type {
	return operatorType
}

// NewDefaultConfig creates the default configuration
func (f *factory) NewDefaultConfig(operatorID string) component.Config {
	return &Config{
		BasicConfig: helper.NewBasicConfig(operatorID, operatorType.String()),
	}
}

// CreateOperator creates an operator
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	basicOperator, err := helper.NewBasicOperator(c.BasicConfig, set)
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
		compiled, err := helper.ExprCompileBool(routeConfig.Expression)
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

	return &Transformer{
		BasicOperator: basicOperator,
		routes:        routes,
	}, nil
}
