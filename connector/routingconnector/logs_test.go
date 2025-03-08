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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/plogutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

func TestLogsRegisterConsumersForValidRoute(t *testing.T) {
	logsDefault := pipeline.NewIDWithName(pipeline.SignalLogs, "default")
	logs0 := pipeline.NewIDWithName(pipeline.SignalLogs, "0")
	logs1 := pipeline.NewIDWithName(pipeline.SignalLogs, "1")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{logsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{logs0},
			},
			{
				Condition: `attributes["X-Tenant"] == "*"`,
				Pipelines: []pipeline.ID{logs0, logs1},
			},
		},
	}

	require.NoError(t, cfg.Validate())

	var defaultSink, sink0, sink1 consumertest.LogsSink

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsDefault: &defaultSink,
		logs0:       &sink0,
		logs1:       &sink1,
	})

	conn, err := NewFactory().CreateLogsToLogs(context.Background(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Logs))

	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.True(t, conn.Capabilities().MutatesData)

	rtConn := conn.(*logsConnector)
	require.NoError(t, err)
	require.Same(t, &defaultSink, rtConn.router.defaultConsumer)

	route, ok := rtConn.router.routes[rtConn.router.table[0].Statement]
	assert.True(t, ok)
	require.Same(t, &sink0, route.consumer)

	route, ok = rtConn.router.routes[rtConn.router.table[1].Statement]
	assert.True(t, ok)

	routeConsumer, err := router.Consumer(logs0, logs1)
	require.NoError(t, err)
	require.Equal(t, routeConsumer, route.consumer)

	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()
}

func TestLogsAreCorrectlySplitPerResourceAttributeWithOTTL(t *testing.T) {
	logsDefault := pipeline.NewIDWithName(pipeline.SignalLogs, "default")
	logs0 := pipeline.NewIDWithName(pipeline.SignalLogs, "0")
	logs1 := pipeline.NewIDWithName(pipeline.SignalLogs, "1")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{logsDefault},
		Table: []RoutingTableItem{
			{
				Condition: `IsMatch(attributes["X-Tenant"], ".*acme") == true`,
				Pipelines: []pipeline.ID{logs0},
			},
			{
				Statement: `route() where IsMatch(attributes["X-Tenant"], "_acme") == true`,
				Pipelines: []pipeline.ID{logs1},
			},
			{
				Statement: `route() where attributes["X-Tenant"] == "ecorp"`,
				Pipelines: []pipeline.ID{logsDefault, logs0},
			},
		},
	}

	var defaultSink, sink0, sink1 consumertest.LogsSink

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsDefault: &defaultSink,
		logs0:       &sink0,
		logs1:       &sink1,
	})

	resetSinks := func() {
		defaultSink.Reset()
		sink0.Reset()
		sink1.Reset()
	}

	factory := NewFactory()
	conn, err := factory.CreateLogsToLogs(
		context.Background(),
		connectortest.NewNopSettings(metadata.Type),
		cfg,
		router.(consumer.Logs),
	)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	t.Run("logs matched by no expressions", func(t *testing.T) {
		resetSinks()

		l := plog.NewLogs()
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "something-else")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, conn.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultSink.AllLogs(), 1)
		assert.Empty(t, sink0.AllLogs())
		assert.Empty(t, sink1.AllLogs())
	})

	t.Run("logs matched one expression", func(t *testing.T) {
		resetSinks()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "xacme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, conn.ConsumeLogs(context.Background(), l))

		assert.Empty(t, defaultSink.AllLogs())
		assert.Len(t, sink0.AllLogs(), 1)
		assert.Empty(t, sink1.AllLogs())
	})

	t.Run("logs matched by one expression, multiple pipelines", func(t *testing.T) {
		resetSinks()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "ecorp")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, conn.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultSink.AllLogs(), 1)
		assert.Len(t, sink0.AllLogs(), 1)
		assert.Empty(t, sink1.AllLogs())

		assert.Equal(t, 1, defaultSink.AllLogs()[0].LogRecordCount())
		assert.Equal(t, 1, sink0.AllLogs()[0].LogRecordCount())
		assert.Equal(t, defaultSink.AllLogs(), sink0.AllLogs())
	})
}

