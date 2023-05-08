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
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/fanoutconsumer"
)

func TestLogsRegisterConsumersForValidRoute(t *testing.T) {
	logsDefault := component.NewIDWithName(component.DataTypeLogs, "default")
	logs0 := component.NewIDWithName(component.DataTypeLogs, "0")
	logs1 := component.NewIDWithName(component.DataTypeLogs, "1")

	cfg := &Config{
		DefaultPipelines: []component.ID{logsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
				Pipelines: []component.ID{logs0},
			},
			{
				Statement: `route() where resource.attributes["X-Tenant"] == "*"`,
				Pipelines: []component.ID{logs0, logs1},
			},
		},
	}

	require.NoError(t, cfg.Validate())

	defaultSink := &consumertest.LogsSink{}
	sink0 := &consumertest.LogsSink{}
	sink1 := &consumertest.LogsSink{}

	router := fanoutconsumer.NewLogsRouter(
		map[component.ID]consumer.Logs{
			logsDefault: defaultSink,
			logs0:       sink0,
			logs1:       sink1,
		})

	conn, err := NewFactory().CreateLogsToLogs(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router)

	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.False(t, conn.Capabilities().MutatesData)

	rtConn := conn.(*logsConnector)
	require.NoError(t, err)
	require.Same(t, defaultSink, rtConn.router.defaultConsumer)

	route, ok := rtConn.router.routes[rtConn.router.table[0].Statement]
	assert.True(t, ok)
	require.Same(t, sink0, route.consumer)

	route, ok = rtConn.router.routes[rtConn.router.table[1].Statement]
	assert.True(t, ok)

	routeConsumer, err := router.(connector.LogsRouter).Consumer(logs0, logs1)
	require.NoError(t, err)
	require.Equal(t, routeConsumer, route.consumer)

	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()
}

func TestLogsAreCorrectlySplitPerResourceAttributeWithOTTL(t *testing.T) {
	logsDefault := component.NewIDWithName(component.DataTypeLogs, "default")
	logs0 := component.NewIDWithName(component.DataTypeLogs, "0")
	logs1 := component.NewIDWithName(component.DataTypeLogs, "1")

	cfg := &Config{
		DefaultPipelines: []component.ID{logsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `route() where IsMatch(resource.attributes["X-Tenant"], ".*acme") == true`,
				Pipelines: []component.ID{logs0},
			},
			{
				Statement: `route() where IsMatch(resource.attributes["X-Tenant"], "_acme") == true`,
				Pipelines: []component.ID{logs1},
			},
			{
				Statement: `route() where resource.attributes["X-Tenant"] == "ecorp"`,
				Pipelines: []component.ID{logsDefault, logs0},
			},
		},
	}

	defaultSink := &consumertest.LogsSink{}
	sink0 := &consumertest.LogsSink{}
	sink1 := &consumertest.LogsSink{}

	resetSinks := func() {
		defaultSink.Reset()
		sink0.Reset()
		sink1.Reset()
	}

	consumer := fanoutconsumer.NewLogsRouter(
		map[component.ID]consumer.Logs{
			logsDefault: defaultSink,
			logs0:       sink0,
			logs1:       sink1,
		})

	factory := NewFactory()
	conn, err := factory.CreateLogsToLogs(
		context.Background(),
		connectortest.NewNopCreateSettings(),
		cfg,
		consumer,
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
		assert.Len(t, sink0.AllLogs(), 0)
		assert.Len(t, sink1.AllLogs(), 0)
	})

	t.Run("logs matched one expression", func(t *testing.T) {
		resetSinks()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "xacme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, conn.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultSink.AllLogs(), 0)
		assert.Len(t, sink0.AllLogs(), 1)
		assert.Len(t, sink1.AllLogs(), 0)
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

		assert.Len(t, defaultSink.AllLogs(), 0)
		assert.Len(t, sink0.AllLogs(), 1)
		assert.Len(t, sink1.AllLogs(), 1)

		assert.Equal(t, sink0.AllLogs()[0].LogRecordCount(), 2)
		assert.Equal(t, sink1.AllLogs()[0].LogRecordCount(), 2)
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
		assert.Equal(t, attr.AsString(), "something-else")
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
		assert.Len(t, sink1.AllLogs(), 0)

		assert.Equal(t, defaultSink.AllLogs()[0].LogRecordCount(), 1)
		assert.Equal(t, sink0.AllLogs()[0].LogRecordCount(), 1)
		assert.Equal(t, defaultSink.AllLogs(), sink0.AllLogs())
	})
}

func TestLogsResourceAttributeDroppedByOTTL(t *testing.T) {
	logsDefault := component.NewIDWithName(component.DataTypeLogs, "default")
	logsOther := component.NewIDWithName(component.DataTypeLogs, "other")

	cfg := &Config{
		DefaultPipelines: []component.ID{logsDefault},
		Table: []RoutingTableItem{
			{
				Statement: `delete_key(resource.attributes, "X-Tenant") where resource.attributes["X-Tenant"] == "acme"`,
				Pipelines: []component.ID{logsOther},
			},
		},
	}

	sink0 := &consumertest.LogsSink{}
	sink1 := &consumertest.LogsSink{}

	consumer := fanoutconsumer.NewLogsRouter(
		map[component.ID]consumer.Logs{
			logsDefault: sink0,
			logsOther:   sink1,
		})

	factory := NewFactory()
	conn, err := factory.CreateLogsToLogs(
		context.Background(),
		connectortest.NewNopCreateSettings(),
		cfg,
		consumer,
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
	require.Len(t, sink0.AllLogs(), 0,
		"metrics should not be routed to default pipeline",
	)
}

func TestLogsConnectorCapabilities(t *testing.T) {
	logsDefault := component.NewIDWithName(component.DataTypeLogs, "default")
	logsOther := component.NewIDWithName(component.DataTypeLogs, "other")

	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
			Pipelines: []component.ID{logsOther},
		}},
	}

	sink0 := &consumertest.LogsSink{}
	sink1 := &consumertest.LogsSink{}

	consumer := fanoutconsumer.NewLogsRouter(
		map[component.ID]consumer.Logs{
			logsDefault: sink0,
			logsOther:   sink1,
		})

	factory := NewFactory()
	conn, err := factory.CreateLogsToLogs(
		context.Background(),
		connectortest.NewNopCreateSettings(),
		cfg,
		consumer,
	)

	require.NoError(t, err)
	assert.Equal(t, false, conn.Capabilities().MutatesData)
}
