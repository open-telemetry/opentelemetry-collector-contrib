// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/componentchecker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
)

func TestPopulateActiveComponentsIntegration(t *testing.T) {
	// Load the sample configuration file
	configPath := filepath.Join("internal", "componentchecker", "testdata", "sample-config.yaml")

	// Verify the config file exists
	_, err := os.Stat(configPath)
	require.NoError(t, err, "sample-config.yaml should exist")

	// Create a resolver to load the configuration
	resolverSettings := confmap.ResolverSettings{
		URIs: []string{"file:" + configPath},
		ProviderFactories: []confmap.ProviderFactory{
			fileprovider.NewFactory(),
		},
		ConverterFactories: []confmap.ConverterFactory{},
	}

	resolver, err := confmap.NewResolver(resolverSettings)
	require.NoError(t, err, "should be able to create resolver")

	confMap, err := resolver.Resolve(context.Background())
	require.NoError(t, err, "should be able to load config file")

	// Create a realistic ModuleInfoJSON that matches the components in sample-config.yaml
	moduleInfoJSON := createModuleInfoFromSampleConfig()

	// Test PopulateActiveComponents with the loaded configuration
	activeComponents, err := componentchecker.PopulateActiveComponents(confMap, moduleInfoJSON)
	require.NoError(t, err, "PopulateActiveComponents should not return error")
	require.NotNil(t, activeComponents, "activeComponents should not be nil")

	// Verify that we have the expected components from the sample config
	// Expected: 2 extensions + components across 3 pipelines
	// Extensions: health_check, pprof (2)
	// Pipeline components: each component appears once per pipeline it's used in
	// - otlp: 3 times (traces, metrics, logs)
	// - hostmetrics: 1 time (metrics)
	// - memory_limiter: 3 times (traces, metrics, logs)
	// - batch: 3 times (traces, metrics, logs)
	// - debug: 3 times (traces, metrics, logs)
	// - otlphttp: 3 times (traces, metrics, logs)
	// - datadog/connector: 2 times (traces exporter, metrics receiver)
	// Total: 2 + 3 + 1 + 3 + 3 + 3 + 3 + 2 = 20
	expectedComponentCount := 20
	assert.Len(t, *activeComponents, expectedComponentCount, "should have expected number of active components")

	// Verify that extensions are present
	hasHealthCheck := false
	hasPprof := false

	// Verify that pipeline components are present
	hasOtlp := false
	hasHostmetrics := false
	hasBatch := false
	hasMemoryLimiter := false
	hasDebug := false
	hasOtlphttp := false

	for _, component := range *activeComponents {
		switch component.Type {
		case "health_check":
			hasHealthCheck = true
			assert.Equal(t, "extension", component.Kind)
			assert.Empty(t, component.Pipeline) // Extensions don't have pipelines
		case "pprof":
			hasPprof = true
			assert.Equal(t, "extension", component.Kind)
			assert.Empty(t, component.Pipeline)
		case "otlp":
			hasOtlp = true
			assert.Equal(t, "receiver", component.Kind)
			assert.Contains(t, []string{"traces", "metrics", "logs"}, component.Pipeline)
		case "hostmetrics":
			hasHostmetrics = true
			assert.Equal(t, "receiver", component.Kind)
			assert.Equal(t, "metrics", component.Pipeline)
		case "batch":
			hasBatch = true
			assert.Equal(t, "processor", component.Kind)
			assert.Contains(t, []string{"traces", "metrics", "logs"}, component.Pipeline)
		case "memory_limiter":
			hasMemoryLimiter = true
			assert.Equal(t, "processor", component.Kind)
			assert.Contains(t, []string{"traces", "metrics", "logs"}, component.Pipeline)
		case "debug":
			hasDebug = true
			assert.Equal(t, "exporter", component.Kind)
			assert.Contains(t, []string{"traces", "metrics", "logs"}, component.Pipeline)
		case "otlphttp":
			hasOtlphttp = true
			assert.Equal(t, "exporter", component.Kind)
			assert.Contains(t, []string{"traces", "metrics", "logs"}, component.Pipeline)
		}

		// Verify that all components have module information
		assert.NotEmpty(t, component.Gomod, "component %s should have gomod info", component.Type)
		assert.NotEmpty(t, component.Version, "component %s should have version info", component.Type)
		assert.NotEmpty(t, component.ID, "component %s should have ID", component.Type)
		assert.NotEmpty(t, component.Type, "component should have type")
		assert.NotEmpty(t, component.Kind, "component should have kind")
	}

	// Assert that all expected components are present
	assert.True(t, hasHealthCheck, "should have health_check extension")
	assert.True(t, hasPprof, "should have pprof extension")
	assert.True(t, hasOtlp, "should have otlp receiver")
	assert.True(t, hasHostmetrics, "should have hostmetrics receiver")
	assert.True(t, hasBatch, "should have batch processor")
	assert.True(t, hasMemoryLimiter, "should have memory_limiter processor")
	assert.True(t, hasDebug, "should have debug exporter")
	assert.True(t, hasOtlphttp, "should have otlphttp exporter")
}

