// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	// TargetDatadog routes to Datadog API
	TargetDatadog = "datadog"
	// TargetOTLP routes to OTLP endpoint
	TargetOTLP = "otlp"
)

// DetermineMetricRoute evaluates routing rules for a metric and returns the target route
func (cfg *MetricsRoutingConfig) DetermineMetricRoute(resourceMetric pmetric.ResourceMetrics, scopeMetric pmetric.ScopeMetrics) string {
	if !cfg.Enabled || len(cfg.Rules) == 0 {
		return TargetDatadog
	}

	for _, rule := range cfg.Rules {
		if cfg.evaluateMetricCondition(rule.Condition, resourceMetric, scopeMetric) {
			return rule.Target
		}
	}

	// Default to Datadog if no rules match
	return TargetDatadog
}

// evaluateMetricCondition checks if a metric matches the routing condition
func (cfg *MetricsRoutingConfig) evaluateMetricCondition(condition RoutingCondition, resourceMetric pmetric.ResourceMetrics, scopeMetric pmetric.ScopeMetrics) bool {
	// Check instrumentation scope name
	if condition.InstrumentationScopeName != "" {
		if scopeMetric.Scope().Name() != condition.InstrumentationScopeName {
			return false
		}
	}

	// Check resource attributes
	if len(condition.ResourceAttributes) > 0 {
		resourceAttrs := resourceMetric.Resource().Attributes()
		if !matchAttributes(resourceAttrs, condition.ResourceAttributes) {
			return false
		}
	}

	// For metrics, check scope attributes
	if len(condition.Attributes) > 0 {
		scopeAttrs := scopeMetric.Scope().Attributes()
		if !matchAttributes(scopeAttrs, condition.Attributes) {
			return false
		}
	}

	return true
}

// matchAttributes checks if attributes match the expected key-value pairs
func matchAttributes(attrs pcommon.Map, expected map[string]string) bool {
	for key, expectedValue := range expected {
		if value, exists := attrs.Get(key); !exists || value.AsString() != expectedValue {
			return false
		}
	}
	return true
}
