// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package router // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/router"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "router"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig config creates a new router operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID config creates a new router operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		BasicConfig: helper.NewBasicConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a router operator
type Config struct {
	helper.BasicConfig `mapstructure:",squash"`
	Routes             []*RouteConfig `mapstructure:"routes"`
	Default            []string       `mapstructure:"default"`
}

// RouteConfig is the configuration of a route on a router operator
type RouteConfig struct {
	helper.AttributerConfig `mapstructure:",squash"`
	Expression              string   `mapstructure:"expr"`
	OutputIDs               []string `mapstructure:"output"`
}

// Build will build a router operator from the supplied configuration
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	basicOperator, err := c.BasicConfig.Build(set)
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

		attributer, err := routeConfig.Build()
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
