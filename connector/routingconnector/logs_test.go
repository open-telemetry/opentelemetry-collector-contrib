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
		connectortest.NewNopSettings(), cfg, router.(consumer.Logs))

	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.False(t, conn.Capabilities().MutatesData)

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
		connectortest.NewNopSettings(),
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

	t.Run("logs matched by two expressions", func(t *testing.T) {
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
		assert.Len(t, sink1.AllLogs(), 1)

		assert.Equal(t, 2, sink0.AllLogs()[0].LogRecordCount())
		assert.Equal(t, 2, sink1.AllLogs()[0].LogRecordCount())
		assert.Equal(t, sink0.AllLogs(), sink1.AllLogs())
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
		assert.Len(t, sink1.AllLogs(), 1)

		assert.Equal(t, sink0.AllLogs(), sink1.AllLogs())

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
		MatchOnce: true,
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
		connectortest.NewNopSettings(),
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
		connectortest.NewNopSettings(),
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
		connectortest.NewNopSettings(),
		cfg,
		router.(consumer.Logs),
	)

	require.NoError(t, err)
	assert.False(t, conn.Capabilities().MutatesData)
}
