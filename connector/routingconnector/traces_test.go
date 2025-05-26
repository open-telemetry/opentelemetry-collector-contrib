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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
)

func TestTracesRegisterConsumersForValidRoute(t *testing.T) {
	tracesDefault := pipeline.NewIDWithName(pipeline.SignalTraces, "default")
	traces0 := pipeline.NewIDWithName(pipeline.SignalTraces, "0")
	traces1 := pipeline.NewIDWithName(pipeline.SignalTraces, "1")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{tracesDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{traces0},
			},
			{
				Condition: `attributes["X-Tenant"] == "*"`,
				Pipelines: []pipeline.ID{traces0, traces1},
			},
		},
	}

	require.NoError(t, cfg.Validate())

	var defaultSink, sink0, sink1 consumertest.TracesSink

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesDefault: &defaultSink,
		traces0:       &sink0,
		traces1:       &sink1,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.True(t, conn.Capabilities().MutatesData)

	rtConn := conn.(*tracesConnector)
	require.NoError(t, err)
	require.Same(t, &defaultSink, rtConn.router.defaultConsumer)

	route, ok := rtConn.router.routes[rtConn.router.table[0].Statement]
	assert.True(t, ok)
	require.Same(t, &sink0, route.consumer)

	route, ok = rtConn.router.routes[rtConn.router.table[1].Statement]
	assert.True(t, ok)

	routeConsumer, err := router.Consumer(traces0, traces1)
	require.NoError(t, err)
	require.Equal(t, routeConsumer, route.consumer)

	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()
}

func TestTracesCorrectlySplitPerResourceAttributeWithOTTL(t *testing.T) {
	tracesDefault := pipeline.NewIDWithName(pipeline.SignalTraces, "default")
	traces0 := pipeline.NewIDWithName(pipeline.SignalTraces, "0")
	traces1 := pipeline.NewIDWithName(pipeline.SignalTraces, "1")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{tracesDefault},
		Table: []RoutingTableItem{
			{
				Condition: `attributes["value"] > 0 and attributes["value"] < 4`,
				Pipelines: []pipeline.ID{traces0},
			},
			{
				Statement: `route() where attributes["value"] > 1 and attributes["value"] < 4`,
				Pipelines: []pipeline.ID{traces1},
			},
			{
				Statement: `route() where attributes["value"] == 5`,
				Pipelines: []pipeline.ID{tracesDefault, traces0},
			},
		},
	}

	var defaultSink, sink0, sink1 consumertest.TracesSink

	resetSinks := func() {
		defaultSink.Reset()
		sink0.Reset()
		sink1.Reset()
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesDefault: &defaultSink,
		traces1:       &sink1,
		traces0:       &sink0,
	})

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(
		context.Background(),
		connectortest.NewNopSettings(metadata.Type),
		cfg,
		router.(consumer.Traces),
	)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	t.Run("span matched by 0 expressions", func(t *testing.T) {
		resetSinks()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 10)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))

		assert.Len(t, defaultSink.AllTraces(), 1)
		assert.Empty(t, sink0.AllTraces())
		assert.Empty(t, sink1.AllTraces())
	})

	t.Run("span matched by one of two expressions", func(t *testing.T) {
		resetSinks()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 1)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))

		assert.Empty(t, defaultSink.AllTraces())
		assert.Len(t, sink0.AllTraces(), 1)
		assert.Empty(t, sink1.AllTraces())
	})

	t.Run("span matched by one expression, multiple pipelines", func(t *testing.T) {
		resetSinks()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 5)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))

		assert.Len(t, defaultSink.AllTraces(), 1)
		assert.Len(t, sink0.AllTraces(), 1)
		assert.Empty(t, sink1.AllTraces())

		assert.Equal(t, 1, defaultSink.AllTraces()[0].SpanCount())
		assert.Equal(t, 1, sink0.AllTraces()[0].SpanCount())
		assert.Equal(t, defaultSink.AllTraces(), sink0.AllTraces())
	})
}

