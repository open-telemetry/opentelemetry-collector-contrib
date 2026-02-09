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

// mockJSONFileEndpoint is a mock implementation of JSONFileEndpoint for testing
type mockJSONFileEndpoint struct {
	ID     string
	Name   string
	Labels map[string]string
}

func (m *mockJSONFileEndpoint) Env() observer.EndpointEnv {
	return map[string]any{
		"id":     m.ID,
		"name":   m.Name,
		"labels": m.Labels,
	}
}

func (*mockJSONFileEndpoint) Type() observer.EndpointType {
	return observer.JSONFileType
}

func TestExporterCreator_JSONObserverRouting(t *testing.T) {
	// Create JSON file observer endpoints
	jsonEndpoint1 := observer.Endpoint{
		ID:     observer.EndpointID("jsonfile_observer/service-a"),
		Target: "service-a.example.com:8080",
		Details: &mockJSONFileEndpoint{
			ID:     "service-a",
			Name:   "Service A",
			Labels: map[string]string{"service": "service-a", "env": "production", "region": "us-east"},
		},
	}

	jsonEndpoint2 := observer.Endpoint{
		ID:     observer.EndpointID("jsonfile_observer/service-b"),
		Target: "service-b.example.com:9090",
		Details: &mockJSONFileEndpoint{
			ID:     "service-b",
			Name:   "Service B",
			Labels: map[string]string{"service": "service-b", "env": "staging", "region": "us-west"},
		},
	}

	// Create exporter_creator config with routing rules
	cfg := createDefaultConfig().(*Config)
	cfg.WatchObservers = []component.ID{component.MustNewID("jsonfile_observer")}
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "service.name",
			EndpointProperty:  "labels.service",
		},
		{
			ResourceAttribute: "environment",
			EndpointProperty:  "labels.env",
		},
	}

	// Create exporter templates
	exporterCfg1 := exporterConfig{
		id: component.MustNewIDWithName("otlp", "json"),
		config: userConfigMap{
			"endpoint": "`target`",
		},
		endpointID: jsonEndpoint1.ID,
	}

	exporterCfg2 := exporterConfig{
		id: component.MustNewIDWithName("otlp", "json"),
		config: userConfigMap{
			"endpoint": "`target`",
		},
		endpointID: jsonEndpoint2.ID,
	}

	rule1, err := newRule(`type == "jsonfile" && labels["service"] == "service-a"`)
	require.NoError(t, err)

	rule2, err := newRule(`type == "jsonfile" && labels["service"] == "service-b"`)
	require.NoError(t, err)

	// Both endpoints will match different rules but use the same exporter type
	// Each endpoint will create its own exporter instance
	cfg.exporterTemplates = map[string]exporterTemplate{
		"otlp/json": {
			exporterConfig: exporterCfg1,
			rule:           rule1,
			Rule:           `type == "jsonfile" && labels["service"] == "service-a"`,
			ResourceAttributes: map[string]any{
				"service.name": "`labels[\"service\"]`",
				"environment":  "`labels[\"env\"]`",
			},
			signals: exporterSignals{metrics: true},
		},
		"otlp/json2": {
			exporterConfig: exporterCfg2,
			rule:           rule2,
			Rule:           `type == "jsonfile" && labels["service"] == "service-b"`,
			ResourceAttributes: map[string]any{
				"service.name": "`labels[\"service\"]`",
				"environment":  "`labels[\"env\"]`",
			},
			signals: exporterSignals{metrics: true},
		},
	}

	// Create telemetry router
	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	// Create observer handler
	handler, mr := newObserverHandler(t, cfg, router)

	// Add endpoints - this should create exporters
	handler.OnAdd([]observer.Endpoint{jsonEndpoint1, jsonEndpoint2})

	// Verify exporters were created
	assert.Equal(t, 2, len(handler.exportersByEndpoint))
	require.Len(t, mr.startedComponents, 2, "Should have two exporters started")

	// Get the created exporters
	exp1 := handler.exportersByEndpoint[jsonEndpoint1.ID]
	exp2 := handler.exportersByEndpoint[jsonEndpoint2.ID]
	require.NotNil(t, exp1, "Exporter 1 should be created")
	require.NotNil(t, exp2, "Exporter 2 should be created")

	// Create metrics with resource attributes matching service-a
	metrics1 := pmetric.NewMetrics()
	rm1 := metrics1.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", "service-a")
	rm1.Resource().Attributes().PutStr("environment", "production")
	sm1 := rm1.ScopeMetrics().AppendEmpty()
	metric1 := sm1.Metrics().AppendEmpty()
	metric1.SetName("test_metric_1")
	metric1.SetEmptyGauge()
	dp1 := metric1.Gauge().DataPoints().AppendEmpty()
	dp1.SetIntValue(100)

	// Create metrics with resource attributes matching service-b
	metrics2 := pmetric.NewMetrics()
	rm2 := metrics2.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", "service-b")
	rm2.Resource().Attributes().PutStr("environment", "staging")
	sm2 := rm2.ScopeMetrics().AppendEmpty()
	metric2 := sm2.Metrics().AppendEmpty()
	metric2.SetName("test_metric_2")
	metric2.SetEmptyGauge()
	dp2 := metric2.Gauge().DataPoints().AppendEmpty()
	dp2.SetIntValue(200)

	// Route metrics - service-a metrics should go to exp1, service-b to exp2
	matched1 := router.Route(rm1.Resource().Attributes())
	matched2 := router.Route(rm2.Resource().Attributes())

	// Verify routing
	assert.Len(t, matched1, 1, "Service-a metrics should match one exporter")
	assert.Len(t, matched2, 1, "Service-b metrics should match one exporter")
	assert.Equal(t, exp1, matched1[0], "Service-a metrics should route to service-a exporter")
	assert.Equal(t, exp2, matched2[0], "Service-b metrics should route to service-b exporter")

	// Verify exporters are different instances (compare pointers)
	assert.NotSame(t, exp1, exp2, "Exporters should be different instances")
	assert.Equal(t, 2, len(mr.startedComponents), "Should have two started components")
	assert.NotSame(t, mr.startedComponents[0], mr.startedComponents[1], "Started components should be different instances")
}

