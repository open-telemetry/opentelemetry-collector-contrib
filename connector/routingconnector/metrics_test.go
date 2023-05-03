// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/fanoutconsumer"
)

func TestMetrics_RegisterConsumersForValidRoute(t *testing.T) {
	cfg := &Config{
		DefaultPipelines: []string{"metrics/default"},
		Table: []RoutingTableItem{
			{
				Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
				Pipelines: []string{"metrics/0"},
			},
			{
				Statement: `route() where resource.attributes["X-Tenant"] == "*"`,
				Pipelines: []string{"metrics/0", "metrics/1"},
			},
		},
	}

	require.NoError(t, cfg.Validate())

	defaultSinkID := component.NewIDWithName(component.DataTypeMetrics, "default")
	defaultSink := &consumertest.MetricsSink{}

	sink0ID := component.NewIDWithName(component.DataTypeMetrics, "0")
	sink0 := &consumertest.MetricsSink{}

	sink1ID := component.NewIDWithName(component.DataTypeMetrics, "1")
	sink1 := &consumertest.MetricsSink{}

	router := fanoutconsumer.NewMetricsRouter(
		map[component.ID]consumer.Metrics{
			defaultSinkID: defaultSink,
			sink0ID:       sink0,
			sink1ID:       sink1,
		})

	conn, err := NewFactory().CreateMetricsToMetrics(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router)

	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.False(t, conn.Capabilities().MutatesData)

	rtConn := conn.(*metricsConnector)
	require.NoError(t, err)
	require.Same(t, defaultSink, rtConn.router.defaultConsumer)

	route, ok := rtConn.router.routes[rtConn.router.table[0].Statement]
	assert.True(t, ok)
	require.Same(t, sink0, route.consumer)

	route, ok = rtConn.router.routes[rtConn.router.table[1].Statement]
	assert.True(t, ok)

	routeConsumer, err := router.(connector.MetricsRouter).Consumer(sink0ID, sink1ID)
	require.NoError(t, err)
	require.Equal(t, routeConsumer, route.consumer)

	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()
}

func TestMetricsAreCorrectlySplitPerResourceAttributeWithOTTL(t *testing.T) {
	cfg := &Config{
		DefaultPipelines: []string{"metrics/default"},
		Table: []RoutingTableItem{
			{
				Statement: `route() where resource.attributes["value"] > 2.5`,
				Pipelines: []string{"metrics/0"},
			},
			{
				Statement: `route() where resource.attributes["value"] > 3.0`,
				Pipelines: []string{"metrics/1"},
			},
			{
				Statement: `route() where resource.attributes["value"] == 1.0`,
				Pipelines: []string{"metrics/default", "metrics/0"},
			},
		},
	}

	defaultSink := &consumertest.MetricsSink{}
	sink0 := &consumertest.MetricsSink{}
	sink1 := &consumertest.MetricsSink{}

	resetSinks := func() {
		defaultSink.Reset()
		sink0.Reset()
		sink1.Reset()
	}

	consumer := fanoutconsumer.NewMetricsRouter(
		map[component.ID]consumer.Metrics{
			component.NewIDWithName(component.DataTypeMetrics, "default"): defaultSink,
			component.NewIDWithName(component.DataTypeMetrics, "0"):       sink0,
			component.NewIDWithName(component.DataTypeMetrics, "1"):       sink1,
		})

	factory := NewFactory()
	conn, err := factory.CreateMetricsToMetrics(context.Background(), connectortest.NewNopCreateSettings(), cfg, consumer)

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

func TestMetrics_ResourceAttribute_DroppedByOTTL(t *testing.T) {
	cfg := &Config{
		DefaultPipelines: []string{"metrics/default"},
		Table: []RoutingTableItem{
			{
				Statement: `delete_key(resource.attributes, "X-Tenant") where resource.attributes["X-Tenant"] == "acme"`,
				Pipelines: []string{"metrics/0"},
			},
		},
	}

	sink0 := &consumertest.MetricsSink{}
	sink1 := &consumertest.MetricsSink{}

	consumer := fanoutconsumer.NewMetricsRouter(
		map[component.ID]consumer.Metrics{
			component.NewIDWithName(component.DataTypeMetrics, "default"): sink0,
			component.NewIDWithName(component.DataTypeMetrics, "0"):       sink1,
		})

	factory := NewFactory()
	conn, err := factory.CreateMetricsToMetrics(context.Background(), connectortest.NewNopCreateSettings(), cfg, consumer)

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
	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
			Pipelines: []string{"metrics/0"},
		}},
	}

	consumer := fanoutconsumer.NewMetricsRouter(
		map[component.ID]consumer.Metrics{
			component.NewIDWithName(component.DataTypeMetrics, "default"): &consumertest.MetricsSink{},
			component.NewIDWithName(component.DataTypeMetrics, "0"):       &consumertest.MetricsSink{},
		})

	factory := NewFactory()
	conn, err := factory.CreateMetricsToMetrics(context.Background(), connectortest.NewNopCreateSettings(), cfg, consumer)

	require.NoError(t, err)
	assert.Equal(t, false, conn.Capabilities().MutatesData)
}
