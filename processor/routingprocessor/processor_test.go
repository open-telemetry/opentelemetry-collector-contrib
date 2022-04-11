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
	"sync/atomic"
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

func TestTraces_RegisterExportersForValidRoute(t *testing.T) {
	// prepare
	exp := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})

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
	require.NoError(t, exp.Start(context.Background(), host))

	// verify
	assert.Contains(t, exp.router.tracesExporters["acme"], otlpExp)
}

func TestTraces_ErrorRequestedExporterNotFoundForRoute(t *testing.T) {
	//  prepare
	exp := newProcessor(zap.NewNop(), &Config{
		FromAttribute: "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"non-existing"},
			},
		},
	})
	host := &mockHost{
		Host: componenttest.NewNopHost(),
	}

	// test
	err := exp.Start(context.Background(), host)

	// verify
	assert.Truef(t, errors.Is(err, errNoExportersAfterRegistration), "got: %v", err)
}

func TestTraces_RequestedExporterNotFoundForDefaultRoute(t *testing.T) {
	//  prepare
	exp := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"non-existing"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})

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
	assert.True(t, errors.Is(err, errNoExportersAfterRegistration))
}

func TestTraces_InvalidExporter(t *testing.T) {
	//  prepare
	exp := newProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})

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
	err := exp.Start(context.Background(), host)

	// verify
	assert.Error(t, err)
}

func TestTraces_AreCorrectlySplitPerResourceAttributeRouting(t *testing.T) {
	defaultExp := &mockTracesExporter{}
	tExp := &mockTracesExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp"):   defaultExp,
					config.NewComponentID("otlp/2"): tExp,
				},
			}
		},
	}

	exp := newProcessor(zap.NewNop(), &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  resourceAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})

	tr := pdata.NewTraces()

	rl := tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().InsertString("X-Tenant", "acme")
	span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("span")

	rl = tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().InsertString("X-Tenant", "acme")
	span = rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("span1")

	rl = tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().InsertString("X-Tenant", "something-else")
	span = rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("span2")

	ctx := context.Background()
	require.NoError(t, exp.Start(ctx, host))
	require.NoError(t, exp.ConsumeTraces(ctx, tr))

	// The numbers below stem from the fact that data is routed and grouped
	// per resource attribute which is used for routing.
	// Hence the first 2 traces are grouped together under one pdata.Logs.
	assert.Equal(t, 1, defaultExp.getTraceCount(),
		"one log should be routed to default exporter",
	)
	assert.Equal(t, 1, tExp.getTraceCount(),
		"one log should be routed to non default exporter",
	)
}

func TestTraces_RoutingWorks_Context(t *testing.T) {
	defaultExp := &mockTracesExporter{}
	lExp := &mockTracesExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp"):   defaultExp,
					config.NewComponentID("otlp/2"): lExp,
				},
			}
		},
	}

	exp := newProcessor(zap.NewNop(), &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  contextAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	tr := pdata.NewTraces()
	rs := tr.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().InsertString("X-Tenant", "acme")

	t.Run("non default route is properly used", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeTraces(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "acme",
			})),
			tr,
		))
		assert.Equal(t, 0, defaultExp.getTraceCount(),
			"trace should not be routed to default exporter",
		)
		assert.Equal(t, 1, lExp.getTraceCount(),
			"trace should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeTraces(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "some-custom-value1",
			})),
			tr,
		))
		assert.Equal(t, 1, defaultExp.getTraceCount(),
			"trace should be routed to default exporter",
		)
		assert.Equal(t, 1, lExp.getTraceCount(),
			"trace should not be routed to non default exporter",
		)
	})
}

