// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestExporterRunner_Start(t *testing.T) {
	// Create a mock factory that returns nop exporters
	mockFactory := &mockExporterFactory{}
	host := &mockHostWithFactory{
		factories: map[component.Type]exporter.Factory{
			component.MustNewType("nop"): mockFactory,
		},
	}

	params := exportertest.NewNopSettings(metadata.Type)
	params.ID = component.MustNewIDWithName("exporter_creator", "test")
	runner := newExporterRunner(params, host)

	exporterCfg := exporterConfig{
		id:         component.MustNewIDWithName("nop", "test"),
		config:     userConfigMap{},
		endpointID: observer.EndpointID("endpoint-1"),
	}

	discoveredConfig := userConfigMap{}
	signals := exporterSignals{metrics: true, logs: true, traces: true}

	exp, err := runner.start(exporterCfg, discoveredConfig, signals)
	require.NoError(t, err)
	require.NotNil(t, exp)

	// Test that the exporter can be started
	err = exp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Test shutdown
	err = runner.shutdown(exp)
	require.NoError(t, err)
}

func TestExporterRunner_Shutdown(t *testing.T) {
	host := &mockHostWithFactory{
		factories: map[component.Type]exporter.Factory{},
	}

	params := exportertest.NewNopSettings(metadata.Type)
	runner := newExporterRunner(params, host)

	exp := &nopExporter{}
	err := runner.shutdown(exp)
	require.NoError(t, err)
}

func TestMergeTemplatedAndDiscoveredConfigs(t *testing.T) {
	factory := &mockExporterFactory{}

	// Test with endpoint in discovered config (template doesn't have endpoint)
	templated := userConfigMap{}
	discovered := userConfigMap{
		tmpSetEndpointConfigKey: struct{}{},
		endpointConfigKey:       "localhost:4317",
	}

	merged, endpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, templated, discovered)
	require.NoError(t, err)
	assert.Equal(t, "localhost:4317", endpoint)
	assert.NotNil(t, merged)

	// Test with endpoint in templated config (template endpoint should take precedence)
	templated2 := userConfigMap{
		"endpoint": "prometheus:9090/api/v1/write",
	}
	discovered2 := userConfigMap{
		tmpSetEndpointConfigKey: struct{}{},
		endpointConfigKey:       "prometheus-exporter", // This should be ignored
	}

	merged2, endpoint2, err2 := mergeTemplatedAndDiscoveredConfigs(factory, templated2, discovered2)
	require.NoError(t, err2)
	assert.Equal(t, "prometheus:9090/api/v1/write", endpoint2, "Template endpoint should take precedence")
	assert.NotNil(t, merged2)

	// Test with endpoint in templated config and no discovered config
	templated3 := userConfigMap{
		"endpoint": "localhost:4317",
	}
	discovered3 := userConfigMap{}

	merged3, endpoint3, err3 := mergeTemplatedAndDiscoveredConfigs(factory, templated3, discovered3)
	require.NoError(t, err3)
	assert.Equal(t, "localhost:4317", endpoint3)
	assert.NotNil(t, merged3)
}

