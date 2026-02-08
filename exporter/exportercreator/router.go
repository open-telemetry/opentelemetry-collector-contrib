// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator"

import (
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// telemetryRouter routes telemetry to exporters based on resource attribute matching.
type telemetryRouter struct {
	mu        sync.RWMutex
	rules     []RoutingRule
	exporters map[observer.EndpointID]*routedExporter
}

// routedExporter holds an exporter with its endpoint properties for matching.
type routedExporter struct {
	exporter   component.Component
	properties map[string]any // Flattened endpoint properties
}

// newTelemetryRouter creates a new telemetry router with the given routing rules.
func newTelemetryRouter(rules []RoutingRule) *telemetryRouter {
	return &telemetryRouter{
		rules:     rules,
		exporters: make(map[observer.EndpointID]*routedExporter),
	}
}

// AddExporter registers an exporter with its endpoint properties.
func (r *telemetryRouter) AddExporter(id observer.EndpointID, exp component.Component, env observer.EndpointEnv) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.exporters[id] = &routedExporter{
		exporter:   exp,
		properties: flattenProperties(env),
	}
}

// RemoveExporter unregisters an exporter.
func (r *telemetryRouter) RemoveExporter(id observer.EndpointID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.exporters, id)
}

// Route returns all exporters that match the given resource attributes.
func (r *telemetryRouter) Route(resourceAttrs pcommon.Map) []component.Component {
	if len(r.rules) == 0 {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []component.Component
	for _, exp := range r.exporters {
		if r.matchesAllRules(resourceAttrs, exp.properties) {
			matched = append(matched, exp.exporter)
		}
	}
	return matched
}

// matchesAllRules checks if all routing rules match for the given resource attributes and endpoint properties.
func (r *telemetryRouter) matchesAllRules(resourceAttrs pcommon.Map, properties map[string]any) bool {
	for _, rule := range r.rules {
		// Get the resource attribute value
		attrVal, ok := resourceAttrs.Get(rule.ResourceAttribute)
		if !ok {
			return false
		}

		// Get the endpoint property value using dot notation
		propVal := getNestedProperty(properties, rule.EndpointProperty)
		if propVal == nil {
			return false
		}

		// Compare values
		if attrVal.AsString() != toString(propVal) {
			return false
		}
	}
	return true
}

// flattenProperties converts the endpoint env to a flattened map for property access.
func flattenProperties(env observer.EndpointEnv) map[string]any {
	result := make(map[string]any)
	for k, v := range env {
		result[k] = v
	}
	return result
}

// getNestedProperty retrieves a nested property using dot notation (e.g., "labels.app", "spec.region").
func getNestedProperty(properties map[string]any, path string) any {
	parts := strings.Split(path, ".")
	current := any(properties)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		case map[string]string:
			current = v[part]
		case observer.EndpointEnv:
			current = v[part]
		default:
			return nil
		}
		if current == nil {
			return nil
		}
	}
	return current
}

// toString converts a value to string for comparison.
func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		return ""
	}
}
