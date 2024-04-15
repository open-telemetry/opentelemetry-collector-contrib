// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestMetricsRegisterConsumersForValidRoute(t *testing.T) {
	metricsDefault := component.NewIDWithName(component.DataTypeMetrics, "default")
	metrics0 := component.NewIDWithName(component.DataTypeMetrics, "0")
	metrics1 := component.NewIDWithName(component.DataTypeMetrics, "1")

	cfg := &Config{
		DefaultPipelines: []component.ID{metricsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where attributes["X-Tenant"] == "acme"`,
				Pipelines: []component.ID{metrics0},
			},
			{
				Statement: `route() where attributes["X-Tenant"] == "*"`,
				Pipelines: []component.ID{metrics0, metrics1},
			},
		},
	}

	require.NoError(t, cfg.Validate())

	var defaultSink, sink0, sink1 consumertest.MetricsSink

	router := connector.NewMetricsRouter(map[component.ID]consumer.Metrics{
		metricsDefault: &defaultSink,
		metrics0:       &sink0,
		metrics1:       &sink1,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Metrics))

	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.False(t, conn.Capabilities().MutatesData)

	rtConn := conn.(*metricsConnector)
	require.NoError(t, err)
	require.Same(t, &defaultSink, rtConn.router.defaultConsumer)

	route, ok := rtConn.router.routes[rtConn.router.table[0].Statement]
	assert.True(t, ok)
	require.Same(t, &sink0, route.consumer)

	route, ok = rtConn.router.routes[rtConn.router.table[1].Statement]
	assert.True(t, ok)

	routeConsumer, err := router.Consumer(metrics0, metrics1)
	require.NoError(t, err)
	require.Equal(t, routeConsumer, route.consumer)

	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()
}

func TestMetricsAreCorrectlySplitPerResourceAttributeWithOTTL(t *testing.T) {
	metricsDefault := component.NewIDWithName(component.DataTypeMetrics, "default")
	metrics0 := component.NewIDWithName(component.DataTypeMetrics, "0")
	metrics1 := component.NewIDWithName(component.DataTypeMetrics, "1")

	cfg := &Config{
		DefaultPipelines: []component.ID{metricsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where attributes["value"] > 2.5`,
				Pipelines: []component.ID{metrics0},
			},
			{
				Statement: `route() where attributes["value"] > 3.0`,
				Pipelines: []component.ID{metrics1},
			},
			{
				Statement: `route() where attributes["value"] == 1.0`,
				Pipelines: []component.ID{metricsDefault, metrics0},
			},
		},
	}

	var defaultSink, sink0, sink1 consumertest.MetricsSink

	router := connector.NewMetricsRouter(map[component.ID]consumer.Metrics{
		metricsDefault: &defaultSink,
		metrics0:       &sink0,
		metrics1:       &sink1,
	})

	resetSinks := func() {
		defaultSink.Reset()
		sink0.Reset()
		sink1.Reset()
	}

	factory := NewFactory()
	conn, err := factory.CreateMetricsToMetrics(
		context.Background(),
		connectortest.NewNopCreateSettings(),
		cfg,
		router.(consumer.Metrics),
	)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	t.Run("metric matched by no expressions", func(t *testing.T) {
		resetSinks()

		m := pmetric.NewMetrics()

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 0.0)
		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu")

		require.NoError(t, conn.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultSink.AllMetrics(), 1)
		assert.Len(t, sink0.AllMetrics(), 0)
		assert.Len(t, sink1.AllMetrics(), 0)
	})

	t.Run("metric matched by one of two expressions", func(t *testing.T) {
		resetSinks()

		m := pmetric.NewMetrics()

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 2.7)
		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu")

		require.NoError(t, conn.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultSink.AllMetrics(), 0)
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Len(t, sink1.AllMetrics(), 0)
	})

	t.Run("metric matched by two expressions", func(t *testing.T) {
		resetSinks()

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

		require.NoError(t, conn.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultSink.AllMetrics(), 0)
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Len(t, sink1.AllMetrics(), 1)

		assert.Equal(t, sink0.AllMetrics()[0].MetricCount(), 2)
		assert.Equal(t, sink1.AllMetrics()[0].MetricCount(), 2)
		assert.Equal(t, sink0.AllMetrics(), sink1.AllMetrics())
	})

	t.Run("one metric matched by 2 expressions, others matched by none", func(t *testing.T) {
		resetSinks()

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

		require.NoError(t, conn.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultSink.AllMetrics(), 1)
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Len(t, sink1.AllMetrics(), 1)

		assert.Equal(t, sink0.AllMetrics(), sink1.AllMetrics())

		rmetric := defaultSink.AllMetrics()[0].ResourceMetrics().At(0)
		attr, ok := rmetric.Resource().Attributes().Get("value")
		assert.True(t, ok, "routing attribute must exist")
		assert.Equal(t, attr.Double(), float64(-1.0))
	})

	t.Run("metric matched by one expression, multiple pipelines", func(t *testing.T) {
		resetSinks()

		m := pmetric.NewMetrics()

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 1.0)
		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu")

		require.NoError(t, conn.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultSink.AllMetrics(), 1)
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Len(t, sink1.AllMetrics(), 0)

		assert.Equal(t, defaultSink.AllMetrics()[0].MetricCount(), 1)
		assert.Equal(t, sink0.AllMetrics()[0].MetricCount(), 1)
		assert.Equal(t, defaultSink.AllMetrics(), sink0.AllMetrics())
	})
}