func TestTraces_RoutingWorks_ResourceAttribute(t *testing.T) {
	defaultExp := &mockTracesExporter{}
	mExp := &mockTracesExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp"):   defaultExp,
					config.NewComponentID("otlp/2"): mExp,
				},
			}
		},
	}

	exp := newProcessor(zap.NewNop(), &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  resourceAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	t.Run("non default route is properly used", func(t *testing.T) {
		tr := pdata.NewTraces()
		rs := tr.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().InsertString("X-Tenant", "acme")

		assert.NoError(t, exp.ConsumeTraces(context.Background(), tr))
		assert.Equal(t, 0, defaultExp.getTraceCount(),
			"trace should not be routed to default exporter",
		)
		assert.Equal(t, 1, mExp.getTraceCount(),
			"trace should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		tr := pdata.NewTraces()
		rs := tr.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().InsertString("X-Tenant", "some-custom-value")

		assert.NoError(t, exp.ConsumeTraces(context.Background(), tr))
		assert.Equal(t, 1, defaultExp.getTraceCount(),
			"trace should be routed to default exporter",
		)
		assert.Equal(t, 1, mExp.getTraceCount(),
			"trace should not be routed to non default exporter",
		)
	})
}

func TestProcessorCapabilities(t *testing.T) {
	// prepare
	config := &Config{
		FromAttribute: "X-Tenant",
		Table: []RoutingTableItem{{
			Value:     "acme",
			Exporters: []string{"otlp"},
		}},
	}

	// test
	p := newProcessor(zap.NewNop(), config)
	require.NotNil(t, p)

	// verify
	assert.Equal(t, false, p.Capabilities().MutatesData)
}

func TestMetrics_AreCorrectlySplitPerResourceAttributeRouting(t *testing.T) {
	defaultExp := &mockMetricsExporter{}
	mExp := &mockMetricsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.MetricsDataType: {
					config.NewComponentID("otlp"):   defaultExp,
					config.NewComponentID("otlp/2"): mExp,
				},
			}
		},
	}

	exp := newProcessor(zap.NewNop(), &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  resourceAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})

	m := pdata.NewMetrics()

	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().InsertString("X-Tenant", "acme")
	metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeGauge)
	metric.SetName("cpu")

	rm = m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().InsertString("X-Tenant", "acme")
	metric = rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeGauge)
	metric.SetName("cpu_system")

	rm = m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().InsertString("X-Tenant", "something-else")
	metric = rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeGauge)
	metric.SetName("cpu_idle")

	ctx := context.Background()
	require.NoError(t, exp.Start(ctx, host))
	require.NoError(t, exp.ConsumeMetrics(ctx, m))

	// The numbers below stem from the fact that data is routed and grouped
	// per resource attribute which is used for routing.
	// Hence the first 2 metrics are grouped together under one pdata.Metrics.
	assert.Equal(t, 1, defaultExp.getMetricCount(),
		"one metric should be routed to default exporter",
	)
	assert.Equal(t, 1, mExp.getMetricCount(),
		"one metric should be routed to non default exporter",
	)
}

func TestMetrics_RoutingWorks_Context(t *testing.T) {
	defaultExp := &mockMetricsExporter{}
	mExp := &mockMetricsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.MetricsDataType: {
					config.NewComponentID("otlp"):   defaultExp,
					config.NewComponentID("otlp/2"): mExp,
				},
			}
		},
	}

	exp := newProcessor(zap.NewNop(), &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  contextAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	m := pdata.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().InsertString("X-Tenant", "acme")

	t.Run("non default route is properly used", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeMetrics(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "acme",
			})),
			m,
		))
		assert.Equal(t, 0, defaultExp.getMetricCount(),
			"metric should not be routed to default exporter",
		)
		assert.Equal(t, 1, mExp.getMetricCount(),
			"metric should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeMetrics(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "some-custom-value1",
			})),
			m,
		))
		assert.Equal(t, 1, defaultExp.getMetricCount(),
			"metric should be routed to default exporter",
		)
		assert.Equal(t, 1, mExp.getMetricCount(),
			"metric should not be routed to non default exporter",
		)
	})
}

func TestMetrics_RoutingWorks_ResourceAttribute(t *testing.T) {
	defaultExp := &mockMetricsExporter{}
	mExp := &mockMetricsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.MetricsDataType: {
					config.NewComponentID("otlp"):   defaultExp,
					config.NewComponentID("otlp/2"): mExp,
				},
			}
		},
	}

	exp := newProcessor(zap.NewNop(), &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  resourceAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	t.Run("non default route is properly used", func(t *testing.T) {
		m := pdata.NewMetrics()
		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().InsertString("X-Tenant", "acme")

		assert.NoError(t, exp.ConsumeMetrics(context.Background(), m))
		assert.Equal(t, 0, defaultExp.getMetricCount(),
			"metric should not be routed to default exporter",
		)
		assert.Equal(t, 1, mExp.getMetricCount(),
			"metric should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		m := pdata.NewMetrics()
		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().InsertString("X-Tenant", "some-custom-value")

		assert.NoError(t, exp.ConsumeMetrics(context.Background(), m))
		assert.Equal(t, 1, defaultExp.getMetricCount(),
			"metric should be routed to default exporter",
		)
		assert.Equal(t, 1, mExp.getMetricCount(),
			"metric should not be routed to non default exporter",
		)
	})
}

func TestLogs_RoutingWorks_Context(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.LogsDataType: {
					config.NewComponentID("otlp"):   defaultExp,
					config.NewComponentID("otlp/2"): lExp,
				},
			}
		},
	}

	exp := newProcessor(zap.NewNop(), &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  contextAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	l := pdata.NewLogs()
	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().InsertString("X-Tenant", "acme")

	t.Run("non default route is properly used", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeLogs(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "acme",
			})),
			l,
		))
		assert.Equal(t, 0, defaultExp.getLogCount(),
			"log should not be routed to default exporter",
		)
		assert.Equal(t, 1, lExp.getLogCount(),
			"log should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeLogs(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "some-custom-value1",
			})),
			l,
		))
		assert.Equal(t, 1, defaultExp.getLogCount(),
			"log should be routed to default exporter",
		)
		assert.Equal(t, 1, lExp.getLogCount(),
			"log should not be routed to non default exporter",
		)
	})
}

