// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
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

func TestLogsConnectorDetailed(t *testing.T) {
	testCases := []string{
		filepath.Join("testdata", "logs", "request_context", "match_any_value"),
		filepath.Join("testdata", "logs", "request_context", "match_grpc_value"),
		filepath.Join("testdata", "logs", "request_context", "match_http_value"),
		filepath.Join("testdata", "logs", "request_context", "match_http_value2"),
		filepath.Join("testdata", "logs", "request_context", "match_no_grpc_value"),
		filepath.Join("testdata", "logs", "request_context", "match_no_http_value"),
		filepath.Join("testdata", "logs", "request_context", "no_request_values"),
		filepath.Join("testdata", "logs", "resource_context", "all_match_first_only"),
		filepath.Join("testdata", "logs", "resource_context", "all_match_last_only"),
		filepath.Join("testdata", "logs", "resource_context", "all_match_once"),
		filepath.Join("testdata", "logs", "resource_context", "each_matches_one"),
		filepath.Join("testdata", "logs", "resource_context", "match_none_with_default"),
		filepath.Join("testdata", "logs", "resource_context", "match_none_without_default"),
		filepath.Join("testdata", "logs", "log_context", "all_match_first_only"),
		filepath.Join("testdata", "logs", "log_context", "all_match_last_only"),
		filepath.Join("testdata", "logs", "log_context", "match_none_with_default"),
		filepath.Join("testdata", "logs", "log_context", "match_none_without_default"),
		filepath.Join("testdata", "logs", "log_context", "some_match_each_route"),
		filepath.Join("testdata", "logs", "log_context", "with_resource_condition"),
		filepath.Join("testdata", "logs", "log_context", "with_scope_condition"),
		filepath.Join("testdata", "logs", "log_context", "with_resource_and_scope_conditions"),
		filepath.Join("testdata", "logs", "mixed_context", "match_logs_then_grpc_request"),
		filepath.Join("testdata", "logs", "mixed_context", "match_logs_then_http_request"),
		filepath.Join("testdata", "logs", "mixed_context", "match_logs_then_resource"),
		filepath.Join("testdata", "logs", "mixed_context", "match_resource_then_grpc_request"),
		filepath.Join("testdata", "logs", "mixed_context", "match_resource_then_http_request"),
		filepath.Join("testdata", "logs", "mixed_context", "match_resource_then_logs"),
	}

	for _, tt := range testCases {
		t.Run(tt, func(t *testing.T) {

			cm, err := confmaptest.LoadConf(filepath.Join(tt, "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub("routing")
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			require.NoError(t, component.ValidateConfig(cfg))

			var sinkDefault, sink0, sink1 consumertest.LogsSink
			router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
				pipeline.NewIDWithName(pipeline.SignalLogs, "default"): &sinkDefault,
				pipeline.NewIDWithName(pipeline.SignalLogs, "0"):       &sink0,
				pipeline.NewIDWithName(pipeline.SignalLogs, "1"):       &sink1,
			})

			conn, err := factory.CreateLogsToLogs(
				context.Background(),
				connectortest.NewNopSettings(),
				cfg,
				router.(consumer.Logs),
			)
			require.NoError(t, err)

			var expected0, expected1, expectedDefault *plog.Logs
			if expected, readErr := golden.ReadLogs(filepath.Join(tt, "sink_0.yaml")); readErr == nil {
				expected0 = &expected
			} else if !os.IsNotExist(readErr) {
				t.Fatalf("Error reading sink_0.yaml: %v", readErr)
			}

			if expected, readErr := golden.ReadLogs(filepath.Join(tt, "sink_1.yaml")); readErr == nil {
				expected1 = &expected
			} else if !os.IsNotExist(readErr) {
				t.Fatalf("Error reading sink_1.yaml: %v", readErr)
			}

			if expected, readErr := golden.ReadLogs(filepath.Join(tt, "sink_default.yaml")); readErr == nil {
				expectedDefault = &expected
			} else if !os.IsNotExist(readErr) {
				t.Fatalf("Error reading sink_default.yaml: %v", readErr)
			}

			ctx := context.Background()
			if ctxFromFile, readErr := createContextFromFile(t, filepath.Join(tt, "request.yaml")); readErr == nil {
				ctx = ctxFromFile
			} else if !os.IsNotExist(readErr) {
				t.Fatalf("Error reading request.yaml: %v", readErr)
			}

			input, readErr := golden.ReadLogs(filepath.Join(tt, "input.yaml"))
			require.NoError(t, readErr)

			require.NoError(t, conn.ConsumeLogs(ctx, input))

			if expected0 == nil {
				assert.Empty(t, sink0.AllLogs(), "sink0 should be empty")
			} else {
				require.Len(t, sink0.AllLogs(), 1, "sink0 should have one plog.Logs")
				assert.NoError(t, plogtest.CompareLogs(*expected0, sink0.AllLogs()[0]), "sink0 has unexpected result")
			}

			if expected1 == nil {
				assert.Empty(t, sink1.AllLogs(), "sink1 should be empty")
			} else {
				require.Len(t, sink1.AllLogs(), 1, "sink1 should have one plog.Logs")
				assert.NoError(t, plogtest.CompareLogs(*expected1, sink1.AllLogs()[0]), "sink1 has unexpected result")
			}

			if expectedDefault == nil {
				assert.Empty(t, sinkDefault.AllLogs(), "sinkDefault should be empty")
			} else {
				require.Len(t, sinkDefault.AllLogs(), 1, "sinkDefault should have one plog.Logs")
				assert.NoError(t, plogtest.CompareLogs(*expectedDefault, sinkDefault.AllLogs()[0]), "sinkDefault has unexpected result")
			}
		})
	}
}
