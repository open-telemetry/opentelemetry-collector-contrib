// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pmetricutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
)

func TestMetricsRegisterConsumersForValidRoute(t *testing.T) {
	metricsDefault := pipeline.NewIDWithName(pipeline.SignalMetrics, "default")
	metrics0 := pipeline.NewIDWithName(pipeline.SignalMetrics, "0")
	metrics1 := pipeline.NewIDWithName(pipeline.SignalMetrics, "1")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{metricsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{metrics0},
			},
			{
				Condition: `attributes["X-Tenant"] == "*"`,
				Pipelines: []pipeline.ID{metrics0, metrics1},
			},
		},
	}

	require.NoError(t, cfg.Validate())

	var defaultSink, sink0, sink1 consumertest.MetricsSink

	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
		metricsDefault: &defaultSink,
		metrics0:       &sink0,
		metrics1:       &sink1,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(context.Background(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Metrics))

	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.True(t, conn.Capabilities().MutatesData)

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
	metricsDefault := pipeline.NewIDWithName(pipeline.SignalMetrics, "default")
	metrics0 := pipeline.NewIDWithName(pipeline.SignalMetrics, "0")
	metrics1 := pipeline.NewIDWithName(pipeline.SignalMetrics, "1")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{metricsDefault},
		Table: []RoutingTableItem{
			{
				Condition: `attributes["value"] > 2.5`,
				Pipelines: []pipeline.ID{metrics0},
			},
			{
				Statement: `route() where attributes["value"] > 3.0`,
				Pipelines: []pipeline.ID{metrics1},
			},
			{
				Statement: `route() where attributes["value"] == 1.0`,
				Pipelines: []pipeline.ID{metricsDefault, metrics0},
			},
		},
	}

	var defaultSink, sink0, sink1 consumertest.MetricsSink

	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
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
		connectortest.NewNopSettings(metadata.Type),
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
		assert.Empty(t, sink0.AllMetrics())
		assert.Empty(t, sink1.AllMetrics())
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

		assert.Empty(t, defaultSink.AllMetrics())
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Empty(t, sink1.AllMetrics())
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
		assert.Empty(t, sink1.AllMetrics())

		assert.Equal(t, 1, defaultSink.AllMetrics()[0].MetricCount())
		assert.Equal(t, 1, sink0.AllMetrics()[0].MetricCount())
		assert.Equal(t, defaultSink.AllMetrics(), sink0.AllMetrics())
	})
}

func TestMetricsAreCorrectlyMatchOnceWithOTTL(t *testing.T) {
	metricsDefault := pipeline.NewIDWithName(pipeline.SignalMetrics, "default")
	metrics0 := pipeline.NewIDWithName(pipeline.SignalMetrics, "0")
	metrics1 := pipeline.NewIDWithName(pipeline.SignalMetrics, "1")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{metricsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where attributes["value"] > 2.5`,
				Pipelines: []pipeline.ID{metrics0},
			},
			{
				Statement: `route() where attributes["value"] > 3.0`,
				Pipelines: []pipeline.ID{metrics1},
			},
			{
				Condition: `attributes["value"] == 1.0`,
				Pipelines: []pipeline.ID{metricsDefault, metrics0},
			},
		},
	}

	var defaultSink, sink0, sink1 consumertest.MetricsSink

	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
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
		connectortest.NewNopSettings(metadata.Type),
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
		assert.Empty(t, sink0.AllMetrics())
		assert.Empty(t, sink1.AllMetrics())
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

		assert.Empty(t, defaultSink.AllMetrics())
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Empty(t, sink1.AllMetrics())
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

		assert.Empty(t, defaultSink.AllMetrics())
		assert.Len(t, sink0.AllMetrics(), 1)
		assert.Empty(t, sink1.AllMetrics())

		assert.Equal(t, 2, sink0.AllMetrics()[0].MetricCount())
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
		assert.Empty(t, sink1.AllMetrics())

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
		assert.Empty(t, sink1.AllMetrics())

		assert.Equal(t, 1, defaultSink.AllMetrics()[0].MetricCount())
		assert.Equal(t, 1, sink0.AllMetrics()[0].MetricCount())
		assert.Equal(t, defaultSink.AllMetrics(), sink0.AllMetrics())
	})
}

func TestMetricsResourceAttributeDroppedByOTTL(t *testing.T) {
	metricsDefault := pipeline.NewIDWithName(pipeline.SignalMetrics, "default")
	metricsOther := pipeline.NewIDWithName(pipeline.SignalMetrics, "other")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{metricsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `delete_key(attributes, "X-Tenant") where attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{metricsOther},
			},
		},
	}

	var sink0, sink1 consumertest.MetricsSink

	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
		metricsDefault: &sink0,
		metricsOther:   &sink1,
	})

	factory := NewFactory()
	conn, err := factory.CreateMetricsToMetrics(
		context.Background(),
		connectortest.NewNopSettings(metadata.Type),
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
	require.Empty(t, sink0.AllMetrics(),
		"metrics should not be routed to default pipeline",
	)
}