func TestMetricsAreCorrectlyMatchOnceWithOTTL(t *testing.T) {
	metricsDefault := component.NewIDWithName(component.DataTypeMetrics, "default")
	metrics0 := component.NewIDWithName(component.DataTypeMetrics, "0")
	metrics1 := component.NewIDWithName(component.DataTypeMetrics, "1")

	cfg := &Config{
		DefaultPipelines: []component.ID{metricsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where attributes["value"] > 2.5`,
				Pipelines: []component.ID{metrics0},
			},
			{
				Statement: `route() where attributes["value"] > 3.0`,
				Pipelines: []component.ID{metrics1},
			},
			{
				Statement: `route() where attributes["value"] == 1.0`,
				Pipelines: []component.ID{metricsDefault, metrics0},
			},
		},
		MatchOnce: true,
	}

	var defaultSink, sink0, sink1 consumertest.MetricsSink

	router := connector.NewMetricsRouter(map[component.ID]consumer.Metrics{
		metricsDefault: &defaultSink,
		metrics0:       &sink0,
		metrics1:       &sink1,
	})

	resetSinks := func() {
		defaultSink.Reset()
		sink0.Reset()
		sink1.Reset()
	}

	factory := NewFactory()
	conn, err := factory.CreateMetricsToMetrics(
		context.Background(),
		connectortest.NewNopCreateSettings(),
		cfg,
		router.(consumer.Metrics),
	)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	t.Run("metric matched by no expressions", func(t *testing.T) {
		resetSinks()

		m := pmetric.NewMetrics()

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 0.0)
		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu")

		require.NoError(t, conn.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultSink.AllMetrics(), 1)
		assert.Len(t, sink0.AllMetrics(), 0)
		assert.Len(t, sink1.AllMetrics(), 0)
	})

	t.Run("metric matched by one of two expressions", func(t *testing.T) {
		resetSinks()

		m := pmetric.NewMetrics()

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 2.7)
		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu")

		require.NoError(t, conn.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultSink.AllMetrics(), 0)
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Len(t, sink1.AllMetrics(), 0)
	})

	t.Run("metric matched by two expressions, but sinks to one", func(t *testing.T) {
		resetSinks()

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

		require.NoError(t, conn.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultSink.AllMetrics(), 0)
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Len(t, sink1.AllMetrics(), 0)

		assert.Equal(t, sink0.AllMetrics()[0].MetricCount(), 2)
	})

	t.Run("one metric matched by 2 expressions, others matched by none", func(t *testing.T) {
		resetSinks()

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

		require.NoError(t, conn.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultSink.AllMetrics(), 1)
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Len(t, sink1.AllMetrics(), 0)

		rmetric := defaultSink.AllMetrics()[0].ResourceMetrics().At(0)
		attr, ok := rmetric.Resource().Attributes().Get("value")
		assert.True(t, ok, "routing attribute must exist")
		assert.Equal(t, attr.Double(), float64(-1.0))
	})

	t.Run("metric matched by one expression, multiple pipelines", func(t *testing.T) {
		resetSinks()

		m := pmetric.NewMetrics()

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutDouble("value", 1.0)
		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("cpu")

		require.NoError(t, conn.ConsumeMetrics(context.Background(), m))

		assert.Len(t, defaultSink.AllMetrics(), 1)
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Len(t, sink1.AllMetrics(), 0)

		assert.Equal(t, defaultSink.AllMetrics()[0].MetricCount(), 1)
		assert.Equal(t, sink0.AllMetrics()[0].MetricCount(), 1)
		assert.Equal(t, defaultSink.AllMetrics(), sink0.AllMetrics())
	})
}

func TestMetricsResourceAttributeDroppedByOTTL(t *testing.T) {
	metricsDefault := component.NewIDWithName(component.DataTypeMetrics, "default")
	metricsOther := component.NewIDWithName(component.DataTypeMetrics, "other")

	cfg := &Config{
		DefaultPipelines: []component.ID{metricsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `delete_key(attributes, "X-Tenant") where attributes["X-Tenant"] == "acme"`,
				Pipelines: []component.ID{metricsOther},
			},
		},
	}

	var sink0, sink1 consumertest.MetricsSink

	router := connector.NewMetricsRouter(map[component.ID]consumer.Metrics{
		metricsDefault: &sink0,
		metricsOther:   &sink1,
	})

	factory := NewFactory()
	conn, err := factory.CreateMetricsToMetrics(
		context.Background(),
		connectortest.NewNopCreateSettings(),
		cfg,
		router.(consumer.Metrics),
	)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "acme")
	rm.Resource().Attributes().PutStr("attr", "acme")

	assert.NoError(t, conn.ConsumeMetrics(context.Background(), m))
	metrics := sink1.AllMetrics()
	require.Len(t, metrics, 1, "metric should be routed to non default exporter")
	require.Equal(t, 1, metrics[0].ResourceMetrics().Len())
	attrs := metrics[0].ResourceMetrics().At(0).Resource().Attributes()
	_, ok := attrs.Get("X-Tenant")
	assert.False(t, ok, "routing attribute should have been dropped")
	v, ok := attrs.Get("attr")
	assert.True(t, ok, "non routing attributes shouldn't be dropped")
	assert.Equal(t, "acme", v.Str())
	require.Len(t, sink0.AllMetrics(), 0,
		"metrics should not be routed to default pipeline",
	)
}

func TestMetricsConnectorCapabilities(t *testing.T) {
	metricsDefault := component.NewIDWithName(component.DataTypeMetrics, "default")
	metricsOther := component.NewIDWithName(component.DataTypeMetrics, "other")

	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where attributes["X-Tenant"] == "acme"`,
			Pipelines: []component.ID{metricsOther},
		}},
	}

	router := connector.NewMetricsRouter(map[component.ID]consumer.Metrics{
		metricsDefault: consumertest.NewNop(),
		metricsOther:   consumertest.NewNop(),
	})

	factory := NewFactory()
	conn, err := factory.CreateMetricsToMetrics(
		context.Background(),
		connectortest.NewNopCreateSettings(),
		cfg,
		router.(consumer.Metrics),
	)

	require.NoError(t, err)
	assert.Equal(t, false, conn.Capabilities().MutatesData)
}
