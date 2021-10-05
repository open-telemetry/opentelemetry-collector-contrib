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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func TestRouteIsFoundForGRPCContexts(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{}
	wg.Add(1)

	exp := &processorImp{
		config: Config{
			FromAttribute: "X-Tenant",
		},
		logger: zap.NewNop(),
		traceExporters: map[string][]component.TracesExporter{
			"acme": {
				&mockExporter{
					ConsumeTracesFunc: func(context.Context, pdata.Traces) error {
						wg.Done()
						return nil
					},
				},
			},
		},
	}
	traces := pdata.NewTraces()

	// test
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("X-Tenant", "acme"))
	err := exp.ConsumeTraces(ctx, traces)

	// verify
	wg.Wait() // ensure that the exporter has been called
	assert.NoError(t, err)
}

func TestDefaultRouteIsUsedWhenRouteCantBeDetermined(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{
			"no key",
			context.Background(),
		},
		{
			"no value",
			metadata.NewIncomingContext(context.Background(), metadata.Pairs("X-Tenant", "acme")),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// prepare
			wg := &sync.WaitGroup{}
			wg.Add(1)

			exp := &processorImp{
				config: Config{
					FromAttribute: "X-Tenant",
				},
				logger: zap.NewNop(),
				defaultTracesExporters: []component.TracesExporter{
					&mockExporter{
						ConsumeTracesFunc: func(context.Context, pdata.Traces) error {
							wg.Done()
							return nil
						},
					},
				},
			}
			traces := pdata.NewTraces()

			// test
			err := exp.ConsumeTraces(tt.ctx, traces)

			// verify
			wg.Wait() // ensure that the exporter has been called
			assert.NoError(t, err)

		})
	}
}

func TestRegisterExportersForValidRoute(t *testing.T) {
	//  prepare
	exp, err := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})
	require.NoError(t, err)

	otlpExpFactory := otlpexporter.NewFactory()
	otlpConfig := &otlpexporter.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("otlp")),
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: "example.com:1234",
		},
	}
	otlpExp, err := otlpExpFactory.CreateTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), otlpConfig)
	require.NoError(t, err)
	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					otlpConfig.ID(): otlpExp,
				},
			}
		},
	}

	// test
	exp.Start(context.Background(), host)

	// verify
	assert.Contains(t, exp.traceExporters["acme"], otlpExp)
}

func TestErrorRequestedExporterNotFoundForRoute(t *testing.T) {
	//  prepare
	exp, err := newProcessor(zap.NewNop(), &Config{
		FromAttribute: "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"non-existing"},
			},
		},
	})
	require.NoError(t, err)
	host := &mockHost{
		Host: componenttest.NewNopHost(),
	}

	// test
	err = exp.Start(context.Background(), host)

	// verify
	assert.True(t, errors.Is(err, errExporterNotFound))
}

func TestErrorRequestedExporterNotFoundForDefaultRoute(t *testing.T) {
	//  prepare
	exp, err := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"non-existing"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})
	require.NoError(t, err)

	otlpExpFactory := otlpexporter.NewFactory()
	otlpConfig := &otlpexporter.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("otlp")),
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: "example.com:1234",
		},
	}
	otlpExp, err := otlpExpFactory.CreateTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), otlpConfig)
	require.NoError(t, err)
	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					otlpConfig.ID(): otlpExp,
				},
			}
		},
	}

	// test
	err = exp.Start(context.Background(), host)

	// verify
	assert.True(t, errors.Is(err, errExporterNotFound))
}

func TestInvalidExporter(t *testing.T) {
	//  prepare
	exp, err := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})
	require.NoError(t, err)

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp"): &mockComponent{},
				},
			}
		},
	}

	// test
	err = exp.Start(context.Background(), host)

	// verify
	assert.Error(t, err)
}

func TestValueFromExistingGRPCAttribute(t *testing.T) {
	// prepare
	exp, err := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("X-Tenant", "acme"))

	// test
	val := exp.extractValueFromContext(ctx)

	// verify
	assert.Equal(t, "acme", val)
}

func TestMultipleValuesFromExistingGRPCAttribute(t *testing.T) {
	// prepare
	exp, err := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("X-Tenant", "globex", "X-Tenant", "acme"))

	// test
	val := exp.extractValueFromContext(ctx)

	// verify
	assert.Equal(t, "globex", val)
}

func TestNoValuesFromExistingGRPCAttribute(t *testing.T) {
	// prepare
	exp, err := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("X-Tenant", ""))

	// test
	val := exp.extractValueFromContext(ctx)

	// verify
	assert.Equal(t, "", val)
}

func TestAttributeFromExistingGRPCContext(t *testing.T) {
	// prepare
	exp, err := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{}))

	// test
	val := exp.extractValueFromContext(ctx)

	// verify
	assert.Equal(t, "", val)
}

func TestNoAttributeInContext(t *testing.T) {
	// prepare
	exp, err := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})
	require.NoError(t, err)

	// test
	val := exp.extractValueFromContext(context.Background())

	// verify
	assert.Equal(t, "", val)
}

func TestFailedToPushDataToExporter(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{}
	expectedErr := errors.New("some error")
	wg.Add(2)
	exp := &processorImp{
		logger: zap.NewNop(),
		traceExporters: map[string][]component.TracesExporter{
			"acme": {
				&mockExporter{
					ConsumeTracesFunc: func(context.Context, pdata.Traces) error {
						wg.Done()
						return nil
					},
				},
				&mockExporter{ // this is a cross-test with the scenario with multiple exporters
					ConsumeTracesFunc: func(context.Context, pdata.Traces) error {
						wg.Done()
						return expectedErr
					},
				},
			},
		},
	}
	traces := pdata.NewTraces()

	// test
	err := exp.pushDataToExporters(context.Background(), traces, exp.traceExporters["acme"])

	// verify
	wg.Wait() // ensure that the exporter has been called
	assert.Equal(t, expectedErr, err)
}

func TestProcessorCapabilities(t *testing.T) {
	// prepare
	config := &Config{
		FromAttribute: "X-Tenant",
		Table: []RoutingTableItem{{
			Exporters: []string{"otlp"},
		}},
	}

	// test
	p, err := newProcessor(zap.NewNop(), config)
	caps := p.Capabilities()

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, false, caps.MutatesData)
}

type mockHost struct {
	component.Host
	GetExportersFunc func() map[config.DataType]map[config.ComponentID]component.Exporter
}

func (m *mockHost) GetExporters() map[config.DataType]map[config.ComponentID]component.Exporter {
	if m.GetExportersFunc != nil {
		return m.GetExportersFunc()
	}
	return m.Host.GetExporters()
}

type mockComponent struct{}

func (m *mockComponent) Start(context.Context, component.Host) error {
	return nil
}
func (m *mockComponent) Shutdown(context.Context) error {
	return nil
}

type mockExporter struct {
	mockComponent
	ConsumeTracesFunc func(ctx context.Context, td pdata.Traces) error
}

func (m *mockExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *mockExporter) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	if m.ConsumeTracesFunc != nil {
		return m.ConsumeTracesFunc(ctx, td)
	}
	return nil
}
