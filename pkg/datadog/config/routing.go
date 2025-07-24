// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"

import (
	"go.opentelemetry.io/collector/config/configopaque"
)

// RoutingConfig defines configuration for conditional routing of telemetry data
type RoutingConfig struct {
	// Enabled determines if routing is enabled
	Enabled bool `mapstructure:"enabled"`

	// OTLPEndpoint is the endpoint for OTLP routing
	OTLPEndpoint string `mapstructure:"otlp_endpoint"`

	// OTLPHeaders are headers to include when routing to OTLP endpoint
	OTLPHeaders map[string]configopaque.String `mapstructure:"otlp_headers"`

	// Rules define the routing conditions
	Rules []RoutingRule `mapstructure:"rules"`
}

// RoutingRule defines a condition and corresponding action for routing
type RoutingRule struct {
	// Name is a descriptive name for the rule
	Name string `mapstructure:"name"`

	// Condition defines when this rule applies
	Condition RoutingCondition `mapstructure:"condition"`

	// Target specifies where to route the data (datadog or otlp)
	Target string `mapstructure:"target"`
}

// RoutingCondition defines the condition for a routing rule
type RoutingCondition struct {
	// InstrumentationScopeName matches against the instrumentation scope name
	InstrumentationScopeName string `mapstructure:"instrumentation_scope_name"`

	// ResourceAttributes matches against resource attributes
	ResourceAttributes map[string]string `mapstructure:"resource_attributes"`

	// Attributes matches against span/metric/log attributes
	Attributes map[string]string `mapstructure:"attributes"`
}

// MetricsRoutingConfig is routing configuration specific to metrics
type MetricsRoutingConfig struct {
	RoutingConfig `mapstructure:",squash"`
}
