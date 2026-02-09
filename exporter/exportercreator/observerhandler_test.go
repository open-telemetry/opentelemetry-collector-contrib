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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestObserverHandler_OnAdd(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	exporterCfg := exporterConfig{
		id:         component.MustNewIDWithName("otlp", "test"),
		config:     userConfigMap{"endpoint": "localhost:4317"},
		endpointID: portEndpoint.ID,
	}
	rule, err := newRule(`type == "port"`)
	require.NoError(t, err)
	cfg.exporterTemplates = map[string]exporterTemplate{
		exporterCfg.id.String(): {
			exporterConfig:     exporterCfg,
			rule:               rule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
			signals:            exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, mr := newObserverHandler(t, cfg, router)
	handler.OnAdd([]observer.Endpoint{portEndpoint, unsupportedEndpoint})

	assert.Equal(t, 1, len(handler.exportersByEndpoint))
	require.NoError(t, mr.lastError)
	require.NotNil(t, mr.startedComponent)
}

func TestObserverHandler_OnRemove(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	exporterCfg := exporterConfig{
		id:         component.MustNewIDWithName("otlp", "test"),
		config:     userConfigMap{"endpoint": "localhost:4317"},
		endpointID: portEndpoint.ID,
	}
	rule, err := newRule(`type == "port"`)
	require.NoError(t, err)
	cfg.exporterTemplates = map[string]exporterTemplate{
		exporterCfg.id.String(): {
			exporterConfig:     exporterCfg,
			rule:               rule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
			signals:            exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, mr := newObserverHandler(t, cfg, router)
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	exp := mr.startedComponent
	require.NotNil(t, exp)
	require.NoError(t, mr.lastError)

	handler.OnRemove([]observer.Endpoint{portEndpoint})

	assert.Equal(t, 0, len(handler.exportersByEndpoint))
	require.Same(t, exp, mr.shutdownComponent)
	require.NoError(t, mr.lastError)
}

func TestObserverHandler_OnChange(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	exporterCfg := exporterConfig{
		id:         component.MustNewIDWithName("otlp", "test"),
		config:     userConfigMap{"endpoint": "localhost:4317"},
		endpointID: portEndpoint.ID,
	}
	rule, err := newRule(`type == "port"`)
	require.NoError(t, err)
	cfg.exporterTemplates = map[string]exporterTemplate{
		exporterCfg.id.String(): {
			exporterConfig:     exporterCfg,
			rule:               rule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
			signals:            exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, mr := newObserverHandler(t, cfg, router)
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	origExp := mr.startedComponent
	require.NotNil(t, origExp)
	require.NoError(t, mr.lastError)

	handler.OnChange([]observer.Endpoint{portEndpoint})

	require.NoError(t, mr.lastError)
	assert.Same(t, origExp, mr.shutdownComponent)

	newExp := mr.startedComponent
	require.NotSame(t, origExp, newExp)
	require.NotNil(t, newExp)

	assert.Equal(t, 1, len(handler.exportersByEndpoint))
}

func TestObserverHandler_Shutdown(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	exporterCfg := exporterConfig{
		id:         component.MustNewIDWithName("otlp", "test"),
		config:     userConfigMap{"endpoint": "localhost:4317"},
		endpointID: portEndpoint.ID,
	}
	rule, err := newRule(`type == "port"`)
	require.NoError(t, err)
	cfg.exporterTemplates = map[string]exporterTemplate{
		exporterCfg.id.String(): {
			exporterConfig:     exporterCfg,
			rule:               rule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
			signals:            exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, mr := newObserverHandler(t, cfg, router)
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	require.Equal(t, 1, len(handler.exportersByEndpoint))

	err = handler.shutdown()
	require.NoError(t, err)
	assert.Equal(t, 1, len(mr.shutdownComponents))
}

func TestObserverHandler_OnAdd_MissingEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	// Template config without endpoint - exporter creation will be attempted
	// In real usage, OTLP exporter creation would fail during unmarshalling if endpoint is required
	// but missing. However, with the mock runner, it will succeed.
	// The key change is that we no longer require endpoint upfront - we let the exporter
	// factory handle validation during creation.
	exporterCfg := exporterConfig{
		id:         component.MustNewIDWithName("otlp", "test"),
		config:     userConfigMap{"tls": map[string]any{"insecure": true}}, // No endpoint field
		endpointID: portEndpoint.ID,
	}
	rule, err := newRule(`type == "port"`)
	require.NoError(t, err)
	cfg.exporterTemplates = map[string]exporterTemplate{
		exporterCfg.id.String(): {
			exporterConfig:     exporterCfg,
			rule:               rule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
			signals:            exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, mr := newObserverHandler(t, cfg, router)
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	// With mock runner, exporter will be created (mock always succeeds)
	// In real usage, exporter creation would fail if endpoint is required but missing
	assert.Equal(t, 1, len(handler.exportersByEndpoint))
	require.NotNil(t, mr.startedComponent)
}

func TestObserverHandler_OnAdd_EndpointExpandsToEmpty(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	// Template config with endpoint that expands to empty/nil - should fail and not create exporter
	exporterCfg := exporterConfig{
		id: component.MustNewIDWithName("otlp", "test"),
		// Endpoint expression that evaluates to nil (non-existent field)
		config:     userConfigMap{"endpoint": "`spec[\"nonexistent\"]`"},
		endpointID: portEndpoint.ID,
	}
	rule, err := newRule(`type == "port"`)
	require.NoError(t, err)
	cfg.exporterTemplates = map[string]exporterTemplate{
		exporterCfg.id.String(): {
			exporterConfig:     exporterCfg,
			rule:               rule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
			signals:            exporterSignals{metrics: true, logs: true, traces: true},
		},
	}

	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)

	handler, mr := newObserverHandler(t, cfg, router)
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	// Exporter should NOT be created when endpoint expands to empty/nil
	assert.Equal(t, 0, len(handler.exportersByEndpoint))
	// Runner's start method should NOT be called
	assert.Nil(t, mr.startedComponent)
	assert.Equal(t, 0, len(mr.startedComponents))
}

type mockRunner struct {
	startedComponent   component.Component
	startedComponents   []component.Component
	shutdownComponent  component.Component
	shutdownComponents  []component.Component
	lastError          error
}

func (r *mockRunner) start(
	exporterCfg exporterConfig,
	discoveredConfig userConfigMap,
	signals exporterSignals,
) (component.Component, error) {
	// Return a nop component for testing
	exp := &nopExporter{}
	r.startedComponent = exp
	r.startedComponents = append(r.startedComponents, exp)
	return exp, nil
}

func (r *mockRunner) shutdown(exp component.Component) error {
	r.shutdownComponent = exp
	r.shutdownComponents = append(r.shutdownComponents, exp)
	return nil
}

// nopExporter is a simple exporter component for testing that implements all exporter interfaces
type nopExporter struct {
	component.StartFunc
	component.ShutdownFunc
}

var (
	_ exporter.Logs    = (*nopExporter)(nil)
	_ exporter.Metrics = (*nopExporter)(nil)
	_ exporter.Traces   = (*nopExporter)(nil)
)

func (n *nopExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (n *nopExporter) Shutdown(ctx context.Context) error {
	return nil
}

func (n *nopExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (n *nopExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return nil
}

func (n *nopExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}

func (n *nopExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return nil
}

func newObserverHandler(
	t *testing.T,
	config *Config,
	router *telemetryRouter,
) (*observerHandler, *mockRunner) {
	set := exportertest.NewNopSettings(metadata.Type)
	set.ID = component.MustNewIDWithName("exporter_creator", "test")
	mr := &mockRunner{}
	return &observerHandler{
		params:              set,
		config:              config,
		exportersByEndpoint: make(map[observer.EndpointID]component.Component),
		router:              router,
		runner:              mr,
	}, mr
}