func TestLogsAreCorrectlyMatchOnceWithOTTL(t *testing.T) {
	logsDefault := pipeline.NewIDWithName(pipeline.SignalLogs, "default")
	logs0 := pipeline.NewIDWithName(pipeline.SignalLogs, "0")
	logs1 := pipeline.NewIDWithName(pipeline.SignalLogs, "1")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{logsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where IsMatch(attributes["X-Tenant"], ".*acme") == true`,
				Pipelines: []pipeline.ID{logs0},
			},
			{
				Statement: `route() where IsMatch(attributes["X-Tenant"], "_acme") == true`,
				Pipelines: []pipeline.ID{logs1},
			},
			{
				Condition: `attributes["X-Tenant"] == "ecorp"`,
				Pipelines: []pipeline.ID{logsDefault, logs0},
			},
		},
	}

	var defaultSink, sink0, sink1 consumertest.LogsSink

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsDefault: &defaultSink,
		logs0:       &sink0,
		logs1:       &sink1,
	})

	resetSinks := func() {
		defaultSink.Reset()
		sink0.Reset()
		sink1.Reset()
	}

	factory := NewFactory()
	conn, err := factory.CreateLogsToLogs(
		context.Background(),
		connectortest.NewNopSettings(metadata.Type),
		cfg,
		router.(consumer.Logs),
	)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	t.Run("logs matched by no expressions", func(t *testing.T) {
		resetSinks()

		l := plog.NewLogs()
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "something-else")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, conn.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultSink.AllLogs(), 1)
		assert.Empty(t, sink0.AllLogs())
		assert.Empty(t, sink1.AllLogs())
	})

	t.Run("logs matched one expression", func(t *testing.T) {
		resetSinks()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "xacme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, conn.ConsumeLogs(context.Background(), l))

		assert.Empty(t, defaultSink.AllLogs())
		assert.Len(t, sink0.AllLogs(), 1)
		assert.Empty(t, sink1.AllLogs())
	})

	t.Run("logs matched by two expressions, but sinks to one", func(t *testing.T) {
		resetSinks()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "x_acme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		rl = l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "_acme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, conn.ConsumeLogs(context.Background(), l))

		assert.Empty(t, defaultSink.AllLogs())
		assert.Len(t, sink0.AllLogs(), 1)
		assert.Empty(t, sink1.AllLogs())

		assert.Equal(t, 2, sink0.AllLogs()[0].LogRecordCount())
	})

	t.Run("one log matched by multiple expressions, other matched none", func(t *testing.T) {
		resetSinks()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "_acme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		rl = l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "something-else")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, conn.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultSink.AllLogs(), 1)
		assert.Len(t, sink0.AllLogs(), 1)
		assert.Empty(t, sink1.AllLogs())

		rlog := defaultSink.AllLogs()[0].ResourceLogs().At(0)
		attr, ok := rlog.Resource().Attributes().Get("X-Tenant")
		assert.True(t, ok, "routing attribute must exists")
		assert.Equal(t, "something-else", attr.AsString())
	})

	t.Run("logs matched by one expression, multiple pipelines", func(t *testing.T) {
		resetSinks()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "ecorp")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, conn.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultSink.AllLogs(), 1)
		assert.Len(t, sink0.AllLogs(), 1)
		assert.Empty(t, sink1.AllLogs())

		assert.Equal(t, 1, defaultSink.AllLogs()[0].LogRecordCount())
		assert.Equal(t, 1, sink0.AllLogs()[0].LogRecordCount())
		assert.Equal(t, defaultSink.AllLogs(), sink0.AllLogs())
	})
}

func TestLogsResourceAttributeDroppedByOTTL(t *testing.T) {
	logsDefault := pipeline.NewIDWithName(pipeline.SignalLogs, "default")
	logsOther := pipeline.NewIDWithName(pipeline.SignalLogs, "other")

	cfg := &Config{
		DefaultPipelines: []pipeline.ID{logsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `delete_key(attributes, "X-Tenant") where attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{logsOther},
			},
		},
	}

	var sink0, sink1 consumertest.LogsSink

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsDefault: &sink0,
		logsOther:   &sink1,
	})

	factory := NewFactory()
	conn, err := factory.CreateLogsToLogs(
		context.Background(),
		connectortest.NewNopSettings(metadata.Type),
		cfg,
		router.(consumer.Logs),
	)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	l := plog.NewLogs()
	rm := l.ResourceLogs().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "acme")
	rm.Resource().Attributes().PutStr("attr", "acme")

	assert.NoError(t, conn.ConsumeLogs(context.Background(), l))
	logs := sink1.AllLogs()
	require.Len(t, logs, 1, "log should be routed to non-default exporter")
	require.Equal(t, 1, logs[0].ResourceLogs().Len())
	attrs := logs[0].ResourceLogs().At(0).Resource().Attributes()
	_, ok := attrs.Get("X-Tenant")
	assert.False(t, ok, "routing attribute should have been dropped")
	v, ok := attrs.Get("attr")
	assert.True(t, ok, "non routing attributes shouldn't be dropped")
	assert.Equal(t, "acme", v.Str())
	assert.Empty(t, sink0.AllLogs(),
		"metrics should not be routed to default pipeline",
	)
}

