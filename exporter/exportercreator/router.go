// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator"

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// telemetryRouter routes telemetry to exporters based on resource attribute matching.
type telemetryRouter struct {
	mu        sync.RWMutex
	rules     []RoutingRule
	exporters map[observer.EndpointID]*routedExporter
	telemetry *metadata.TelemetryBuilder
	logger    *zap.Logger
}

// routedExporter holds an exporter with its endpoint properties for matching.
type routedExporter struct {
	exporter   component.Component
	properties map[string]any // Flattened endpoint properties
}

// newTelemetryRouter creates a new telemetry router with the given routing rules.
func newTelemetryRouter(rules []RoutingRule, telemetry *metadata.TelemetryBuilder) *telemetryRouter {
	return &telemetryRouter{
		rules:     rules,
		exporters: make(map[observer.EndpointID]*routedExporter),
		telemetry: telemetry,
		logger:    nil, // Will be set if logger is available
	}
}

// setLogger sets the logger for debug logging.
func (r *telemetryRouter) setLogger(logger *zap.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = logger
}

// AddExporter registers an exporter with its endpoint properties.
func (r *telemetryRouter) AddExporter(id observer.EndpointID, exp component.Component, env observer.EndpointEnv) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.exporters[id] = &routedExporter{
		exporter:   exp,
		properties: flattenProperties(env),
	}

	// Update the gauge metric with the current count
	if r.telemetry != nil {
		r.telemetry.ExporterCreatorExportersCount.Record(context.Background(), int64(len(r.exporters)))
	}
}

// RemoveExporter unregisters an exporter.
func (r *telemetryRouter) RemoveExporter(id observer.EndpointID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.exporters, id)

	// Update the gauge metric with the current count
	if r.telemetry != nil {
		r.telemetry.ExporterCreatorExportersCount.Record(context.Background(), int64(len(r.exporters)))
	}
}

// Route returns all exporters that match the given resource attributes.
func (r *telemetryRouter) Route(resourceAttrs pcommon.Map) []component.Component {
	if len(r.rules) == 0 {
		if r.logger != nil {
			r.logger.Debug("no routing rules configured")
		}
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.exporters) == 0 {
		if r.logger != nil {
			r.logger.Debug("no exporters available for routing")
		}
		return nil
	}

	var matched []component.Component
	for id, exp := range r.exporters {
		if r.matchesAllRules(resourceAttrs, exp.properties) {
			matched = append(matched, exp.exporter)
			if r.logger != nil {
				r.logger.Debug("exporter matched routing rules",
					zap.String("endpoint_id", string(id)),
				)
			}
		}
	}

	// Debug logging for routing decisions
	if len(matched) == 0 && r.logger != nil {
		// Log why routing failed - convert resource attrs to map for logging
		resourceAttrsMap := make(map[string]string)
		resourceAttrs.Range(func(k string, v pcommon.Value) bool {
			resourceAttrsMap[k] = v.AsString()
			return true
		})
		r.logger.Debug("no exporters matched routing rules",
			zap.Any("resource_attributes", resourceAttrsMap),
			zap.Int("available_exporters", len(r.exporters)),
			zap.Int("routing_rules", len(r.rules)),
		)
		// Log details about each exporter's properties for debugging
		for id, exp := range r.exporters {
			if spec, ok := exp.properties["spec"].(map[string]any); ok {
				if resourceAttrs, ok := spec["resourceAttributes"].(map[string]any); ok {
					r.logger.Debug("exporter endpoint properties",
						zap.String("endpoint_id", string(id)),
						zap.Any("spec.resourceAttributes", resourceAttrs),
					)
				}
			}
		}
	}

	return matched
}

// Count returns the current number of exporters.
func (r *telemetryRouter) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.exporters)
}

// matchesAllRules checks if all routing rules match for the given resource attributes and endpoint properties.
func (r *telemetryRouter) matchesAllRules(resourceAttrs pcommon.Map, properties map[string]any) bool {
	for _, rule := range r.rules {
		// Get the resource attribute value
		attrVal, ok := resourceAttrs.Get(rule.ResourceAttribute)
		if !ok {
			if r.logger != nil {
				r.logger.Debug("routing rule failed: resource attribute not found",
					zap.String("resource_attribute", rule.ResourceAttribute),
					zap.String("endpoint_property", rule.EndpointProperty),
				)
			}
			return false
		}

		// Get the endpoint property value using dot notation
		propVal := getNestedProperty(properties, rule.EndpointProperty)
		if propVal == nil {
			if r.logger != nil {
				r.logger.Debug("routing rule failed: endpoint property not found",
					zap.String("resource_attribute", rule.ResourceAttribute),
					zap.String("endpoint_property", rule.EndpointProperty),
					zap.Any("available_properties", getTopLevelKeys(properties)),
				)
			}
			return false
		}

		// Compare values
		attrStr := attrVal.AsString()
		propStr := toString(propVal)
		if attrStr != propStr {
			if r.logger != nil {
				r.logger.Debug("routing rule failed: values don't match",
					zap.String("resource_attribute", rule.ResourceAttribute),
					zap.String("resource_value", attrStr),
					zap.String("endpoint_property", rule.EndpointProperty),
					zap.String("endpoint_value", propStr),
				)
			}
			return false
		}
	}
	return true
}

// getTopLevelKeys returns the top-level keys of a map for debugging.
func getTopLevelKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		// Try to convert using fmt.Sprintf as a fallback
		return fmt.Sprintf("%v", val)
	}
}
