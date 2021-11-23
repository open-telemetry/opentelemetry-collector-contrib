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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
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

func (mp *mockProcessor) processTraces(context.Context, pdata.Traces) (pdata.Traces, error) {
	return pdata.NewTraces(), nil
}