func TestLogsConnectorCapabilities(t *testing.T) {
	logsDefault := pipeline.NewIDWithName(pipeline.SignalLogs, "default")
	logsOther := pipeline.NewIDWithName(pipeline.SignalLogs, "other")

	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where attributes["X-Tenant"] == "acme"`,
			Pipelines: []pipeline.ID{logsOther},
		}},
	}

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsDefault: consumertest.NewNop(),
		logsOther:   consumertest.NewNop(),
	})

	factory := NewFactory()
	conn, err := factory.CreateLogsToLogs(
		context.Background(),
		connectortest.NewNopSettings(metadata.Type),
		cfg,
		router.(consumer.Logs),
	)

	require.NoError(t, err)
	assert.True(t, conn.Capabilities().MutatesData)
}

func TestLogsConnectorDetailed(t *testing.T) {
	idSink0 := pipeline.NewIDWithName(pipeline.SignalLogs, "0")
	idSink1 := pipeline.NewIDWithName(pipeline.SignalLogs, "1")
	idSinkD := pipeline.NewIDWithName(pipeline.SignalLogs, "default")

	isAcme := `request["X-Tenant"] == "acme"`

	isResourceA := `attributes["resourceName"] == "resourceA"`
	isResourceB := `attributes["resourceName"] == "resourceB"`
	isResourceX := `attributes["resourceName"] == "resourceX"`
	isResourceY := `attributes["resourceName"] == "resourceY"`

	isLogE := `body == "logE"`
	isLogF := `body == "logF"`
	isLogX := `body == "logX"`
	isLogY := `body == "logY"`

	// IsMap and IsString are just candidate for Standard Converter Function to prevent any unknown regressions for this component
	isBodyString := `IsString(body) == true`
	require.Contains(t, common.Functions[ottllog.TransformContext](), "IsString")
	isBodyMap := `IsMap(body) == true`
	require.Contains(t, common.Functions[ottllog.TransformContext](), "IsMap")

	isScopeCFromLowerContext := `instrumentation_scope.name == "scopeC"`
	isScopeDFromLowerContext := `instrumentation_scope.name == "scopeD"`

	isResourceBFromLowerContext := `resource.attributes["resourceName"] == "resourceB"`

	testCases := []struct {
		ctx         context.Context
		input       plog.Logs
		expectSink0 plog.Logs
		expectSink1 plog.Logs
		expectSinkD plog.Logs
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
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plog.Logs{},
			expectSinkD: plogutiltest.NewLogs("AB", "CD", "EF"),
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
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plog.Logs{},
		},
		{
			name: "request/match_grpc_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "acme"}),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plog.Logs{},
		},
		{
			name: "request/match_no_grpc_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "notacme"}),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plog.Logs{},
			expectSinkD: plogutiltest.NewLogs("AB", "CD", "EF"),
		},
		{
			name: "request/match_http_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"acme"}}),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plog.Logs{},
		},
		{
			name: "request/match_http_value2",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"notacme", "acme"}}),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plog.Logs{},
		},
		{
			name: "request/match_no_http_value",
			cfg: testConfig(
				withRoute("request", isAcme, idSink0),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"notacme"}}),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plog.Logs{},
			expectSinkD: plogutiltest.NewLogs("AB", "CD", "EF"),
		},
		{
			name: "resource/all_match_first_only",
			cfg: testConfig(
				withRoute("resource", "true", idSink0),
				withRoute("resource", isResourceY, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plog.Logs{},
		},
		{
			name: "resource/all_match_last_only",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", "true", idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "resource/all_match_only_once",
			cfg: testConfig(
				withRoute("resource", "true", idSink0),
				withRoute("resource", isResourceA+" or "+isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plog.Logs{},
		},
		{
			name: "resource/each_matches_one",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("resource", isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("A", "CD", "EF"),
			expectSink1: plogutiltest.NewLogs("B", "CD", "EF"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "resource/some_match_with_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plogutiltest.NewLogs("B", "CD", "EF"),
			expectSinkD: plogutiltest.NewLogs("A", "CD", "EF"),
		},
		{
			name: "resource/some_match_without_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceB, idSink1),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plogutiltest.NewLogs("B", "CD", "EF"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "resource/match_none_with_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceY, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plog.Logs{},
			expectSinkD: plogutiltest.NewLogs("AB", "CD", "EF"),
		},
		{
			name: "resource/match_none_without_default",
			cfg: testConfig(
				withRoute("resource", isResourceX, idSink0),
				withRoute("resource", isResourceY, idSink1),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plog.Logs{},
			expectSinkD: plog.Logs{},
		},
		{
			name: "log/all_match_first_only",
			cfg: testConfig(
				withRoute("log", "true", idSink0),
				withRoute("log", isLogY, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plog.Logs{},
		},
		{
			name: "log/all_match_last_only",
			cfg: testConfig(
				withRoute("log", isLogX, idSink0),
				withRoute("log", "true", idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "log/all_match_only_once",
			cfg: testConfig(
				withRoute("log", "true", idSink0),
				withRoute("log", isLogE+" or "+isLogF, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plog.Logs{},
		},
		{
			name: "log/each_matches_one",
			cfg: testConfig(
				withRoute("log", isLogE, idSink0),
				withRoute("log", isLogF, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "E"),
			expectSink1: plogutiltest.NewLogs("AB", "CD", "F"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "log/some_match_with_default",
			cfg: testConfig(
				withRoute("log", isLogX, idSink0),
				withRoute("log", isLogF, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plogutiltest.NewLogs("AB", "CD", "F"),
			expectSinkD: plogutiltest.NewLogs("AB", "CD", "E"),
		},
		{
			name: "log/some_match_without_default",
			cfg: testConfig(
				withRoute("log", isLogX, idSink0),
				withRoute("log", isLogF, idSink1),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plogutiltest.NewLogs("AB", "CD", "F"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "log/match_none_with_default",
			cfg: testConfig(
				withRoute("log", isLogX, idSink0),
				withRoute("log", isLogY, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plog.Logs{},
			expectSinkD: plogutiltest.NewLogs("AB", "CD", "EF"),
		},
		{
			name: "log/match_none_without_default",
			cfg: testConfig(
				withRoute("log", isLogX, idSink0),
				withRoute("log", isLogY, idSink1),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plog.Logs{},
			expectSink1: plog.Logs{},
			expectSinkD: plog.Logs{},
		},
		{
			name: "log/with_resource_condition",
			cfg: testConfig(
				withRoute("log", isResourceBFromLowerContext, idSink0),
				withRoute("log", isLogY, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("B", "CD", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plogutiltest.NewLogs("A", "CD", "EF"),
		},
		{
			name: "log/with_scope_condition",
			cfg: testConfig(
				withRoute("log", isScopeCFromLowerContext, idSink0),
				withRoute("log", isLogY, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "C", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plogutiltest.NewLogs("AB", "D", "EF"),
		},
		{
			name: "log/with_resource_and_scope_conditions",
			cfg: testConfig(
				withRoute("log", isResourceBFromLowerContext+" and "+isScopeDFromLowerContext, idSink0),
				withRoute("log", isLogY, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("B", "D", "EF"),
			expectSink1: plog.Logs{},
			expectSinkD: plogutiltest.NewLogsFromOpts(
				plogutiltest.Resource("A",
					plogutiltest.Scope("C", plogutiltest.LogRecord("E"), plogutiltest.LogRecord("F")),
					plogutiltest.Scope("D", plogutiltest.LogRecord("E"), plogutiltest.LogRecord("F")),
				),
				plogutiltest.Resource("B",
					plogutiltest.Scope("C", plogutiltest.LogRecord("E"), plogutiltest.LogRecord("F")),
				),
			),
		},
		{
			name: "mixed/match_resource_then_logs",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("log", isLogE, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("A", "CD", "EF"),
			expectSink1: plogutiltest.NewLogs("B", "CD", "E"),
			expectSinkD: plogutiltest.NewLogs("B", "CD", "F"),
		},
		{
			name: "mixed/match_logs_then_resource",
			cfg: testConfig(
				withRoute("log", isLogE, idSink0),
				withRoute("resource", isResourceB, idSink1),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "E"),
			expectSink1: plogutiltest.NewLogs("B", "CD", "F"),
			expectSinkD: plogutiltest.NewLogs("A", "CD", "F"),
		},
		{
			name: "mixed/match_resource_then_grpc_request",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "acme"}),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("A", "CD", "EF"),
			expectSink1: plogutiltest.NewLogs("B", "CD", "EF"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "mixed/match_logs_then_grpc_request",
			cfg: testConfig(
				withRoute("log", isLogF, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withGRPCMetadata(context.Background(), map[string]string{"X-Tenant": "acme"}),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "F"),
			expectSink1: plogutiltest.NewLogs("AB", "CD", "E"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "mixed/match_resource_then_http_request",
			cfg: testConfig(
				withRoute("resource", isResourceA, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"acme"}}),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("A", "CD", "EF"),
			expectSink1: plogutiltest.NewLogs("B", "CD", "EF"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "mixed/match_logs_then_http_request",
			cfg: testConfig(
				withRoute("log", isLogF, idSink0),
				withRoute("request", isAcme, idSink1),
				withDefault(idSinkD),
			),
			ctx:         withHTTPMetadata(context.Background(), map[string][]string{"X-Tenant": {"acme"}}),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "F"),
			expectSink1: plogutiltest.NewLogs("AB", "CD", "E"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "log/with_converter_function_is_string",
			cfg: testConfig(
				withRoute("log", isBodyString, idSink0),
				withDefault(idSinkD),
			),
			input:       plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSink0: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectSinkD: plog.Logs{},
		},
		{
			name: "log/with_converter_function_is_map",
			cfg: testConfig(
				withRoute("log", isBodyMap, idSink0),
				withDefault(idSinkD),
			),
			input: plogutiltest.NewLogsFromOpts(
				plogutiltest.Resource("A", plogutiltest.Scope("B", setLogRecordMap(plogutiltest.LogRecord("C"), "key", "value"))),
			),
			expectSink0: plogutiltest.NewLogsFromOpts(
				plogutiltest.Resource("A", plogutiltest.Scope("B", setLogRecordMap(plogutiltest.LogRecord("C"), "key", "value"))),
			),
			expectSinkD: plog.Logs{},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var sinkD, sink0, sink1 consumertest.LogsSink
			router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
				pipeline.NewIDWithName(pipeline.SignalLogs, "0"):       &sink0,
				pipeline.NewIDWithName(pipeline.SignalLogs, "1"):       &sink1,
				pipeline.NewIDWithName(pipeline.SignalLogs, "default"): &sinkD,
			})

			conn, err := NewFactory().CreateLogsToLogs(
				context.Background(),
				connectortest.NewNopSettings(metadata.Type),
				tt.cfg,
				router.(consumer.Logs),
			)
			require.NoError(t, err)

			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx
			}

			require.NoError(t, conn.ConsumeLogs(ctx, tt.input))

			assertExpected := func(sink *consumertest.LogsSink, expected plog.Logs, name string) {
				if expected == (plog.Logs{}) {
					assert.Empty(t, sink.AllLogs(), name)
				} else {
					require.Len(t, sink.AllLogs(), 1, name)
					assert.Equal(t, expected, sink.AllLogs()[0], name)
				}
			}
			assertExpected(&sink0, tt.expectSink0, "sink0")
			assertExpected(&sink1, tt.expectSink1, "sink1")
			assertExpected(&sinkD, tt.expectSinkD, "sinkD")
		})
	}
}

func setLogRecordMap(lr plog.LogRecord, key, value string) plog.LogRecord {
	lr.Body().SetEmptyMap().PutStr(key, value)
	return lr
}
