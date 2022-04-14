// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routingprocessor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/service/servicetest"
	"go.uber.org/zap"
)

func TestProcessorGetsCreatedWithValidConfiguration(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := componenttest.NewNopProcessorCreateSettings()
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		DefaultExporters:  []string{"otlp"},
		FromAttribute:     "X-Tenant",
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
	assert.ErrorIs(t, cfg.Validate(), errNoTableItems)
}

func TestProcessorFailsToBeCreatedWhenRouteHasNoExporters(t *testing.T) {
	// prepare
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		DefaultExporters:  []string{"otlp"},
		FromAttribute:     "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value: "acme",
			},
		},
	}
	assert.ErrorIs(t, cfg.Validate(), errNoExporters)
}

func TestProcessorFailsToBeCreatedWhenNoRoutesExist(t *testing.T) {
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		DefaultExporters:  []string{"otlp"},
		FromAttribute:     "X-Tenant",
		Table:             []RoutingTableItem{},
	}
	assert.ErrorIs(t, cfg.Validate(), errNoTableItems)
}

func TestProcessorFailsWithNoFromAttribute(t *testing.T) {
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		DefaultExporters:  []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	}
	assert.ErrorIs(t, cfg.Validate(), errNoMissingFromAttribute)
}

func TestShouldNotFailWhenNextIsProcessor(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := componenttest.NewNopProcessorCreateSettings()
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		DefaultExporters:  []string{"otlp"},
		FromAttribute:     "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	}
	mp := &mockProcessor{}
	next, err := processorhelper.NewTracesProcessor(cfg, consumertest.NewNop(), mp.processTraces)
	require.NoError(t, err)

	// test
	exp, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, next)

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestProcessorDoesNotFailToBuildExportersWithMultiplePipelines(t *testing.T) {
	// prepare
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	processorFactory := NewFactory()
	factories.Processors[typeStr] = processorFactory

	otlpExporterFactory := otlpexporter.NewFactory()
	factories.Exporters["otlp"] = otlpExporterFactory

	otlpConfig := &otlpexporter.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("otlp")),
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: "example.com:1234",
		},
	}

	otlpTracesExporter, err := otlpExporterFactory.CreateTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), otlpConfig)
	require.NoError(t, err)

	otlpMetricsExporter, err := otlpExporterFactory.CreateMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), otlpConfig)
	require.NoError(t, err)

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp/traces"): otlpTracesExporter,
				},
				config.MetricsDataType: {
					config.NewComponentID("otlp/metrics"): otlpMetricsExporter,
				},
			}
		},
	}

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config_multipipelines.yaml"), factories)
	assert.NoError(t, err)

	for _, cfg := range cfg.Processors {
		exp := newProcessor(zap.NewNop(), cfg)
		err = exp.Start(context.Background(), host)
		// assert that no error is thrown due to multiple pipelines and exporters not using the routing processor
		assert.NoError(t, err)
		assert.NoError(t, exp.Shutdown(context.Background()))
	}
}

func TestShutdown(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := componenttest.NewNopProcessorCreateSettings()
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		DefaultExporters:  []string{"otlp"},
		FromAttribute:     "X-Tenant",
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
