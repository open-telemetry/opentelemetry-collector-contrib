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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func TestMetricProcessorCapabilities(t *testing.T) {
	// prepare
	config := &Config{
		FromAttribute: "X-Tenant",
		Table: []RoutingTableItem{{
			Value:     "acme",
			Exporters: []string{"otlp"},
		}},
	}

	// test
	p := newMetricProcessor(zap.NewNop(), config)
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
					config.NewComponentID("otlp"):              defaultExp,
					config.NewComponentIDWithName("otlp", "2"): mExp,
				},
			}
		},
	}

	exp := newMetricProcessor(zap.NewNop(), &Config{
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

	m := pmetric.NewMetrics()

	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutString("X-Tenant", "acme")
	metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("cpu")
	metric.SetEmptyGauge()

	rm = m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutString("X-Tenant", "acme")
	metric = rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("cpu_system")
	metric.SetEmptyGauge()

	rm = m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutString("X-Tenant", "something-else")
	metric = rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("cpu_idle")
	metric.SetEmptyGauge()

	ctx := context.Background()
	require.NoError(t, exp.Start(ctx, host))
	require.NoError(t, exp.ConsumeMetrics(ctx, m))

	// The numbers below stem from the fact that data is routed and grouped
	// per resource attribute which is used for routing.
	// Hence the first 2 metrics are grouped together under one pmetric.Metrics.
	assert.Len(t, defaultExp.AllMetrics(), 1,
		"one metric should be routed to default exporter",
	)
	assert.Len(t, mExp.AllMetrics(), 1,
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
					config.NewComponentID("otlp"):              defaultExp,
					config.NewComponentIDWithName("otlp", "2"): mExp,
				},
			}
		},
	}

	exp := newMetricProcessor(zap.NewNop(), &Config{
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

	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutString("X-Tenant", "acme")

	t.Run("non default route is properly used", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeMetrics(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "acme",
			})),
			m,
		))
		assert.Len(t, defaultExp.AllMetrics(), 0,
			"metric should not be routed to default exporter",
		)
		assert.Len(t, mExp.AllMetrics(), 1,
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
		assert.Len(t, defaultExp.AllMetrics(), 1,
			"metric should be routed to default exporter",
		)
		assert.Len(t, mExp.AllMetrics(), 1,
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
					config.NewComponentID("otlp"):              defaultExp,
					config.NewComponentIDWithName("otlp", "2"): mExp,
				},
			}
		},
	}

	exp := newMetricProcessor(zap.NewNop(), &Config{
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
		m := pmetric.NewMetrics()
		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutString("X-Tenant", "acme")

		assert.NoError(t, exp.ConsumeMetrics(context.Background(), m))
		assert.Len(t, defaultExp.AllMetrics(), 0,
			"metric should not be routed to default exporter",
		)
		assert.Len(t, mExp.AllMetrics(), 1,
			"metric should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		m := pmetric.NewMetrics()
		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutString("X-Tenant", "some-custom-value")

		assert.NoError(t, exp.ConsumeMetrics(context.Background(), m))
		assert.Len(t, defaultExp.AllMetrics(), 1,
			"metric should be routed to default exporter",
		)
		assert.Len(t, mExp.AllMetrics(), 1,
			"metric should not be routed to non default exporter",
		)
	})
}