func TestMetricsConnectorCapabilities(t *testing.T) {
	metricsDefault := pipeline.NewIDWithName(pipeline.SignalMetrics, "default")
	metricsOther := pipeline.NewIDWithName(pipeline.SignalMetrics, "other")

	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where attributes["X-Tenant"] == "acme"`,
			Pipelines: []pipeline.ID{metricsOther},
		}},
	}

	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
		metricsDefault: consumertest.NewNop(),
		metricsOther:   consumertest.NewNop(),
	})

	factory := NewFactory()
	conn, err := factory.CreateMetricsToMetrics(
		context.Background(),
		connectortest.NewNopSettings(metadata.Type),
		cfg,
		router.(consumer.Metrics),
	)

	require.NoError(t, err)
	assert.True(t, conn.Capabilities().MutatesData)
}

func TestMetricsConnectorDetailed(t *testing.T) {
	idSink0 := pipeline.NewIDWithName(pipeline.SignalMetrics, "0")
	idSink1 := pipeline.NewIDWithName(pipeline.SignalMetrics, "1")
	idSinkD := pipeline.NewIDWithName(pipeline.SignalMetrics, "default")

	isAcme := `request["X-Tenant"] == "acme"`

	isResourceA := `attributes["resourceName"] == "resourceA"`
	isResourceB := `attributes["resourceName"] == "resourceB"`
	isResourceX := `attributes["resourceName"] == "resourceX"`
	isResourceY := `attributes["resourceName"] == "resourceY"`

	// IsMap and IsString are just candidate for Standard Converter Function to prevent any unknown regressions for this component
	isResourceString := `IsString(attributes["resourceName"]) == true`
	require.Contains(t, common.Functions[ottlresource.TransformContext](), "IsString")
	isAttributesMap := `IsMap(attributes) == true`
	require.Contains(t, common.Functions[ottlresource.TransformContext](), "IsMap")

	isMetricE := `name == "metricE"`
	isMetricF := `name == "metricF"`
	isMetricX := `name == "metricX"`
	isMetricY := `name == "metricY"`

	isDataPointG := `attributes["dpName"] == "dpG"`
	isDataPointH := `attributes["dpName"] == "dpH"`
	isDataPointX := `attributes["dpName"] == "dpX"`
	isDataPointY := `attributes["dpName"] == "dpY"`

	isMetricFFromLowerContext := `metric.name == "metricF"`
	isScopeDFromLowerContext := `instrumentation_scope.name == "scopeD"`
	isResourceBFromLowerContext := `resource.attributes["resourceName"] == "resourceB"`

	testCases := []struct {
		ctx         context.Context
		input       pmetric.Metrics
		expectSink0 pmetric.Metrics
		expectSink1 pmetric.Metrics
		expectSinkD pmetric.Metrics
		cfg         *Config
		name        string
	}{
		{
			name: "request/no_request_values",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         context.Background(),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
		},
		{
			name: "request/match_any_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx: withGRPCMetadata(
				withHTTPMetadata(
					context.Background(),
					map[string][]string{"X-Tenant": {"acme"}},
				),
				map[string]string{"X-Tenant": "notacme"},
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "request/match_grpc_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "acme"}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "request/match_no_grpc_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "notacme"}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
		},
		{
			name: "request/match_http_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"acme"}}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "request/match_http_value2",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"notacme", "acme"}}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "request/match_no_http_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"notacme"}}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
		},
		{
			name: "resource/all_match_first_only",
			cfg: testConfig(
				withRoute("resource", "true", idSink0),
				withRoute("resource", isResourceY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "resource/all_match_last_only",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", "true", idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "resource/all_match_only_once",
			cfg: testConfig(
				withRoute("resource", "true", idSink0),
				withRoute("resource", isResourceA+" or "+isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "resource/each_matches_one",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("resource", isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("A", "CD", "EF", "GH"),
			expectSink1: pmetricutiltest.NewGauges("B", "CD", "EF", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "resource/some_match_with_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetricutiltest.NewGauges("B", "CD", "EF", "GH"),
			expectSinkD: pmetricutiltest.NewGauges("A", "CD", "EF", "GH"),
		},
		{
			name: "resource/some_match_without_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceB, idSink1),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetricutiltest.NewGauges("B", "CD", "EF", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "resource/match_none_with_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
		},
		{
			name: "resource/match_none_without_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceY, idSink1),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "resource/with_converter_function_is_string",
			cfg: testConfig(
				withRoute("resource", isResourceString, idSink0),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "resource/with_converter_function_is_map",
			cfg: testConfig(
				withRoute("resource", isAttributesMap, idSink0),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "metric/all_match_first_only",
			cfg: testConfig(
				withRoute("metric", "true", idSink0),
				withRoute("metric", isMetricY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "metric/all_match_last_only",
			cfg: testConfig(
				withRoute("metric", isMetricX, idSink0),
				withRoute("metric", "true", idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "metric/all_match_only_once",
			cfg: testConfig(
				withRoute("metric", "true", idSink0),
				withRoute("metric", isMetricE+" or "+isMetricF, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "metric/each_matches_one",
			cfg: testConfig(
				withRoute("metric", isMetricE, idSink0),
				withRoute("metric", isMetricF, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "E", "GH"),
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "F", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "metric/some_match_with_default",
			cfg: testConfig(
				withRoute("metric", isMetricX, idSink0),
				withRoute("metric", isMetricF, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "F", "GH"),
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "E", "GH"),
		},
		{
			name: "metric/some_match_without_default",
			cfg: testConfig(
				withRoute("metric", isMetricX, idSink0),
				withRoute("metric", isMetricF, idSink1),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "F", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "metric/match_none_with_default",
			cfg: testConfig(
				withRoute("metric", isMetricX, idSink0),
				withRoute("metric", isMetricY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
		},
		{
			name: "metric/match_none_without_default",
			cfg: testConfig(
				withRoute("metric", isMetricX, idSink0),
				withRoute("metric", isMetricY, idSink1),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "metric/with_resource_condition",
			cfg: testConfig(
				withRoute("metric", isResourceBFromLowerContext, idSink0),
				withRoute("metric", isMetricY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("B", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("A", "CD", "EF", "GH"),
		},
		{
			name: "metric/with_scope_condition",
			cfg: testConfig(
				withRoute("metric", isScopeDFromLowerContext, idSink0),
				withRoute("metric", isMetricY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "D", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("AB", "C", "EF", "GH"),
		},
		{
			name: "metric/with_resource_and_scope_conditions",
			cfg: testConfig(
				withRoute("metric", isResourceBFromLowerContext+" and "+isScopeDFromLowerContext, idSink0),
				withRoute("metric", isMetricY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("B", "D", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
		},
		{
			name: "datapoint/all_match_first_only",
			cfg: testConfig(
				withRoute("datapoint", "true", idSink0),
				withRoute("datapoint", isDataPointY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "datapoint/all_match_last_only",
			cfg: testConfig(
				withRoute("datapoint", isDataPointX, idSink0),
				withRoute("datapoint", "true", idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "datapoint/all_match_only_once",
			cfg: testConfig(
				withRoute("datapoint", "true", idSink0),
				withRoute("datapoint", isDataPointG+" or "+isDataPointH, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "datapoint/each_matches_one",
			cfg: testConfig(
				withRoute("datapoint", isDataPointG, idSink0),
				withRoute("datapoint", isDataPointH, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "G"),
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "EF", "H"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "datapoint/some_match_with_default",
			cfg: testConfig(
				withRoute("datapoint", isDataPointX, idSink0),
				withRoute("datapoint", isDataPointH, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "EF", "H"),
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "EF", "G"),
		},
		{
			name: "datapoint/some_match_without_default",
			cfg: testConfig(
				withRoute("datapoint", isDataPointX, idSink0),
				withRoute("datapoint", isDataPointH, idSink1),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "EF", "H"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "datapoint/match_none_with_default",
			cfg: testConfig(
				withRoute("datapoint", isDataPointX, idSink0),
				withRoute("datapoint", isDataPointY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
		},
		{
			name: "datapoint/match_none_without_default",
			cfg: testConfig(
				withRoute("datapoint", isDataPointX, idSink0),
				withRoute("datapoint", isDataPointY, idSink1),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetric.Metrics{},
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "datapoint/with_resource_condition",
			cfg: testConfig(
				withRoute("datapoint", isResourceBFromLowerContext, idSink0),
				withRoute("datapoint", isDataPointY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("B", "CD", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("A", "CD", "EF", "GH"),
		},
		{
			name: "datapoint/with_scope_condition",
			cfg: testConfig(
				withRoute("datapoint", isScopeDFromLowerContext, idSink0),
				withRoute("datapoint", isDataPointY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "D", "EF", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("AB", "C", "EF", "GH"),
		},
		{
			name: "datapoint/with_metric_condition",
			cfg: testConfig(
				withRoute("datapoint", isMetricFFromLowerContext, idSink0),
				withRoute("datapoint", isDataPointY, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "F", "GH"),
			expectSink1: pmetric.Metrics{},
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "E", "GH"),
		},
		{
			name: "mixed/match_resource_then_metrics",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("metric", isMetricE, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("A", "CD", "EF", "GH"),
			expectSink1: pmetricutiltest.NewGauges("B", "CD", "E", "GH"),
			expectSinkD: pmetricutiltest.NewGauges("B", "CD", "F", "GH"),
		},
		{
			name: "mixed/match_metrics_then_resource",
			cfg: testConfig(
				withRoute("metric", isMetricE, idSink0),
				withRoute("resource", isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "E", "GH"),
			expectSink1: pmetricutiltest.NewGauges("B", "CD", "F", "GH"),
			expectSinkD: pmetricutiltest.NewGauges("A", "CD", "F", "GH"),
		},
		{
			name: "mixed/match_resource_then_datapoint",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("datapoint", isDataPointG, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("A", "CD", "EF", "GH"),
			expectSink1: pmetricutiltest.NewGauges("B", "CD", "EF", "G"),
			expectSinkD: pmetricutiltest.NewGauges("B", "CD", "EF", "H"),
		},
		{
			name: "mixed/match_datapoint_then_resource",
			cfg: testConfig(
				withRoute("datapoint", isDataPointG, idSink0),
				withRoute("resource", isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "G"),
			expectSink1: pmetricutiltest.NewGauges("B", "CD", "EF", "H"),
			expectSinkD: pmetricutiltest.NewGauges("A", "CD", "EF", "H"),
		},
		{
			name: "mixed/match_metric_then_datapoint",
			cfg: testConfig(
				withRoute("metric", isMetricE, idSink0),
				withRoute("datapoint", isDataPointG, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "E", "GH"),
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "F", "G"),
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "F", "H"),
		},
		{
			name: "mixed/match_datapoint_then_metric",
			cfg: testConfig(
				withRoute("datapoint", isDataPointG, idSink0),
				withRoute("metric", isMetricE, idSink1),
				withDefault(idSinkD),
			),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "G"),
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "E", "H"),
			expectSinkD: pmetricutiltest.NewGauges("AB", "CD", "F", "H"),
		},
		{
			name: "mixed/match_resource_then_grpc_request",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "acme"}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("A", "CD", "EF", "GH"),
			expectSink1: pmetricutiltest.NewGauges("B", "CD", "EF", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "mixed/match_metrics_then_grpc_request",
			cfg: testConfig(
				withRoute("metric", isMetricF, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "acme"}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "F", "GH"),
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "E", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "mixed/match_datapoint_then_grpc_request",
			cfg: testConfig(
				withRoute("datapoint", isDataPointG, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "acme"}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "G"),
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "EF", "H"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "mixed/match_resource_then_http_request",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"acme"}}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("A", "CD", "EF", "GH"),
			expectSink1: pmetricutiltest.NewGauges("B", "CD", "EF", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "mixed/match_metrics_then_http_request",
			cfg: testConfig(
				withRoute("metric", isMetricF, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"acme"}}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "F", "GH"),
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "E", "GH"),
			expectSinkD: pmetric.Metrics{},
		},
		{
			name: "mixed/match_datapoint_then_http_request",
			cfg: testConfig(
				withRoute("datapoint", isDataPointG, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"acme"}}),
			input:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectSink0: pmetricutiltest.NewGauges("AB", "CD", "EF", "G"),
			expectSink1: pmetricutiltest.NewGauges("AB", "CD", "EF", "H"),
			expectSinkD: pmetric.Metrics{},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var sinkD, sink0, sink1 consumertest.MetricsSink
			router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
				pipeline.NewIDWithName(pipeline.SignalMetrics, "0"):       &sink0,
				pipeline.NewIDWithName(pipeline.SignalMetrics, "1"):       &sink1,
				pipeline.NewIDWithName(pipeline.SignalMetrics, "default"): &sinkD,
			})

			conn, err := NewFactory().CreateMetricsToMetrics(
				context.Background(),
				connectortest.NewNopSettings(metadata.Type),
				tt.cfg,
				router.(consumer.Metrics),
			)
			require.NoError(t, err)

			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx
			}

			require.NoError(t, conn.ConsumeMetrics(ctx, tt.input))

			assertExpected := func(sink *consumertest.MetricsSink, expected pmetric.Metrics, name string) {
				if expected == (pmetric.Metrics{}) {
					assert.Empty(t, sink.AllMetrics(), name)
				} else {
					require.Len(t, sink.AllMetrics(), 1, name)
					assert.Equal(t, expected, sink.AllMetrics()[0], name)
				}
			}
			assertExpected(&sink0, tt.expectSink0, "sink0")
			assertExpected(&sink1, tt.expectSink1, "sink1")
			assertExpected(&sinkD, tt.expectSinkD, "sinkD")
		})
	}
}