func TestTracesCorrectlyMatchOnceWithOTTL(t *testing.T) {
	tracesDefault := pipeline.NewIDWithName(pipeline.SignalTraces, "default")
	traces0 := pipeline.NewIDWithName(pipeline.SignalTraces, "0")
	traces1 := pipeline.NewIDWithName(pipeline.SignalTraces, "1")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{tracesDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where attributes["value"] > 0 and attributes["value"] < 4`,
				Pipelines: []pipeline.ID{traces0},
			},
			{
				Statement: `route() where attributes["value"] > 1 and attributes["value"] < 4`,
				Pipelines: []pipeline.ID{traces1},
			},
			{
				Condition: `attributes["value"] == 5`,
				Pipelines: []pipeline.ID{tracesDefault, traces0},
			},
		},
	}

	var defaultSink, sink0, sink1 consumertest.TracesSink

	resetSinks := func() {
		defaultSink.Reset()
		sink0.Reset()
		sink1.Reset()
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesDefault: &defaultSink,
		traces0:       &sink0,
		traces1:       &sink1,
	})

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(
		context.Background(),
		connectortest.NewNopSettings(metadata.Type),
		cfg,
		router.(consumer.Traces),
	)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	t.Run("span matched by 0 expressions", func(t *testing.T) {
		resetSinks()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 10)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))

		assert.Len(t, defaultSink.AllTraces(), 1)
		assert.Empty(t, sink0.AllTraces())
		assert.Empty(t, sink1.AllTraces())
	})

	t.Run("span matched by one of two expressions", func(t *testing.T) {
		resetSinks()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 1)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))

		assert.Empty(t, defaultSink.AllTraces())
		assert.Len(t, sink0.AllTraces(), 1)
		assert.Empty(t, sink1.AllTraces())
	})

	t.Run("span matched by all expressions, but sinks to one", func(t *testing.T) {
		resetSinks()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 2)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		rl = tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 3)
		span = rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span1")

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))

		assert.Empty(t, defaultSink.AllTraces())
		assert.Len(t, sink0.AllTraces(), 1)
		assert.Empty(t, sink1.AllTraces())

		assert.Equal(t, 2, sink0.AllTraces()[0].SpanCount())
	})

	t.Run("span matched by one expression, multiple pipelines", func(t *testing.T) {
		resetSinks()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 5)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))

		assert.Len(t, defaultSink.AllTraces(), 1)
		assert.Len(t, sink0.AllTraces(), 1)
		assert.Empty(t, sink1.AllTraces())

		assert.Equal(t, 1, defaultSink.AllTraces()[0].SpanCount())
		assert.Equal(t, 1, sink0.AllTraces()[0].SpanCount())
		assert.Equal(t, defaultSink.AllTraces(), sink0.AllTraces())
	})
}

func TestTracesResourceAttributeDroppedByOTTL(t *testing.T) {
	tracesDefault := pipeline.NewIDWithName(pipeline.SignalTraces, "default")
	tracesOther := pipeline.NewIDWithName(pipeline.SignalTraces, "other")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{tracesDefault},
		Table: []RoutingTableItem{
			{
				Statement: `delete_key(attributes, "X-Tenant") where attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{tracesOther},
			},
		},
	}

	var sink0, sink1 consumertest.TracesSink

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesDefault: &sink0,
		tracesOther:   &sink1,
	})

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(
		context.Background(),
		connectortest.NewNopSettings(metadata.Type),
		cfg,
		router.(consumer.Traces),
	)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	tr := ptrace.NewTraces()
	rm := tr.ResourceSpans().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "acme")
	rm.Resource().Attributes().PutStr("attr", "acme")

	assert.NoError(t, conn.ConsumeTraces(context.Background(), tr))
	traces := sink1.AllTraces()
	require.Len(t, traces, 1,
		"trace should be routed to non default pipeline",
	)
	require.Equal(t, 1, traces[0].ResourceSpans().Len())
	attrs := traces[0].ResourceSpans().At(0).Resource().Attributes()
	_, ok := attrs.Get("X-Tenant")
	assert.False(t, ok, "routing attribute should have been dropped")
	v, ok := attrs.Get("attr")
	assert.True(t, ok, "non-routing attributes shouldn't have been dropped")
	assert.Equal(t, "acme", v.Str())
	require.Empty(t, sink0.AllTraces(),
		"trace should not be routed to default pipeline",
	)
}