func TestDataToFlattenedJSONStringIntegration(t *testing.T) {
	// Load the sample configuration file
	configPath := filepath.Join("internal", "componentchecker", "testdata", "sample-config.yaml")

	// Create a resolver to load the configuration
	resolverSettings := confmap.ResolverSettings{
		URIs: []string{"file:" + configPath},
		ProviderFactories: []confmap.ProviderFactory{
			fileprovider.NewFactory(),
		},
		ConverterFactories: []confmap.ConverterFactory{},
	}

	resolver, err := confmap.NewResolver(resolverSettings)
	require.NoError(t, err, "should be able to create resolver")

	confMap, err := resolver.Resolve(context.Background())
	require.NoError(t, err, "should be able to load config file")

	// Test DataToFlattenedJSONString with the loaded configuration
	jsonString := componentchecker.DataToFlattenedJSONString(confMap.ToStringMap())

	// Verify that the result is valid JSON and doesn't contain newlines or carriage returns
	assert.NotEmpty(t, jsonString, "JSON string should not be empty")
	assert.NotContains(t, jsonString, "\n", "JSON string should not contain newlines")
	assert.NotContains(t, jsonString, "\r", "JSON string should not contain carriage returns")

	// Verify it's valid JSON by attempting to unmarshal
	var result map[string]any
	err = json.Unmarshal([]byte(jsonString), &result)
	assert.NoError(t, err, "flattened JSON should be valid JSON")
}

// createModuleInfoFromSampleConfig creates a realistic ModuleInfoJSON
// that matches the components used in the sample-config.yaml
func createModuleInfoFromSampleConfig() *payload.ModuleInfoJSON {
	moduleInfo := payload.NewModuleInfoJSON()

	// Add realistic component information based on what's in the sample config
	components := []payload.CollectorModule{
		// Receivers
		{
			Type:       "otlp",
			Kind:       "receiver",
			Gomod:      "go.opentelemetry.io/collector/receiver/otlpreceiver",
			Version:    "v0.127.0",
			Configured: true,
		},
		{
			Type:       "hostmetrics",
			Kind:       "receiver",
			Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver",
			Version:    "v0.127.0",
			Configured: true,
		},

		// Processors
		{
			Type:       "batch",
			Kind:       "processor",
			Gomod:      "go.opentelemetry.io/collector/processor/batchprocessor",
			Version:    "v0.127.0",
			Configured: true,
		},
		{
			Type:       "memory_limiter",
			Kind:       "processor",
			Gomod:      "go.opentelemetry.io/collector/processor/memorylimiterprocessor",
			Version:    "v0.127.0",
			Configured: true,
		},

		// Exporters
		{
			Type:       "debug",
			Kind:       "exporter",
			Gomod:      "go.opentelemetry.io/collector/exporter/debugexporter",
			Version:    "v0.127.0",
			Configured: true,
		},
		{
			Type:       "otlphttp",
			Kind:       "exporter",
			Gomod:      "go.opentelemetry.io/collector/exporter/otlphttpexporter",
			Version:    "v0.127.0",
			Configured: true,
		},

		// Extensions
		{
			Type:       "health_check",
			Kind:       "extension",
			Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension",
			Version:    "v0.127.0",
			Configured: true,
		},
		{
			Type:       "pprof",
			Kind:       "extension",
			Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension",
			Version:    "v0.127.0",
			Configured: true,
		},

		// Connectors
		{
			Type:       "datadog",
			Kind:       "connector",
			Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector",
			Version:    "v0.127.0",
			Configured: true,
		},
	}

	for _, component := range components {
		moduleInfo.AddComponent(component)
	}

	return moduleInfo
}