func TestMergeTemplatedAndDiscoveredConfigs_AllTemplatedFields(t *testing.T) {
	factory := &mockExporterFactory{}

	// Test that all templated fields are preserved in the merged config
	templated := userConfigMap{
		"endpoint": "prometheus:9090/api/v1/write",
		"tls": map[string]any{
			"insecure":             false,
			"insecure_skip_verify": false,
		},
		"config": map[string]any{
			"resource_to_telemetry_conversion": map[string]any{
				"enabled": true,
			},
			"external_labels": map[string]any{
				"cluster": "production",
				"region":  "us-west-2",
			},
		},
		"headers": map[string]any{
			"X-Custom-Header": "value",
		},
		"timeout": "30s",
	}

	discovered := userConfigMap{
		tmpSetEndpointConfigKey: struct{}{},
		endpointConfigKey:       "prometheus-exporter", // Should be ignored since template has endpoint
		"additional_field":      "discovered-value",    // Should be merged in
	}

	merged, endpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, templated, discovered)
	require.NoError(t, err)
	assert.Equal(t, "prometheus:9090/api/v1/write", endpoint, "Template endpoint should be used")

	// Verify all templated fields are present in merged config
	mergedMap := merged.ToStringMap()

	// Verify endpoint
	assert.Equal(t, "prometheus:9090/api/v1/write", mergedMap["endpoint"], "Template endpoint should be preserved")
	assert.NotEqual(t, "prometheus-exporter", mergedMap["endpoint"], "Discovered endpoint should not override template")

	// Verify TLS config - confmap may return nested maps as *confmap.Conf or map[string]any
	tlsVal := mergedMap["tls"]
	require.NotNil(t, tlsVal, "TLS config should be present")
	var tlsConfig map[string]any
	switch v := tlsVal.(type) {
	case map[string]any:
		tlsConfig = v
	case *confmap.Conf:
		tlsConfig = v.ToStringMap()
	default:
		t.Fatalf("TLS config has unexpected type: %T", v)
	}
	assert.Equal(t, false, tlsConfig["insecure"])
	assert.Equal(t, false, tlsConfig["insecure_skip_verify"])

	// Verify config section
	configVal := mergedMap["config"]
	require.NotNil(t, configVal, "Config section should be present")
	var configSection map[string]any
	switch v := configVal.(type) {
	case map[string]any:
		configSection = v
	case *confmap.Conf:
		configSection = v.ToStringMap()
	default:
		t.Fatalf("Config section has unexpected type: %T", v)
	}

	rtcVal := configSection["resource_to_telemetry_conversion"]
	require.NotNil(t, rtcVal, "resource_to_telemetry_conversion should be present")
	var rtc map[string]any
	switch v := rtcVal.(type) {
	case map[string]any:
		rtc = v
	case *confmap.Conf:
		rtc = v.ToStringMap()
	default:
		t.Fatalf("resource_to_telemetry_conversion has unexpected type: %T", v)
	}
	assert.Equal(t, true, rtc["enabled"])

	externalLabelsVal := configSection["external_labels"]
	require.NotNil(t, externalLabelsVal, "external_labels should be present")
	var externalLabels map[string]any
	switch v := externalLabelsVal.(type) {
	case map[string]any:
		externalLabels = v
	case *confmap.Conf:
		externalLabels = v.ToStringMap()
	default:
		t.Fatalf("external_labels has unexpected type: %T", v)
	}
	assert.Equal(t, "production", externalLabels["cluster"])
	assert.Equal(t, "us-west-2", externalLabels["region"])

	// Verify headers
	headersVal := mergedMap["headers"]
	require.NotNil(t, headersVal, "Headers should be present")
	var headers map[string]any
	switch v := headersVal.(type) {
	case map[string]any:
		headers = v
	case *confmap.Conf:
		headers = v.ToStringMap()
	default:
		t.Fatalf("Headers has unexpected type: %T", v)
	}
	assert.Equal(t, "value", headers["X-Custom-Header"])

	// Verify timeout
	assert.Equal(t, "30s", mergedMap["timeout"])

	// Verify discovered field is also merged in
	assert.Equal(t, "discovered-value", mergedMap["additional_field"], "Discovered fields should be merged in")
}

func TestMergeTemplatedAndDiscoveredConfigs_NestedFields(t *testing.T) {
	factory := &mockExporterFactory{}

	// Test that deeply nested templated fields are preserved
	templated := userConfigMap{
		"endpoint": "https://collector.example.com:4317",
		"tls": map[string]any{
			"insecure":  true,
			"cert_file": "/path/to/cert.pem",
			"key_file":  "/path/to/key.pem",
			"ca_file":   "/path/to/ca.pem",
		},
		"auth": map[string]any{
			"authenticator": "bearer",
			"bearer": map[string]any{
				"token": "secret-token",
			},
		},
		"compression": "gzip",
	}

	discovered := userConfigMap{
		"extra_field": "extra-value",
	}

	merged, endpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, templated, discovered)
	require.NoError(t, err)
	assert.Equal(t, "https://collector.example.com:4317", endpoint)

	mergedMap := merged.ToStringMap()

	// Verify nested TLS fields - handle both map[string]any and *confmap.Conf
	tlsVal := mergedMap["tls"]
	require.NotNil(t, tlsVal, "TLS config should be present")
	var tlsConfig map[string]any
	switch v := tlsVal.(type) {
	case map[string]any:
		tlsConfig = v
	case *confmap.Conf:
		tlsConfig = v.ToStringMap()
	default:
		t.Fatalf("TLS config has unexpected type: %T", v)
	}
	assert.Equal(t, true, tlsConfig["insecure"])
	assert.Equal(t, "/path/to/cert.pem", tlsConfig["cert_file"])
	assert.Equal(t, "/path/to/key.pem", tlsConfig["key_file"])
	assert.Equal(t, "/path/to/ca.pem", tlsConfig["ca_file"])

	// Verify nested auth fields
	authVal := mergedMap["auth"]
	require.NotNil(t, authVal, "Auth config should be present")
	var authConfig map[string]any
	switch v := authVal.(type) {
	case map[string]any:
		authConfig = v
	case *confmap.Conf:
		authConfig = v.ToStringMap()
	default:
		t.Fatalf("Auth config has unexpected type: %T", v)
	}
	assert.Equal(t, "bearer", authConfig["authenticator"])

	bearerVal := authConfig["bearer"]
	require.NotNil(t, bearerVal, "Bearer config should be present")
	var bearerConfig map[string]any
	switch v := bearerVal.(type) {
	case map[string]any:
		bearerConfig = v
	case *confmap.Conf:
		bearerConfig = v.ToStringMap()
	default:
		t.Fatalf("Bearer config has unexpected type: %T", v)
	}
	assert.Equal(t, "secret-token", bearerConfig["token"])

	// Verify top-level fields
	assert.Equal(t, "gzip", mergedMap["compression"])
	assert.Equal(t, "extra-value", mergedMap["extra_field"], "Discovered fields should be merged")
}

