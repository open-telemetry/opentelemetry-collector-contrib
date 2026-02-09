// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestExporterCreator_WithCRDEndpoint(t *testing.T) {
	// Create a CRD endpoint similar to what k8s_observer would provide
	crdEndpoint := observer.Endpoint{
		ID:     observer.EndpointID("default/prometheus-exporter-uid"),
		Target: "prometheus-remote-write-exporter",
		Details: &observer.K8sCRD{
			Name:      "prometheus-remote-write-exporter",
			UID:       "prometheus-exporter-uid",
			Namespace: "default",
			Group:     "telemetry.opentelemetry.io",
			Version:   "v1alpha1",
			Kind:      "Exporter",
			Labels: map[string]string{
				"app":         "prometheus",
				"environment": "production",
			},
			Annotations: map[string]string{
				"description": "Prometheus Remote Write exporter for production metrics",
			},
			Spec: map[string]any{
				"exporterType": "prometheusremotewrite",
				"endpoint":     "https://prometheus-remote-write.example.com/api/v1/write",
				"tls": map[string]any{
					"insecure":             false,
					"insecure_skip_verify": false,
				},
				"config": map[string]any{
					"resource_to_telemetry_conversion": map[string]any{
						"enabled": true,
					},
				},
				"resourceAttributes": map[string]any{
					"generator":   "prometheus",
					"environment": "production",
				},
			},
		},
	}

	// Create exporter_creator config with a template that matches this CRD
	cfg := createDefaultConfig().(*Config)
	cfg.WatchObservers = []component.ID{component.MustNewID("k8s_observer")}
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "generator",
			EndpointProperty:  "spec.resourceAttributes.generator",
		},
		{
			ResourceAttribute: "environment",
			EndpointProperty:  "spec.resourceAttributes.environment",
		},
	}

	exporterCfg := exporterConfig{
		id: component.MustNewIDWithName("prometheusremotewrite", "crd"),
		config: userConfigMap{
			"endpoint": "`spec[\"endpoint\"]`",
		},
		endpointID: crdEndpoint.ID,
	}

	rule, err := newRule(`type == "k8s.crd" && kind == "Exporter" && spec["exporterType"] == "prometheusremotewrite"`)
	require.NoError(t, err)

	cfg.exporterTemplates = map[string]exporterTemplate{
		exporterCfg.id.String(): {
			exporterConfig: exporterCfg,
			rule:           rule,
			Rule:           `type == "k8s.crd" && kind == "Exporter" && spec["exporterType"] == "prometheusremotewrite"`,
			ResourceAttributes: map[string]any{
				"generator":   "`spec[\"resourceAttributes\"][\"generator\"]`",
				"environment": "`spec[\"resourceAttributes\"][\"environment\"]`",
			},
			signals: exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	// Create observer handler and test OnAdd
	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, mr := newObserverHandler(t, cfg, router)
	handler.OnAdd([]observer.Endpoint{crdEndpoint})

	// Verify exporter was created
	assert.Equal(t, 1, len(handler.exportersByEndpoint))
	require.NoError(t, mr.lastError)
	require.NotNil(t, mr.startedComponent)

	// Verify exporter was registered with router
	assert.Equal(t, 1, router.Count())

	// Test routing - create metrics with matching resource attributes
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("generator", "prometheus")
	rm.Resource().Attributes().PutStr("environment", "production")

	matched := router.Route(rm.Resource().Attributes())
	assert.Len(t, matched, 1, "Should match the CRD-based exporter")

	// Test with non-matching attributes
	metrics2 := pmetric.NewMetrics()
	rm2 := metrics2.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("generator", "other")
	rm2.Resource().Attributes().PutStr("environment", "production")

	matched2 := router.Route(rm2.Resource().Attributes())
	assert.Len(t, matched2, 0, "Should not match with different generator")
}

func TestExporterCreator_WithMultipleCRDEndpoints(t *testing.T) {
	// Create multiple CRD endpoints
	otlpEndpoint := observer.Endpoint{
		ID:     observer.EndpointID("default/otlp-exporter-uid"),
		Target: "otlp-exporter",
		Details: &observer.K8sCRD{
			Name:      "otlp-exporter",
			UID:       "otlp-exporter-uid",
			Namespace: "default",
			Group:     "telemetry.opentelemetry.io",
			Version:   "v1alpha1",
			Kind:      "Exporter",
			Spec: map[string]any{
				"exporterType": "otlp",
				"endpoint":     "https://otlp-collector.example.com:4317",
				"tls": map[string]any{
					"insecure": true,
				},
				"config": map[string]any{
					"compression": "gzip",
				},
				"resourceAttributes": map[string]any{
					"generator":   "otlp",
					"environment": "staging",
				},
			},
		},
	}

	influxEndpoint := observer.Endpoint{
		ID:     observer.EndpointID("default/influxdb-exporter-uid"),
		Target: "influxdb-exporter",
		Details: &observer.K8sCRD{
			Name:      "influxdb-exporter",
			UID:       "influxdb-exporter-uid",
			Namespace: "default",
			Group:     "telemetry.opentelemetry.io",
			Version:   "v1alpha1",
			Kind:      "Exporter",
			Spec: map[string]any{
				"exporterType": "influxdb",
				"endpoint":     "http://influxdb.example.com:8086",
				"tls": map[string]any{
					"insecure":             true,
					"insecure_skip_verify": true,
				},
				"config": map[string]any{
					"org":    "my-org",
					"bucket": "telemetry",
					"token":  "secret-token",
				},
				"resourceAttributes": map[string]any{
					"generator":   "influxdb",
					"environment": "development",
				},
			},
		},
	}

	cfg := createDefaultConfig().(*Config)
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "generator",
			EndpointProperty:  "spec.resourceAttributes.generator",
		},
	}

	// Create templates for both exporter types
	otlpRule, err := newRule(`type == "k8s.crd" && kind == "Exporter" && spec["exporterType"] == "otlp"`)
	require.NoError(t, err)

	influxRule, err := newRule(`type == "k8s.crd" && kind == "Exporter" && spec["exporterType"] == "influxdb"`)
	require.NoError(t, err)

	cfg.exporterTemplates = map[string]exporterTemplate{
		"otlp/crd": {
			exporterConfig: exporterConfig{
				id: component.MustNewIDWithName("otlp", "crd"),
				config: userConfigMap{
					"endpoint": "`spec[\"endpoint\"]`",
				},
				endpointID: otlpEndpoint.ID,
			},
			rule: otlpRule,
			Rule: `type == "k8s.crd" && kind == "Exporter" && spec["exporterType"] == "otlp"`,
			ResourceAttributes: map[string]any{
				"generator": "`spec[\"resourceAttributes\"][\"generator\"]`",
			},
			signals: exporterSignals{metrics: true, logs: true, traces: true},
		},
		"influxdb/crd": {
			exporterConfig: exporterConfig{
				id: component.MustNewIDWithName("influxdb", "crd"),
				config: userConfigMap{
					"endpoint": "`spec[\"endpoint\"]`",
				},
				endpointID: influxEndpoint.ID,
			},
			rule: influxRule,
			Rule: `type == "k8s.crd" && kind == "Exporter" && spec["exporterType"] == "influxdb"`,
			ResourceAttributes: map[string]any{
				"generator": "`spec[\"resourceAttributes\"][\"generator\"]`",
			},
			signals: exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, mr := newObserverHandler(t, cfg, router)

	// Add both endpoints
	handler.OnAdd([]observer.Endpoint{otlpEndpoint, influxEndpoint})

	// Verify both exporters were created
	assert.Equal(t, 2, len(handler.exportersByEndpoint))
	assert.Equal(t, 2, router.Count())
	require.NoError(t, mr.lastError)

	// Test routing to OTLP exporter
	metricsOTLP := pmetric.NewMetrics()
	rmOTLP := metricsOTLP.ResourceMetrics().AppendEmpty()
	rmOTLP.Resource().Attributes().PutStr("generator", "otlp")

	matchedOTLP := router.Route(rmOTLP.Resource().Attributes())
	assert.Len(t, matchedOTLP, 1, "Should match OTLP exporter")

	// Test routing to InfluxDB exporter
	metricsInflux := pmetric.NewMetrics()
	rmInflux := metricsInflux.ResourceMetrics().AppendEmpty()
	rmInflux.Resource().Attributes().PutStr("generator", "influxdb")

	matchedInflux := router.Route(rmInflux.Resource().Attributes())
	assert.Len(t, matchedInflux, 1, "Should match InfluxDB exporter")

	// Test removal
	handler.OnRemove([]observer.Endpoint{otlpEndpoint})
	assert.Equal(t, 1, len(handler.exportersByEndpoint))
	assert.Equal(t, 1, router.Count())
}

func TestExporterCreator_CRDEndpointConfigExpansion(t *testing.T) {
	// Test that config values are properly expanded from CRD spec
	crdEndpoint := observer.Endpoint{
		ID:     observer.EndpointID("default/test-uid"),
		Target: "test-exporter",
		Details: &observer.K8sCRD{
			Name:      "test-exporter",
			UID:       "test-uid",
			Namespace: "default",
			Group:     "telemetry.opentelemetry.io",
			Version:   "v1alpha1",
			Kind:      "Exporter",
			Spec: map[string]any{
				"exporterType": "otlp",
				"endpoint":     "https://collector.example.com:4317",
				"tls": map[string]any{
					"insecure": true,
				},
				"config": map[string]any{
					"compression": "gzip",
				},
			},
		},
	}

	// Create template with config expansion
	cfg := createDefaultConfig().(*Config)
	exporterCfg := exporterConfig{
		id: component.MustNewIDWithName("otlp", "crd"),
		config: userConfigMap{
			"endpoint": "`spec[\"endpoint\"]`",
			"tls": map[string]any{
				"insecure": "`spec[\"tls\"][\"insecure\"]`",
			},
			"compression": "`spec[\"config\"][\"compression\"]`",
		},
		endpointID: crdEndpoint.ID,
	}

	rule, err := newRule(`type == "k8s.crd" && kind == "Exporter"`)
	require.NoError(t, err)

	cfg.exporterTemplates = map[string]exporterTemplate{
		exporterCfg.id.String(): {
			exporterConfig: exporterCfg,
			rule:           rule,
			Rule:           `type == "k8s.crd" && kind == "Exporter"`,
			signals:        exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, _ := newObserverHandler(t, cfg, router)

	// Get the endpoint environment
	env, err := crdEndpoint.Env()
	require.NoError(t, err)

	// Manually test config expansion
	expandedConfig, err := expandConfig(exporterCfg.config, env)
	require.NoError(t, err)

	// Verify expanded values
	assert.Equal(t, "https://collector.example.com:4317", expandedConfig["endpoint"])
	assert.Equal(t, true, expandedConfig["tls"].(map[string]any)["insecure"])
	assert.Equal(t, "gzip", expandedConfig["compression"])

	// Test that OnAdd would use this expanded config
	handler.OnAdd([]observer.Endpoint{crdEndpoint})
	assert.Equal(t, 1, len(handler.exportersByEndpoint))
}

func TestExporterCreator_CRDEndpointChange(t *testing.T) {
	// Test that endpoint changes are handled correctly
	initialEndpoint := observer.Endpoint{
		ID:     observer.EndpointID("default/test-uid"),
		Target: "test-exporter",
		Details: &observer.K8sCRD{
			Name:      "test-exporter",
			UID:       "test-uid",
			Namespace: "default",
			Group:     "telemetry.opentelemetry.io",
			Version:   "v1alpha1",
			Kind:      "Exporter",
			Spec: map[string]any{
				"exporterType": "otlp",
				"endpoint":     "https://old-endpoint.example.com:4317",
			},
		},
	}

	changedEndpoint := observer.Endpoint{
		ID:     observer.EndpointID("default/test-uid"),
		Target: "test-exporter",
		Details: &observer.K8sCRD{
			Name:      "test-exporter",
			UID:       "test-uid",
			Namespace: "default",
			Group:     "telemetry.opentelemetry.io",
			Version:   "v1alpha1",
			Kind:      "Exporter",
			Spec: map[string]any{
				"exporterType": "otlp",
				"endpoint":     "https://new-endpoint.example.com:4317",
			},
		},
	}

	cfg := createDefaultConfig().(*Config)
	exporterCfg := exporterConfig{
		id: component.MustNewIDWithName("otlp", "crd"),
		config: userConfigMap{
			"endpoint": "`spec[\"endpoint\"]`",
		},
		endpointID: initialEndpoint.ID,
	}

	rule, err := newRule(`type == "k8s.crd" && kind == "Exporter"`)
	require.NoError(t, err)

	cfg.exporterTemplates = map[string]exporterTemplate{
		exporterCfg.id.String(): {
			exporterConfig: exporterCfg,
			rule:           rule,
			Rule:           `type == "k8s.crd" && kind == "Exporter"`,
			signals:        exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, mr := newObserverHandler(t, cfg, router)

	// Add initial endpoint
	handler.OnAdd([]observer.Endpoint{initialEndpoint})
	assert.Equal(t, 1, len(handler.exportersByEndpoint))
	require.NotNil(t, mr.startedComponent, "Initial exporter should be created")
	require.Len(t, mr.startedComponents, 1, "Should have one started component")
	initialExp := mr.startedComponent

	// Simulate endpoint change
	handler.OnChange([]observer.Endpoint{changedEndpoint})

	// Verify old exporter was shut down and new one created
	require.NotNil(t, mr.shutdownComponent, "Old exporter should be shut down")
	assert.Equal(t, initialExp, mr.shutdownComponent, "Old exporter should be shut down")
	require.NotNil(t, mr.startedComponent, "New exporter should be created")
	require.Len(t, mr.startedComponents, 2, "Should have two started components (initial and new)")
	// Verify they are different instances by checking the list
	assert.Equal(t, initialExp, mr.startedComponents[0], "First component should be the initial one")
	assert.Equal(t, mr.startedComponent, mr.startedComponents[1], "Second component should be the new one")
	// Compare pointers to ensure they're different instances
	assert.NotSame(t, mr.startedComponents[0], mr.startedComponents[1], "Components should be different instances")
	assert.Equal(t, 1, len(handler.exportersByEndpoint), "Should still have one exporter")
}

func TestExporterCreator_CRDEndpointRouting(t *testing.T) {
	// Test routing with CRD-based resource attributes
	crdEndpoint := observer.Endpoint{
		ID:     observer.EndpointID("default/test-uid"),
		Target: "test-exporter",
		Details: &observer.K8sCRD{
			Name:      "test-exporter",
			UID:       "test-uid",
			Namespace: "default",
			Group:     "telemetry.opentelemetry.io",
			Version:   "v1alpha1",
			Kind:      "Exporter",
			Labels: map[string]string{
				"app": "myapp",
			},
			Spec: map[string]any{
				"exporterType": "otlp",
				"endpoint":     "https://collector.example.com:4317",
				"resourceAttributes": map[string]any{
					"service_name": "my-service",
					"deployment":   "my-deployment",
				},
			},
		},
	}

	cfg := createDefaultConfig().(*Config)
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "service_name",
			EndpointProperty:  "spec.resourceAttributes.service_name",
		},
		{
			ResourceAttribute: "deployment",
			EndpointProperty:  "spec.resourceAttributes.deployment",
		},
	}

	exporterCfg := exporterConfig{
		id: component.MustNewIDWithName("otlp", "crd"),
		config: userConfigMap{
			"endpoint": "`spec[\"endpoint\"]`",
		},
		endpointID: crdEndpoint.ID,
	}

	rule, err := newRule(`type == "k8s.crd" && kind == "Exporter"`)
	require.NoError(t, err)

	cfg.exporterTemplates = map[string]exporterTemplate{
		exporterCfg.id.String(): {
			exporterConfig: exporterCfg,
			rule:           rule,
			Rule:           `type == "k8s.crd" && kind == "Exporter"`,
			ResourceAttributes: map[string]any{
				"service_name": "`spec[\"resourceAttributes\"][\"service_name\"]`",
				"deployment":   "`spec[\"resourceAttributes\"][\"deployment\"]`",
			},
			signals: exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, _ := newObserverHandler(t, cfg, router)
	handler.OnAdd([]observer.Endpoint{crdEndpoint})

	// The router matches against endpoint properties, which come from the endpoint env
	// The endpoint env includes the spec, so we can access spec.resourceAttributes
	// But we need to verify the endpoint was added with the correct properties
	env, err := crdEndpoint.Env()
	require.NoError(t, err)

	// Verify the endpoint properties are correctly set up
	// The router should be able to match against spec.resourceAttributes
	properties := flattenProperties(env)
	spec, ok := properties["spec"].(map[string]any)
	require.True(t, ok, "Spec should be a map")
	resourceAttrs, ok := spec["resourceAttributes"].(map[string]any)
	require.True(t, ok, "resourceAttributes should be a map")
	assert.Equal(t, "my-service", resourceAttrs["service_name"])
	assert.Equal(t, "my-deployment", resourceAttrs["deployment"])

	// Create metrics with matching resource attributes
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service_name", "my-service")
	rm.Resource().Attributes().PutStr("deployment", "my-deployment")

	matched := router.Route(rm.Resource().Attributes())
	assert.Len(t, matched, 1, "Should route to CRD-based exporter")

	// Test with partial match (should not match)
	metrics2 := pmetric.NewMetrics()
	rm2 := metrics2.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service_name", "my-service")
	// Missing "deployment" attribute

	matched2 := router.Route(rm2.Resource().Attributes())
	assert.Len(t, matched2, 0, "Should not match with missing attributes")
}