func TestTraceConnectorCapabilities(t *testing.T) {
	tracesDefault := pipeline.NewIDWithName(pipeline.SignalTraces, "default")
	tracesOther := pipeline.NewIDWithName(pipeline.SignalTraces, "0")

	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where attributes["X-Tenant"] == "acme"`,
			Pipelines: []pipeline.ID{tracesOther},
		}},
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesDefault: consumertest.NewNop(),
		tracesOther:   consumertest.NewNop(),
	})

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(
		context.Background(),
		connectortest.NewNopSettings(metadata.Type),
		cfg,
		router.(consumer.Traces),
	)

	require.NoError(t, err)
	assert.True(t, conn.Capabilities().MutatesData)
}

func TestTracesConnectorDetailed(t *testing.T) {
	idSink0 := pipeline.NewIDWithName(pipeline.SignalTraces, "0")
	idSink1 := pipeline.NewIDWithName(pipeline.SignalTraces, "1")
	idSinkD := pipeline.NewIDWithName(pipeline.SignalTraces, "default")

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

	isSpanE := `name == "spanE"`
	isSpanF := `name == "spanF"`
	isSpanX := `name == "spanX"`
	isSpanY := `name == "spanY"`

	isScopeCFromLowerContext := `instrumentation_scope.name == "scopeC"`
	isScopeDFromLowerContext := `instrumentation_scope.name == "scopeD"`

	isResourceBFromLowerContext := `resource.attributes["resourceName"] == "resourceB"`

	testCases := []struct {
		ctx         context.Context
		input       ptrace.Traces
		expectSink0 ptrace.Traces
		expectSink1 ptrace.Traces
		expectSinkD ptrace.Traces
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
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptrace.Traces{},
			expectSinkD: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
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
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "request/match_grpc_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "acme"}),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "request/match_no_grpc_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "notacme"}),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptrace.Traces{},
			expectSinkD: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
		},
		{
			name: "request/match_http_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"acme"}}),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "request/match_http_value2",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"notacme", "acme"}}),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "request/match_no_http_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"notacme"}}),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptrace.Traces{},
			expectSinkD: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
		},
		{
			name: "resource/all_match_first_only",
			cfg: testConfig(
				withRoute("resource", "true", idSink0),
				withRoute("resource", isResourceY, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "resource/all_match_last_only",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", "true", idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "resource/all_match_only_once",
			cfg: testConfig(
				withRoute("resource", "true", idSink0),
				withRoute("resource", isResourceA+" or "+isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "resource/each_matches_one",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("resource", isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSink0: ptraceutiltest.NewTraces("A", "CD", "EF", "FG"),
			expectSink1: ptraceutiltest.NewTraces("B", "CD", "EF", "FG"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "resource/some_match_with_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptraceutiltest.NewTraces("B", "CD", "EF", "FG"),
			expectSinkD: ptraceutiltest.NewTraces("A", "CD", "EF", "FG"),
		},
		{
			name: "resource/some_match_without_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceB, idSink1),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptraceutiltest.NewTraces("B", "CD", "EF", "FG"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "resource/match_none_with_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceY, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptrace.Traces{},
			expectSinkD: ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
		},
		{
			name: "resource/match_none_without_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceY, idSink1),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptrace.Traces{},
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "resource/with_converter_function_is_string",
			cfg: testConfig(
				withRoute("resource", isResourceString, idSink0),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "resource/with_converter_function_is_map",
			cfg: testConfig(
				withRoute("resource", isAttributesMap, idSink0),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "span/all_match_first_only",
			cfg: testConfig(
				withRoute("span", "true", idSink0),
				withRoute("span", isSpanY, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "span/all_match_last_only",
			cfg: testConfig(
				withRoute("span", isSpanX, idSink0),
				withRoute("span", "true", idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "span/all_match_only_once",
			cfg: testConfig(
				withRoute("span", "true", idSink0),
				withRoute("span", isSpanE+" or "+isSpanF, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "span/each_matches_one",
			cfg: testConfig(
				withRoute("span", isSpanE, idSink0),
				withRoute("span", isSpanF, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "E", "GH"),
			expectSink1: ptraceutiltest.NewTraces("AB", "CD", "F", "GH"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "span/some_match_with_default",
			cfg: testConfig(
				withRoute("span", isSpanX, idSink0),
				withRoute("span", isSpanF, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptraceutiltest.NewTraces("AB", "CD", "F", "GH"),
			expectSinkD: ptraceutiltest.NewTraces("AB", "CD", "E", "GH"),
		},
		{
			name: "span/some_match_without_default",
			cfg: testConfig(
				withRoute("span", isSpanX, idSink0),
				withRoute("span", isSpanF, idSink1),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptraceutiltest.NewTraces("AB", "CD", "F", "GH"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "span/match_none_with_default",
			cfg: testConfig(
				withRoute("span", isSpanX, idSink0),
				withRoute("span", isSpanY, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptrace.Traces{},
			expectSinkD: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
		},
		{
			name: "span/match_none_without_default",
			cfg: testConfig(
				withRoute("span", isSpanX, idSink0),
				withRoute("span", isSpanY, idSink1),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptrace.Traces{},
			expectSink1: ptrace.Traces{},
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "span/with_resource_condition",
			cfg: testConfig(
				withRoute("span", isResourceBFromLowerContext, idSink0),
				withRoute("span", isSpanY, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("B", "CD", "EF", "GH"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptraceutiltest.NewTraces("A", "CD", "EF", "GH"),
		},
		{
			name: "span/with_scope_condition",
			cfg: testConfig(
				withRoute("span", isScopeCFromLowerContext, idSink0),
				withRoute("span", isSpanY, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "C", "EF", "GH"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptraceutiltest.NewTraces("AB", "D", "EF", "GH"),
		},
		{
			name: "span/with_resource_and_scope_conditions",
			cfg: testConfig(
				withRoute("span", isResourceBFromLowerContext+" and "+isScopeDFromLowerContext, idSink0),
				withRoute("span", isSpanY, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("B", "D", "EF", "GH"),
			expectSink1: ptrace.Traces{},
			expectSinkD: ptraceutiltest.NewTracesFromOpts(
				ptraceutiltest.Resource("A",
					ptraceutiltest.Scope("C",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
					ptraceutiltest.Scope("D",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
				),
				ptraceutiltest.Resource("B",
					ptraceutiltest.Scope("C",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
				),
			),
		},
		{
			name: "mixed/match_resource_then_metrics",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("span", isSpanE, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("A", "CD", "EF", "GH"),
			expectSink1: ptraceutiltest.NewTraces("B", "CD", "E", "GH"),
			expectSinkD: ptraceutiltest.NewTraces("B", "CD", "F", "GH"),
		},
		{
			name: "mixed/match_metrics_then_resource",
			cfg: testConfig(
				withRoute("span", isSpanE, idSink0),
				withRoute("resource", isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "E", "GH"),
			expectSink1: ptraceutiltest.NewTraces("B", "CD", "F", "GH"),
			expectSinkD: ptraceutiltest.NewTraces("A", "CD", "F", "GH"),
		},

		{
			name: "mixed/match_resource_then_grpc_request",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "acme"}),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("A", "CD", "EF", "GH"),
			expectSink1: ptraceutiltest.NewTraces("B", "CD", "EF", "GH"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "mixed/match_metrics_then_grpc_request",
			cfg: testConfig(
				withRoute("span", isSpanF, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "acme"}),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "F", "GH"),
			expectSink1: ptraceutiltest.NewTraces("AB", "CD", "E", "GH"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "mixed/match_resource_then_http_request",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"acme"}}),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("A", "CD", "EF", "GH"),
			expectSink1: ptraceutiltest.NewTraces("B", "CD", "EF", "GH"),
			expectSinkD: ptrace.Traces{},
		},
		{
			name: "mixed/match_metrics_then_http_request",
			cfg: testConfig(
				withRoute("span", isSpanF, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"acme"}}),
			input:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectSink0: ptraceutiltest.NewTraces("AB", "CD", "F", "GH"),
			expectSink1: ptraceutiltest.NewTraces("AB", "CD", "E", "GH"),
			expectSinkD: ptrace.Traces{},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var sinkD, sink0, sink1 consumertest.TracesSink
			router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
				pipeline.NewIDWithName(pipeline.SignalTraces, "0"):       &sink0,
				pipeline.NewIDWithName(pipeline.SignalTraces, "1"):       &sink1,
				pipeline.NewIDWithName(pipeline.SignalTraces, "default"): &sinkD,
			})

			conn, err := NewFactory().CreateTracesToTraces(
				context.Background(),
				connectortest.NewNopSettings(metadata.Type),
				tt.cfg,
				router.(consumer.Traces),
			)
			require.NoError(t, err)

			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx
			}

			require.NoError(t, conn.ConsumeTraces(ctx, tt.input))

			assertExpected := func(sink *consumertest.TracesSink, expected ptrace.Traces, name string) {
				if expected == (ptrace.Traces{}) {
					assert.Empty(t, sink.AllTraces(), name)
				} else {
					require.Len(t, sink.AllTraces(), 1, name)
					assert.Equal(t, expected, sink.AllTraces()[0], name)
				}
			}
			assertExpected(&sink0, tt.expectSink0, "sink0")
			assertExpected(&sink1, tt.expectSink1, "sink1")
			assertExpected(&sinkD, tt.expectSinkD, "sinkD")
		})
	}
}