func TestMetrics_RoutingWorks_ResourceAttribute_DropsRoutingAttribute(t *testing.T) {
	defaultExp := &mockMetricsExporter{}
	mExp := &mockMetricsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.MetricsDataType: {
					config.NewComponentID("otlp"):              defaultExp,
					config.NewComponentIDWithName("otlp", "2"): mExp,
				},
			}
		},
	}

	exp := newMetricProcessor(zap.NewNop(), &Config{
		AttributeSource:              resourceAttributeSource,
		FromAttribute:                "X-Tenant",
		DropRoutingResourceAttribute: true,
		DefaultExporters:             []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutString("X-Tenant", "acme")
	rm.Resource().Attributes().PutString("attr", "acme")

	assert.NoError(t, exp.ConsumeMetrics(context.Background(), m))
	metrics := mExp.AllMetrics()
	require.Len(t, metrics, 1, "metric should be routed to non default exporter")
	require.Equal(t, 1, metrics[0].ResourceMetrics().Len())
	attrs := metrics[0].ResourceMetrics().At(0).Resource().Attributes()
	_, ok := attrs.Get("X-Tenant")
	assert.False(t, ok, "routing attribute should have been dropped")
	v, ok := attrs.Get("attr")
	assert.True(t, ok, "non routing attributes shouldn't be dropped")
	assert.Equal(t, "acme", v.StringVal())
}

type mockMetricsExporter struct {
	mockComponent
	consumertest.MetricsSink
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
						config.NewComponentID("otlp"):              defaultExp,
						config.NewComponentIDWithName("otlp", "2"): mExp,
					},
				}
			},
		}

		exp := newMetricProcessor(zap.NewNop(), cfg)
		assert.NoError(b, exp.Start(context.Background(), host))

		for i := 0; i < b.N; i++ {
			m := pmetric.NewMetrics()
			rm := m.ResourceMetrics().AppendEmpty()

			attrs := rm.Resource().Attributes()
			attrs.PutString("X-Tenant", "acme")
			attrs.PutString("X-Tenant1", "acme")
			attrs.PutString("X-Tenant2", "acme")

			assert.NoError(b, exp.ConsumeMetrics(context.Background(), m))
		}
	}

	runBenchmark(b, cfg)
}

func TestMetricsAreCorrectlySplitPerResourceAttributeRoutingWithOTTL(t *testing.T) {
	defaultExp := &mockMetricsExporter{}
	firstExp := &mockMetricsExporter{}
	secondExp := &mockMetricsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.MetricsDataType: {
					config.NewComponentID("otlp"):              defaultExp,
					config.NewComponentIDWithName("otlp", "1"): firstExp,
					config.NewComponentIDWithName("otlp", "2"): secondExp,
				},
			}
		},
	}

	exp := newMetricProcessor(zap.NewNop(), &Config{
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Expression: `route() where resource.attributes["value"] > 2.5`,
				Exporters:  []string{"otlp/1"},
			},
			{
				Expression: `route() where resource.attributes["value"] > 3.0`,
				Exporters:  []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	t.Run("metric matched by no expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		m := pmetric.NewMetrics()

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 0.0)
		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu")

		require.NoError(t, exp.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultExp.AllMetrics(), 1)
		assert.Len(t, firstExp.AllMetrics(), 0)
		assert.Len(t, secondExp.AllMetrics(), 0)
	})

	t.Run("metric matched by one of two expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		m := pmetric.NewMetrics()

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 2.7)
		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu")

		require.NoError(t, exp.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultExp.AllMetrics(), 0)
		assert.Len(t, firstExp.AllMetrics(), 1)
		assert.Len(t, secondExp.AllMetrics(), 0)
	})

	t.Run("metric matched by all expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		m := pmetric.NewMetrics()

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 5.0)
		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu")

		rm = m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 3.1)
		metric = rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu1")

		require.NoError(t, exp.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultExp.AllMetrics(), 0)
		assert.Len(t, firstExp.AllMetrics(), 1)
		assert.Len(t, secondExp.AllMetrics(), 1)

		assert.Equal(t, firstExp.AllMetrics()[0].MetricCount(), 2)
		assert.Equal(t, secondExp.AllMetrics()[0].MetricCount(), 2)
		assert.Equal(t, firstExp.AllMetrics(), secondExp.AllMetrics())
	})

	t.Run("one metric matched by all expressions, other matched by none", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		m := pmetric.NewMetrics()

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 5.0)
		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu")

		rm = m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", -1.0)
		metric = rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu1")

		require.NoError(t, exp.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultExp.AllMetrics(), 1)
		assert.Len(t, firstExp.AllMetrics(), 1)
		assert.Len(t, secondExp.AllMetrics(), 1)

		assert.Equal(t, firstExp.AllMetrics(), secondExp.AllMetrics())

		rmetric := defaultExp.AllMetrics()[0].ResourceMetrics().At(0)
		attr, ok := rmetric.Resource().Attributes().Get("value")
		assert.True(t, ok, "routing attribute must exists")
		assert.Equal(t, attr.DoubleVal(), float64(-1.0))
	})
}