func TestLogs_RoutingWorks_ResourceAttribute(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.LogsDataType: {
					config.NewComponentID("otlp"):   defaultExp,
					config.NewComponentID("otlp/2"): lExp,
				},
			}
		},
	}

	exp := newProcessor(zap.NewNop(), &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  resourceAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	t.Run("non default route is properly used", func(t *testing.T) {
		l := pdata.NewLogs()
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().InsertString("X-Tenant", "acme")

		assert.NoError(t, exp.ConsumeLogs(context.Background(), l))
		assert.Equal(t, 0, defaultExp.getLogCount(),
			"log should not be routed to default exporter",
		)
		assert.Equal(t, 1, lExp.getLogCount(),
			"log should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		l := pdata.NewLogs()
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().InsertString("X-Tenant", "some-custom-value")

		assert.NoError(t, exp.ConsumeLogs(context.Background(), l))
		assert.Equal(t, 1, defaultExp.getLogCount(),
			"log should be routed to default exporter",
		)
		assert.Equal(t, 1, lExp.getLogCount(),
			"log should not be routed to non default exporter",
		)
	})
}

func TestLogs_AreCorrectlySplitPerResourceAttributeRouting(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.LogsDataType: {
					config.NewComponentID("otlp"):   defaultExp,
					config.NewComponentID("otlp/2"): lExp,
				},
			}
		},
	}

	exp := newProcessor(zap.NewNop(), &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  resourceAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})

	l := pdata.NewLogs()

	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().InsertString("X-Tenant", "acme")
	log := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	log.SetName("mylog")

	rl = l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().InsertString("X-Tenant", "acme")
	log = rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	log.SetName("mylog1")

	rl = l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().InsertString("X-Tenant", "something-else")
	log = rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	log.SetName("mylog2")

	ctx := context.Background()
	require.NoError(t, exp.Start(ctx, host))
	require.NoError(t, exp.ConsumeLogs(ctx, l))

	// The numbers below stem from the fact that data is routed and grouped
	// per resource attribute which is used for routing.
	// Hence the first 2 metrics are grouped together under one pdata.Logs.
	assert.Equal(t, 1, defaultExp.getLogCount(),
		"one log should be routed to default exporter",
	)
	assert.Equal(t, 1, lExp.getLogCount(),
		"one log should be routed to non default exporter",
	)
}

func Benchmark_MetricsRouting_ResourceAttribute(b *testing.B) {
	cfg := &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  resourceAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	}

	runBenchmark := func(b *testing.B, cfg *Config) {
		defaultExp := &mockMetricsExporter{}
		mExp := &mockMetricsExporter{}

		host := &mockHost{
			Host: componenttest.NewNopHost(),
			GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
				return map[config.DataType]map[config.ComponentID]component.Exporter{
					config.MetricsDataType: {
						config.NewComponentID("otlp"):   defaultExp,
						config.NewComponentID("otlp/2"): mExp,
					},
				}
			},
		}

		exp := newProcessor(zap.NewNop(), cfg)
		exp.Start(context.Background(), host)

		for i := 0; i < b.N; i++ {
			m := pdata.NewMetrics()
			rm := m.ResourceMetrics().AppendEmpty()

			attrs := rm.Resource().Attributes()
			attrs.InsertString("X-Tenant", "acme")
			attrs.InsertString("X-Tenant1", "acme")
			attrs.InsertString("X-Tenant2", "acme")

			exp.ConsumeMetrics(context.Background(), m)
		}
	}

	runBenchmark(b, cfg)
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

type mockMetricsExporter struct {
	mockComponent
	metricCount int32
}

func (m *mockMetricsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *mockMetricsExporter) ConsumeMetrics(context.Context, pdata.Metrics) error {
	atomic.AddInt32(&m.metricCount, 1)
	return nil
}

func (m *mockMetricsExporter) getMetricCount() int {
	return int(atomic.LoadInt32(&m.metricCount))
}

type mockLogsExporter struct {
	mockComponent
	logCount int32
}

func (m *mockLogsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *mockLogsExporter) ConsumeLogs(context.Context, pdata.Logs) error {
	atomic.AddInt32(&m.logCount, 1)
	return nil
}

func (m *mockLogsExporter) getLogCount() int {
	return int(atomic.LoadInt32(&m.logCount))
}

type mockTracesExporter struct {
	mockComponent
	traceCount int32
}

func (m *mockTracesExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *mockTracesExporter) ConsumeTraces(context.Context, pdata.Traces) error {
	atomic.AddInt32(&m.traceCount, 1)
	return nil
}

func (m *mockTracesExporter) getTraceCount() int {
	return int(atomic.LoadInt32(&m.traceCount))
}