func TestExporterCreator_CRDObserverRouting(t *testing.T) {
	// Create CRD endpoints
	crdEndpoint1 := observer.Endpoint{
		ID:     observer.EndpointID("default/exporter-1-uid"),
		Target: "exporter-1",
		Details: &observer.K8sCRD{
			Name:      "exporter-1",
			UID:       "exporter-1-uid",
			Namespace: "default",
			Group:     "telemetry.opentelemetry.io",
			Version:   "v1alpha1",
			Kind:      "Exporter",
			Labels: map[string]string{
				"app": "app1",
			},
			Spec: map[string]any{
				"exporterType": "prometheusremotewrite",
				"endpoint":     "https://prometheus-1.example.com/api/v1/write",
				"resourceAttributes": map[string]any{
					"service_name": "app1-service", // Note: underscore, not dot
					"deployment":   "app1-deployment",
					"namespace":    "default",
				},
			},
		},
	}

	crdEndpoint2 := observer.Endpoint{
		ID:     observer.EndpointID("default/exporter-2-uid"),
		Target: "exporter-2",
		Details: &observer.K8sCRD{
			Name:      "exporter-2",
			UID:       "exporter-2-uid",
			Namespace: "default",
			Group:     "telemetry.opentelemetry.io",
			Version:   "v1alpha1",
			Kind:      "Exporter",
			Labels: map[string]string{
				"app": "app2",
			},
			Spec: map[string]any{
				"exporterType": "prometheusremotewrite",
				"endpoint":     "https://prometheus-2.example.com/api/v1/write",
				"resourceAttributes": map[string]any{
					"service_name": "app2-service", // Note: underscore, not dot
					"deployment":   "app2-deployment",
					"namespace":    "default",
				},
			},
		},
	}

	// Create exporter_creator config with routing rules
	// The routing rules match against the endpoint's native properties (spec.resourceAttributes for CRDs)
	cfg := createDefaultConfig().(*Config)
	cfg.WatchObservers = []component.ID{component.MustNewID("k8s_observer")}
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "service.name",
			EndpointProperty:  "spec.resourceAttributes.service_name", // Matches CRD spec structure
		},
		{
			ResourceAttribute: "deployment",
			EndpointProperty:  "spec.resourceAttributes.deployment", // Matches CRD spec structure
		},
	}

	// Create exporter templates
	exporterCfg1 := exporterConfig{
		id: component.MustNewIDWithName("prometheusremotewrite", "crd"),
		config: userConfigMap{
			"endpoint": "`spec[\"endpoint\"]`",
		},
		endpointID: crdEndpoint1.ID,
	}

	rule, err := newRule(`type == "k8s.crd" && kind == "Exporter" && spec["exporterType"] == "prometheusremotewrite"`)
	require.NoError(t, err)

	// Both endpoints will match the same rule and create exporters
	cfg.exporterTemplates = map[string]exporterTemplate{
		"prometheusremotewrite/crd": {
			exporterConfig: exporterCfg1,
			rule:           rule,
			Rule:           `type == "k8s.crd" && kind == "Exporter" && spec["exporterType"] == "prometheusremotewrite"`,
			ResourceAttributes: map[string]any{
				"service.name": "`spec[\"resourceAttributes\"][\"service_name\"]`",
				"deployment":   "`spec[\"resourceAttributes\"][\"deployment\"]`",
			},
			signals: exporterSignals{metrics: true},
		},
	}

	// Create telemetry router
	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	// Create observer handler
	handler, mr := newObserverHandler(t, cfg, router)

	// Add endpoints - this should create exporters
	handler.OnAdd([]observer.Endpoint{crdEndpoint1, crdEndpoint2})

	// Verify exporters were created
	assert.Equal(t, 2, len(handler.exportersByEndpoint))
	require.Len(t, mr.startedComponents, 2, "Should have two exporters started")

	// Get the created exporters
	exp1 := handler.exportersByEndpoint[crdEndpoint1.ID]
	exp2 := handler.exportersByEndpoint[crdEndpoint2.ID]
	require.NotNil(t, exp1, "Exporter 1 should be created")
	require.NotNil(t, exp2, "Exporter 2 should be created")

	// Create metrics with resource attributes matching app1
	metrics1 := pmetric.NewMetrics()
	rm1 := metrics1.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", "app1-service")
	rm1.Resource().Attributes().PutStr("deployment", "app1-deployment")
	sm1 := rm1.ScopeMetrics().AppendEmpty()
	metric1 := sm1.Metrics().AppendEmpty()
	metric1.SetName("app1_metric")
	metric1.SetEmptyGauge()
	dp1 := metric1.Gauge().DataPoints().AppendEmpty()
	dp1.SetIntValue(100)

	// Create metrics with resource attributes matching app2
	metrics2 := pmetric.NewMetrics()
	rm2 := metrics2.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", "app2-service")
	rm2.Resource().Attributes().PutStr("deployment", "app2-deployment")
	sm2 := rm2.ScopeMetrics().AppendEmpty()
	metric2 := sm2.Metrics().AppendEmpty()
	metric2.SetName("app2_metric")
	metric2.SetEmptyGauge()
	dp2 := metric2.Gauge().DataPoints().AppendEmpty()
	dp2.SetIntValue(200)

	// Route metrics
	matched1 := router.Route(rm1.Resource().Attributes())
	matched2 := router.Route(rm2.Resource().Attributes())

	// Verify routing
	assert.Len(t, matched1, 1, "App1 metrics should match one exporter")
	assert.Len(t, matched2, 1, "App2 metrics should match one exporter")
	assert.Equal(t, exp1, matched1[0], "App1 metrics should route to exporter-1")
	assert.Equal(t, exp2, matched2[0], "App2 metrics should route to exporter-2")

	// Verify exporters are different instances (compare pointers)
	assert.NotSame(t, exp1, exp2, "Exporters should be different instances")
	assert.Equal(t, 2, len(mr.startedComponents), "Should have two started components")
	assert.NotSame(t, mr.startedComponents[0], mr.startedComponents[1], "Started components should be different instances")
}

