// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// nopExporterComponent is a simple exporter component for testing
type nopExporterComponent struct {
	component.StartFunc
	component.ShutdownFunc
}

func TestTelemetryRouter_AddExporter(t *testing.T) {
	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter([]RoutingRule{}, telemetry)

	mockExporter := &nopExporterComponent{}
	env := observer.EndpointEnv{
		"labels": map[string]string{
			"app": "test",
		},
	}

	router.AddExporter(observer.EndpointID("endpoint-1"), mockExporter, env)

	assert.Equal(t, 1, router.Count())
}

func TestTelemetryRouter_RemoveExporter(t *testing.T) {
	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter([]RoutingRule{}, telemetry)

	mockExporter := &nopExporterComponent{}
	env := observer.EndpointEnv{
		"labels": map[string]string{
			"app": "test",
		},
	}

	router.AddExporter(observer.EndpointID("endpoint-1"), mockExporter, env)
	assert.Equal(t, 1, router.Count())

	router.RemoveExporter(observer.EndpointID("endpoint-1"))
	assert.Equal(t, 0, router.Count())
}

func TestTelemetryRouter_Route(t *testing.T) {
	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	rules := []RoutingRule{
		{
			ResourceAttribute: "app",
			EndpointProperty:  "labels.app",
		},
	}
	router := newTelemetryRouter(rules, telemetry)

	mockExporter := &nopExporterComponent{}
	env := observer.EndpointEnv{
		"labels": map[string]string{
			"app": "test",
		},
	}

	router.AddExporter(observer.EndpointID("endpoint-1"), mockExporter, env)

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("app", "test")
	matched := router.Route(resourceAttrs)
	assert.Len(t, matched, 1)

	// Test non-matching resource attributes
	resourceAttrs2 := pcommon.NewMap()
	resourceAttrs2.PutStr("app", "unknown")
	matched2 := router.Route(resourceAttrs2)
	assert.Len(t, matched2, 0)

	// Test no rules
	routerNoRules := newTelemetryRouter([]RoutingRule{}, telemetry)
	matched3 := routerNoRules.Route(resourceAttrs)
	assert.Nil(t, matched3)
}

func TestTelemetryRouter_MatchesAllRules(t *testing.T) {
	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	rules := []RoutingRule{
		{
			ResourceAttribute: "app",
			EndpointProperty:  "labels.app",
		},
		{
			ResourceAttribute: "region",
			EndpointProperty:  "labels.region",
		},
	}
	router := newTelemetryRouter(rules, telemetry)

	properties := map[string]any{
		"labels": map[string]string{
			"app":    "test",
			"region": "west",
		},
	}

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("app", "test")
	resourceAttrs.PutStr("region", "west")

	assert.True(t, router.matchesAllRules(resourceAttrs, properties))

	// Missing one attribute
	resourceAttrs2 := pcommon.NewMap()
	resourceAttrs2.PutStr("app", "test")
	assert.False(t, router.matchesAllRules(resourceAttrs2, properties))

	// Mismatched value
	resourceAttrs3 := pcommon.NewMap()
	resourceAttrs3.PutStr("app", "test")
	resourceAttrs3.PutStr("region", "east")
	assert.False(t, router.matchesAllRules(resourceAttrs3, properties))
}

func TestGetNestedProperty(t *testing.T) {
	properties := map[string]any{
		"labels": map[string]string{
			"app": "test",
		},
		"spec": map[string]any{
			"region": "west",
			"nested": map[string]any{
				"value": 123,
			},
			"resourceAttributes": map[string]any{
				"generator": "alpha",
				"service":   "my-service",
			},
		},
	}

	// Test simple property
	val := getNestedProperty(properties, "labels.app")
	assert.Equal(t, "test", val)

	// Test nested property
	val2 := getNestedProperty(properties, "spec.region")
	assert.Equal(t, "west", val2)

	// Test deeply nested property
	val3 := getNestedProperty(properties, "spec.nested.value")
	assert.Equal(t, 123, val3)

	// Test spec.resourceAttributes.generator (the actual use case)
	val4 := getNestedProperty(properties, "spec.resourceAttributes.generator")
	assert.Equal(t, "alpha", val4)

	// Test spec.resourceAttributes.service
	val5 := getNestedProperty(properties, "spec.resourceAttributes.service")
	assert.Equal(t, "my-service", val5)

	// Test non-existent property
	val6 := getNestedProperty(properties, "labels.missing")
	// When accessing a non-existent key in a map, Go returns the zero value for the type
	// For map[string]string, that's an empty string, not nil
	assert.Equal(t, "", val6)

	// Test non-existent path
	val7 := getNestedProperty(properties, "missing.path")
	assert.Nil(t, val7)
}

func TestFlattenProperties(t *testing.T) {
	env := observer.EndpointEnv{
		"labels": map[string]string{
			"app": "test",
		},
		"spec": map[string]any{
			"region": "west",
		},
		"simple": "value",
	}

	flattened := flattenProperties(env)
	// flattenProperties converts EndpointEnv to map[string]any, so we check the values match
	assert.Equal(t, env["labels"], flattened["labels"])
	assert.Equal(t, env["spec"], flattened["spec"])
	assert.Equal(t, env["simple"], flattened["simple"])
}

func TestToString(t *testing.T) {
	assert.Equal(t, "test", toString("test"))
	assert.Equal(t, "123", toString(123))
	assert.Equal(t, "true", toString(true))
	assert.Equal(t, "45.67", toString(45.67))
	assert.Equal(t, "", toString(nil))
}

func TestTelemetryRouter_Route_WithCRDSpec(t *testing.T) {
	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	rules := []RoutingRule{
		{
			ResourceAttribute: "generator",
			EndpointProperty:  "spec.resourceAttributes.generator",
		},
	}
	router := newTelemetryRouter(rules, telemetry)

	mockExporter := &nopExporterComponent{}
	env := observer.EndpointEnv{
		"spec": map[string]any{
			"exporterType": "prometheusremotewrite",
			"resourceAttributes": map[string]any{
				"generator": "alpha",
			},
		},
	}

	router.AddExporter(observer.EndpointID("endpoint-1"), mockExporter, env)

	// Test matching resource attributes
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("generator", "alpha")
	matched := router.Route(resourceAttrs)
	assert.Len(t, matched, 1, "Should match CRD exporter with generator=alpha")

	// Test non-matching resource attributes
	resourceAttrs2 := pcommon.NewMap()
	resourceAttrs2.PutStr("generator", "beta")
	matched2 := router.Route(resourceAttrs2)
	assert.Len(t, matched2, 0, "Should not match with generator=beta")
}
