// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	p, err := newMetricProcessor(noopTelemetrySettings, config)
	require.NoError(t, err)

	require.NotNil(t, p)

	// verify
	assert.Equal(t, false, p.Capabilities().MutatesData)
}

func TestMetrics_AreCorrectlySplitPerResourceAttributeRouting(t *testing.T) {
	defaultExp := &mockMetricsExporter{}
	mExp := &mockMetricsExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeMetrics: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): mExp,
		},
	})

	exp, err := newMetricProcessor(noopTelemetrySettings, &Config{
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
	require.NoError(t, err)

	m := pmetric.NewMetrics()

	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "acme")
	metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("cpu")
	metric.SetEmptyGauge()

	rm = m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "acme")
	metric = rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("cpu_system")
	metric.SetEmptyGauge()

	rm = m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "something-else")
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

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeMetrics: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): mExp,
		},
	})

	exp, err := newMetricProcessor(noopTelemetrySettings, &Config{
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
	require.NoError(t, err)

	require.NoError(t, exp.Start(context.Background(), host))

	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "acme")

	t.Run("grpc metadata: non default route is properly used", func(t *testing.T) {
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

	t.Run("grpc metadata: default route is taken when no matching route can be found", func(t *testing.T) {
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

	t.Run("client.Info metadata: non default route is properly used", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeMetrics(
			client.NewContext(context.Background(),
				client.Info{Metadata: client.NewMetadata(map[string][]string{
					"X-Tenant": {"acme"},
				})}),
			m,
		))
		assert.Len(t, defaultExp.AllMetrics(), 1,
			"metric should not be routed to default exporter",
		)
		assert.Len(t, mExp.AllMetrics(), 2,
			"metric should be routed to non default exporter",
		)
	})

	t.Run("client.Info metadata: default route is taken when no matching route can be found", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeMetrics(
			client.NewContext(context.Background(),
				client.Info{Metadata: client.NewMetadata(map[string][]string{
					"X-Tenant": {"some-custom-value1"},
				})}),
			m,
		))
		assert.Len(t, defaultExp.AllMetrics(), 2,
			"metric should be routed to default exporter",
		)
		assert.Len(t, mExp.AllMetrics(), 2,
			"metric should not be routed to non default exporter",
		)
	})

}

func TestMetrics_RoutingWorks_ResourceAttribute(t *testing.T) {
	defaultExp := &mockMetricsExporter{}
	mExp := &mockMetricsExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeMetrics: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): mExp,
		},
	})

	exp, err := newMetricProcessor(noopTelemetrySettings, &Config{
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
	require.NoError(t, err)

	require.NoError(t, exp.Start(context.Background(), host))

	t.Run("non default route is properly used", func(t *testing.T) {
		m := pmetric.NewMetrics()
		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("X-Tenant", "acme")

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
		rm.Resource().Attributes().PutStr("X-Tenant", "some-custom-value")

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

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeMetrics: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): mExp,
		},
	})

	exp, err := newMetricProcessor(noopTelemetrySettings, &Config{
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
	require.NoError(t, err)

	require.NoError(t, exp.Start(context.Background(), host))

	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "acme")
	rm.Resource().Attributes().PutStr("attr", "acme")

	assert.NoError(t, exp.ConsumeMetrics(context.Background(), m))
	metrics := mExp.AllMetrics()
	require.Len(t, metrics, 1, "metric should be routed to non default exporter")
	require.Equal(t, 1, metrics[0].ResourceMetrics().Len())
	attrs := metrics[0].ResourceMetrics().At(0).Resource().Attributes()
	_, ok := attrs.Get("X-Tenant")
	assert.False(t, ok, "routing attribute should have been dropped")
	v, ok := attrs.Get("attr")
	assert.True(t, ok, "non routing attributes shouldn't be dropped")
	assert.Equal(t, "acme", v.Str())
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

		host := newMockHost(map[component.DataType]map[component.ID]component.Component{
			component.DataTypeMetrics: {
				component.NewID("otlp"):              defaultExp,
				component.NewIDWithName("otlp", "2"): mExp,
			},
		})

		exp, err := newMetricProcessor(noopTelemetrySettings, cfg)
		require.NoError(b, err)

		assert.NoError(b, exp.Start(context.Background(), host))

		for i := 0; i < b.N; i++ {
			m := pmetric.NewMetrics()
			rm := m.ResourceMetrics().AppendEmpty()

			attrs := rm.Resource().Attributes()
			attrs.PutStr("X-Tenant", "acme")
			attrs.PutStr("X-Tenant1", "acme")
			attrs.PutStr("X-Tenant2", "acme")

			assert.NoError(b, exp.ConsumeMetrics(context.Background(), m))
		}
	}

	runBenchmark(b, cfg)
}

func TestMetricsAreCorrectlySplitPerResourceAttributeRoutingWithOTTL(t *testing.T) {
	defaultExp := &mockMetricsExporter{}
	firstExp := &mockMetricsExporter{}
	secondExp := &mockMetricsExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeMetrics: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "1"): firstExp,
			component.NewIDWithName("otlp", "2"): secondExp,
		},
	})

	exp, err := newMetricProcessor(noopTelemetrySettings, &Config{
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Statement: `route() where resource.attributes["value"] > 2.5`,
				Exporters: []string{"otlp/1"},
			},
			{
				Statement: `route() where resource.attributes["value"] > 3.0`,
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, err)

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
		assert.Equal(t, attr.Double(), float64(-1.0))
	})
}
