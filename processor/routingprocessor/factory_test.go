// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingprocessor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var noopTelemetrySettings = component.TelemetrySettings{
	TracerProvider: trace.NewNoopTracerProvider(),
	MeterProvider:  noop.NewMeterProvider(),
	Logger:         zap.NewNop(),
}

func TestProcessorGetsCreatedWithValidConfiguration(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := processortest.NewNopCreateSettings()
	cfg := &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	}

	t.Run("traces", func(t *testing.T) {
		exp, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())
		// verify
		assert.NoError(t, err)
		assert.NotNil(t, exp)
	})

	t.Run("metrics", func(t *testing.T) {
		exp, err := factory.CreateMetricsProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())
		// verify
		assert.NoError(t, err)
		assert.NotNil(t, exp)
	})

	t.Run("logs", func(t *testing.T) {
		exp, err := factory.CreateLogsProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())
		// verify
		assert.NoError(t, err)
		assert.NotNil(t, exp)
	})
}

func TestFailOnEmptyConfiguration(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	assert.ErrorIs(t, component.ValidateConfig(cfg), errNoTableItems)
}

func TestProcessorFailsToBeCreatedWhenRouteHasNoExporters(t *testing.T) {
	// prepare
	cfg := &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value: "acme",
			},
		},
	}
	assert.ErrorIs(t, component.ValidateConfig(cfg), errNoExporters)
}

func TestProcessorFailsToBeCreatedWhenNoRoutesExist(t *testing.T) {
	cfg := &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table:            []RoutingTableItem{},
	}
	assert.ErrorIs(t, component.ValidateConfig(cfg), errNoTableItems)
}

func TestProcessorFailsWithNoFromAttribute(t *testing.T) {
	cfg := &Config{
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	}
	assert.ErrorIs(t, component.ValidateConfig(cfg), errNoMissingFromAttribute)
}

func TestShouldNotFailWhenNextIsProcessor(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := processortest.NewNopCreateSettings()
	cfg := &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	}
	mp := &mockProcessor{}

	next, err := processorhelper.NewTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop(), mp.processTraces)
	require.NoError(t, err)

	// test
	exp, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, next)

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestProcessorDoesNotFailToBuildExportersWithMultiplePipelines(t *testing.T) {
	otlpExporterFactory := otlpexporter.NewFactory()
	otlpConfig := &otlpexporter.Config{
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: "example.com:1234",
		},
	}

	otlpTracesExporter, err := otlpExporterFactory.CreateTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), otlpConfig)
	require.NoError(t, err)

	otlpMetricsExporter, err := otlpExporterFactory.CreateMetricsExporter(context.Background(), exportertest.NewNopCreateSettings(), otlpConfig)
	require.NoError(t, err)

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeTraces: {
			component.NewID("otlp/traces"): otlpTracesExporter,
		},
		component.DataTypeMetrics: {
			component.NewID("otlp/metrics"): otlpMetricsExporter,
		},
	})

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	for k := range cm.ToStringMap() {
		// Check if all processor variations that are defined in test config can be actually created
		t.Run(k, func(t *testing.T) {
			cfg := createDefaultConfig()

			sub, err := cm.Sub(k)
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			exp, err := newMetricProcessor(noopTelemetrySettings, cfg)
			require.NoError(t, err)

			err = exp.Start(context.Background(), host)
			// assert that no error is thrown due to multiple pipelines and exporters not using the routing processor
			assert.NoError(t, err)
			assert.NoError(t, exp.Shutdown(context.Background()))
		})
	}
}

func TestShutdown(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := processortest.NewNopCreateSettings()
	cfg := &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	}

	exp, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, exp)

	// test
	err = exp.Shutdown(context.Background())

	// verify
	assert.NoError(t, err)
}

type mockProcessor struct{}

func (mp *mockProcessor) processTraces(context.Context, ptrace.Traces) (ptrace.Traces, error) {
	return ptrace.NewTraces(), nil
}

type mockHost struct {
	component.Host
	exps map[component.DataType]map[component.ID]component.Component
}

func newMockHost(exps map[component.DataType]map[component.ID]component.Component) component.Host {
	return &mockHost{
		Host: componenttest.NewNopHost(),
		exps: exps,
	}
}

func (m *mockHost) GetExporters() map[component.DataType]map[component.ID]component.Component {
	return m.exps
}

type mockComponent struct {
	component.StartFunc
	component.ShutdownFunc
}