func TestMergeTemplatedAndDiscoveredConfigs_ComplexPrometheusConfig(t *testing.T) {
	factory := &mockExporterFactory{}

	// Test a realistic prometheusremotewrite configuration
	templated := userConfigMap{
		"endpoint": "prometheus:9090/api/v1/write",
		"tls": map[string]any{
			"insecure":             false,
			"insecure_skip_verify": false,
		},
		"config": map[string]any{
			"resource_to_telemetry_conversion": map[string]any{
				"enabled": true,
			},
		},
		"external_labels": map[string]any{
			"cluster":   "prod",
			"namespace": "monitoring",
		},
		"headers": map[string]any{
			"X-Prometheus-Remote-Write-Version": "0.1.0",
		},
		"timeout": "10s",
		"retry_on_failure": map[string]any{
			"enabled":          true,
			"initial_interval": "5s",
			"max_interval":     "30s",
		},
	}

	discovered := userConfigMap{
		tmpSetEndpointConfigKey: struct{}{},
		endpointConfigKey:       "prometheus-exporter", // Should be ignored
	}

	merged, endpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, templated, discovered)
	require.NoError(t, err)
	assert.Equal(t, "prometheus:9090/api/v1/write", endpoint)

	mergedMap := merged.ToStringMap()

	// Verify all fields are present
	assert.Equal(t, "prometheus:9090/api/v1/write", mergedMap["endpoint"])

	// Helper function to extract nested map from confmap
	getNestedMap := func(val any) map[string]any {
		switch v := val.(type) {
		case map[string]any:
			return v
		case *confmap.Conf:
			return v.ToStringMap()
		default:
			t.Fatalf("Unexpected type: %T", v)
			return nil
		}
	}

	tlsConfig := getNestedMap(mergedMap["tls"])
	assert.Equal(t, false, tlsConfig["insecure"])
	assert.Equal(t, false, tlsConfig["insecure_skip_verify"])

	configSection := getNestedMap(mergedMap["config"])
	rtc := getNestedMap(configSection["resource_to_telemetry_conversion"])
	assert.Equal(t, true, rtc["enabled"])

	externalLabels := getNestedMap(mergedMap["external_labels"])
	assert.Equal(t, "prod", externalLabels["cluster"])
	assert.Equal(t, "monitoring", externalLabels["namespace"])

	headers := getNestedMap(mergedMap["headers"])
	assert.Equal(t, "0.1.0", headers["X-Prometheus-Remote-Write-Version"])

	assert.Equal(t, "10s", mergedMap["timeout"])

	retryConfig := getNestedMap(mergedMap["retry_on_failure"])
	assert.Equal(t, true, retryConfig["enabled"])
	assert.Equal(t, "5s", retryConfig["initial_interval"])
	assert.Equal(t, "30s", retryConfig["max_interval"])
}

type mockExporterFactory struct {
	exporter.Factory
}

func (m *mockExporterFactory) Type() component.Type {
	return component.MustNewType("nop")
}

func (m *mockExporterFactory) CreateDefaultConfig() component.Config {
	return &mockExporterConfig{}
}

func (m *mockExporterFactory) CreateLogs(_ context.Context, _ exporter.Settings, _ component.Config) (exporter.Logs, error) {
	return newNopExporter(), nil
}

func (m *mockExporterFactory) CreateMetrics(_ context.Context, _ exporter.Settings, _ component.Config) (exporter.Metrics, error) {
	return newNopExporter(), nil
}

func (m *mockExporterFactory) CreateTraces(_ context.Context, _ exporter.Settings, _ component.Config) (exporter.Traces, error) {
	return newNopExporter(), nil
}

func newNopExporter() *nopExporter {
	return &nopExporter{}
}

type mockExporterConfig struct{}

type mockHostWithFactory struct {
	component.Host
	factories map[component.Type]exporter.Factory
}

func (m *mockHostWithFactory) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	if kind != component.KindExporter {
		return nil
	}
	return m.factories[componentType]
}

func (m *mockHostWithFactory) GetExtensions() map[component.ID]component.Component {
	return nil
}