func TestExporterCreator_JSONObserverRouting_MultipleRules(t *testing.T) {
	// Test routing with multiple rules - metrics should match all rules
	jsonEndpoint := observer.Endpoint{
		ID:     observer.EndpointID("jsonfile_observer/multi-service"),
		Target: "multi-service.example.com:8080",
		Details: &mockJSONFileEndpoint{
			ID:   "multi-service",
			Name: "Multi Service",
			Labels: map[string]string{
				"service": "multi",
				"env":     "production",
				"region":  "us-east",
				"team":    "platform",
			},
		},
	}

	cfg := createDefaultConfig().(*Config)
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "service.name",
			EndpointProperty:  "labels.service",
		},
		{
			ResourceAttribute: "environment",
			EndpointProperty:  "labels.env",
		},
		{
			ResourceAttribute: "region",
			EndpointProperty:  "labels.region",
		},
	}

	exporterCfg := exporterConfig{
		id: component.MustNewIDWithName("otlp", "json"),
		config: userConfigMap{
			"endpoint": "`target`",
		},
		endpointID: jsonEndpoint.ID,
	}

	rule, err := newRule(`type == "jsonfile" && labels["service"] == "multi"`)
	require.NoError(t, err)

	cfg.exporterTemplates = map[string]exporterTemplate{
		"otlp/json/multi": {
			exporterConfig: exporterCfg,
			rule:           rule,
			Rule:           `type == "jsonfile" && labels["service"] == "multi"`,
			ResourceAttributes: map[string]any{
				"service.name": "`labels[\"service\"]`",
				"environment":  "`labels[\"env\"]`",
				"region":       "`labels[\"region\"]`",
			},
			signals: exporterSignals{metrics: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, mr := newObserverHandler(t, cfg, router)
	handler.OnAdd([]observer.Endpoint{jsonEndpoint})

	assert.Equal(t, 1, len(handler.exportersByEndpoint))
	require.Len(t, mr.startedComponents, 1)

	exp := handler.exportersByEndpoint[jsonEndpoint.ID]

	// Create metrics matching all routing rules
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "multi")
	rm.Resource().Attributes().PutStr("environment", "production")
	rm.Resource().Attributes().PutStr("region", "us-east")

	matched := router.Route(rm.Resource().Attributes())
	assert.Len(t, matched, 1, "Metrics should match exporter")
	assert.Equal(t, exp, matched[0], "Metrics should route to correct exporter")

	// Create metrics matching only some rules - should not match
	metrics2 := pmetric.NewMetrics()
	rm2 := metrics2.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", "multi")
	rm2.Resource().Attributes().PutStr("environment", "production")
	// Missing "region" attribute

	matched2 := router.Route(rm2.Resource().Attributes())
	assert.Len(t, matched2, 0, "Metrics missing required attribute should not match")
}

func TestExporterCreator_CRDObserverRouting_UnmatchedMetrics(t *testing.T) {
	// Test that unmatched metrics don't route to any exporter
	crdEndpoint := observer.Endpoint{
		ID:     observer.EndpointID("default/exporter-uid"),
		Target: "exporter",
		Details: &observer.K8sCRD{
			Name:      "exporter",
			UID:       "exporter-uid",
			Namespace: "default",
			Group:     "telemetry.opentelemetry.io",
			Version:   "v1alpha1",
			Kind:      "Exporter",
			Spec: map[string]any{
				"exporterType": "otlp",
				"endpoint":     "https://collector.example.com:4317",
				"resourceAttributes": map[string]any{
					"service_name": "matched-service", // Note: underscore, not dot
					"deployment":   "matched-deployment",
				},
			},
		},
	}

	cfg := createDefaultConfig().(*Config)
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "service.name",
			EndpointProperty:  "spec.resourceAttributes.service_name", // Matches CRD spec structure
		},
		{
			ResourceAttribute: "deployment",
			EndpointProperty:  "spec.resourceAttributes.deployment", // Matches CRD spec structure
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
		"otlp/crd": {
			exporterConfig: exporterCfg,
			rule:           rule,
			Rule:           `type == "k8s.crd" && kind == "Exporter"`,
			ResourceAttributes: map[string]any{
				"service.name": "`spec[\"resourceAttributes\"][\"service_name\"]`",
				"deployment":   "`spec[\"resourceAttributes\"][\"deployment\"]`",
			},
			signals: exporterSignals{metrics: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, _ := newObserverHandler(t, cfg, router)
	handler.OnAdd([]observer.Endpoint{crdEndpoint})

	exp := handler.exportersByEndpoint[crdEndpoint.ID]
	require.NotNil(t, exp)

	// Create metrics with matching attributes
	metrics1 := pmetric.NewMetrics()
	rm1 := metrics1.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", "matched-service")
	rm1.Resource().Attributes().PutStr("deployment", "matched-deployment")

	matched1 := router.Route(rm1.Resource().Attributes())
	assert.Len(t, matched1, 1, "Matching metrics should route to exporter")
	assert.Equal(t, exp, matched1[0])

	// Create metrics with non-matching attributes
	metrics2 := pmetric.NewMetrics()
	rm2 := metrics2.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", "unmatched-service")
	rm2.Resource().Attributes().PutStr("deployment", "unmatched-deployment")

	matched2 := router.Route(rm2.Resource().Attributes())
	assert.Len(t, matched2, 0, "Non-matching metrics should not route to any exporter")
}
