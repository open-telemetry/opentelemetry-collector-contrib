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
	"errors"
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

	// test
	exp, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())

	// verify
	assert.Nil(t, err)
	assert.NotNil(t, exp)
}

func TestFailOnEmptyConfiguration(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := componenttest.NewNopProcessorCreateSettings()
	cfg := factory.CreateDefaultConfig()

	// test
	exp, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())

	// verify
	assert.Error(t, err)
	assert.Nil(t, exp)
}

func TestProcessorFailsToBeCreatedWhenRouteHasNoExporters(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := componenttest.NewNopProcessorCreateSettings()
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

	// test
	exp, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())

	// verify
	assert.True(t, errors.Is(err, errNoExporters))
	assert.Nil(t, exp)
}

func TestProcessorFailsToBeCreatedWhenNoRoutesExist(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := componenttest.NewNopProcessorCreateSettings()
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		DefaultExporters:  []string{"otlp"},
		FromAttribute:     "X-Tenant",
		Table:             []RoutingTableItem{},
	}

	// test
	exp, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())

	// verify
	assert.True(t, errors.Is(err, errNoTableItems))
	assert.Nil(t, exp)
}

func TestProcessorFailsWithNoFromAttribute(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := componenttest.NewNopProcessorCreateSettings()
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

	// test
	exp, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())

	// verify
	assert.True(t, errors.Is(err, errNoMissingFromAttribute))
	assert.Nil(t, exp)
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
