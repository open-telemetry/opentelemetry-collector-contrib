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
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/fanoutconsumer"
)

func TestTraces_RegisterConsumersForValidRoute(t *testing.T) {
	cfg := &Config{
		DefaultPipelines: []string{"traces/default"},
		Table: []RoutingTableItem{
			{
				Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
				Pipelines: []string{"traces/0"},
			},
			{
				Statement: `route() where resource.attributes["X-Tenant"] == "*"`,
				Pipelines: []string{"traces/0", "traces/1"},
			},
		},
	}

	require.NoError(t, cfg.Validate())

	defaultSinkID := component.NewIDWithName(component.DataTypeTraces, "default")
	defaultSink := &consumertest.TracesSink{}

	sink0ID := component.NewIDWithName(component.DataTypeTraces, "0")
	sink0 := &consumertest.TracesSink{}

	sink1ID := component.NewIDWithName(component.DataTypeTraces, "1")
	sink1 := &consumertest.TracesSink{}

	router := fanoutconsumer.NewTracesRouter(
		map[component.ID]consumer.Traces{
			defaultSinkID: defaultSink,
			sink0ID:       sink0,
			sink1ID:       sink1,
		})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router)

	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.False(t, conn.Capabilities().MutatesData)

	rtConn := conn.(*tracesConnector)
	require.NoError(t, err)
	require.Same(t, defaultSink, rtConn.router.defaultConsumer)

	route, ok := rtConn.router.routes[rtConn.router.table[0].Statement]
	assert.True(t, ok)
	require.Same(t, sink0, route.consumer)

	route, ok = rtConn.router.routes[rtConn.router.table[1].Statement]
	assert.True(t, ok)

	routeConsumer, err := router.(connector.TracesRouter).Consumer(sink0ID, sink1ID)
	require.NoError(t, err)
	require.Equal(t, routeConsumer, route.consumer)

	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()
}

func TestTracesAreCorrectlySplitPerResourceAttributeWithOTTL(t *testing.T) {
	cfg := &Config{
		DefaultPipelines: []string{"traces/default"},
		Table: []RoutingTableItem{
			{
				Statement: `route() where resource.attributes["value"] > 0 and resource.attributes["value"] < 4`,
				Pipelines: []string{"traces/0"},
			},
			{
				Statement: `route() where resource.attributes["value"] > 1 and resource.attributes["value"] < 4`,
				Pipelines: []string{"traces/1"},
			},
			{
				Statement: `route() where resource.attributes["value"] == 5`,
				Pipelines: []string{"traces/default", "traces/0"},
			},
		},
	}

	defaultSink := &consumertest.TracesSink{}
	sink0 := &consumertest.TracesSink{}
	sink1 := &consumertest.TracesSink{}

	resetSinks := func() {
		defaultSink.Reset()
		sink0.Reset()
		sink1.Reset()
	}

	consumer := fanoutconsumer.NewTracesRouter(
		map[component.ID]consumer.Traces{
			component.NewIDWithName(component.DataTypeTraces, "default"): defaultSink,
			component.NewIDWithName(component.DataTypeTraces, "0"):       sink0,
			component.NewIDWithName(component.DataTypeTraces, "1"):       sink1,
		})

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(context.Background(), connectortest.NewNopCreateSettings(), cfg, consumer)

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
		assert.Len(t, sink0.AllTraces(), 0)
		assert.Len(t, sink1.AllTraces(), 0)
	})

	t.Run("span matched by one of two expressions", func(t *testing.T) {
		resetSinks()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 1)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))

		assert.Len(t, defaultSink.AllTraces(), 0)
		assert.Len(t, sink0.AllTraces(), 1)
		assert.Len(t, sink1.AllTraces(), 0)
	})

	t.Run("span matched by all expressions", func(t *testing.T) {
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

		assert.Len(t, defaultSink.AllTraces(), 0)
		assert.Len(t, sink0.AllTraces(), 1)
		assert.Len(t, sink1.AllTraces(), 1)

		assert.Equal(t, sink0.AllTraces()[0].SpanCount(), 2)
		assert.Equal(t, sink1.AllTraces()[0].SpanCount(), 2)
		assert.Equal(t, sink0.AllTraces(), sink1.AllTraces())
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
		assert.Len(t, sink1.AllTraces(), 0)

		assert.Equal(t, defaultSink.AllTraces()[0].SpanCount(), 1)
		assert.Equal(t, sink0.AllTraces()[0].SpanCount(), 1)
		assert.Equal(t, defaultSink.AllTraces(), sink0.AllTraces())
	})
}

func TestTraces_ResourceAttribute_DroppedByOTTL(t *testing.T) {
	cfg := &Config{
		DefaultPipelines: []string{"traces/default"},
		Table: []RoutingTableItem{
			{
				Statement: `delete_key(resource.attributes, "X-Tenant") where resource.attributes["X-Tenant"] == "acme"`,
				Pipelines: []string{"traces/0"},
			},
		},
	}

	sink0 := &consumertest.TracesSink{}
	sink1 := &consumertest.TracesSink{}

	consumer := fanoutconsumer.NewTracesRouter(
		map[component.ID]consumer.Traces{
			component.NewIDWithName(component.DataTypeTraces, "default"): sink0,
			component.NewIDWithName(component.DataTypeTraces, "0"):       sink1,
		})

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(context.Background(), connectortest.NewNopCreateSettings(), cfg, consumer)

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
	require.Len(t, sink0.AllTraces(), 0,
		"trace should not be routed to default pipeline",
	)
}

func TestTraceProcessorCapabilities(t *testing.T) {
	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
			Pipelines: []string{"traces/0"},
		}},
	}

	consumer := fanoutconsumer.NewTracesRouter(
		map[component.ID]consumer.Traces{
			component.NewIDWithName(component.DataTypeTraces, "default"): &consumertest.TracesSink{},
			component.NewIDWithName(component.DataTypeTraces, "0"):       &consumertest.TracesSink{},
		})

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(context.Background(), connectortest.NewNopCreateSettings(), cfg, consumer)

	require.NoError(t, err)
	assert.Equal(t, false, conn.Capabilities().MutatesData)
}
