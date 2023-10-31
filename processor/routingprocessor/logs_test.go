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
	"go.opentelemetry.io/collector/pdata/plog"
	"google.golang.org/grpc/metadata"
)

func TestLogProcessorCapabilities(t *testing.T) {
	// prepare
	config := &Config{
		FromAttribute: "X-Tenant",
		Table: []RoutingTableItem{{
			Value:     "acme",
			Exporters: []string{"otlp"},
		}},
	}

	// test
	p, err := newLogProcessor(noopTelemetrySettings, config)
	require.NoError(t, err)
	require.NotNil(t, p)

	// verify
	assert.Equal(t, false, p.Capabilities().MutatesData)
}

func TestLogs_RoutingWorks_Context(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeLogs: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): lExp,
		},
	})

	exp, err := newLogProcessor(noopTelemetrySettings, &Config{
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

	l := plog.NewLogs()
	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "acme")

	t.Run("grpc metadata: non default route is properly used", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeLogs(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "acme",
			})),
			l,
		))
		assert.Len(t, defaultExp.AllLogs(), 0,
			"log should not be routed to default exporter",
		)
		assert.Len(t, lExp.AllLogs(), 1,
			"log should be routed to non default exporter",
		)
	})

	t.Run("grpc metadata: default route is taken when no matching route can be found", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeLogs(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "some-custom-value1",
			})),
			l,
		))
		assert.Len(t, defaultExp.AllLogs(), 1,
			"log should be routed to default exporter",
		)
		assert.Len(t, lExp.AllLogs(), 1,
			"log should not be routed to non default exporter",
		)
	})

	t.Run("client.Info metadata: non default route is properly used", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeLogs(
			client.NewContext(context.Background(),
				client.Info{Metadata: client.NewMetadata(map[string][]string{
					"X-Tenant": {"acme"},
				})}),
			l,
		))
		assert.Len(t, defaultExp.AllLogs(), 1,
			"log should not be routed to default exporter",
		)
		assert.Len(t, lExp.AllLogs(), 2,
			"log should be routed to non default exporter",
		)
	})

	t.Run("client.Info metadata: default route is taken when no matching route can be found", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeLogs(client.NewContext(context.Background(),
			client.Info{Metadata: client.NewMetadata(map[string][]string{
				"X-Tenant": {"some-custom-value1"},
			})}),
			l,
		))
		assert.Len(t, defaultExp.AllLogs(), 2,
			"log should be routed to default exporter",
		)
		assert.Len(t, lExp.AllLogs(), 2,
			"log should not be routed to non default exporter",
		)
	})
}

func TestLogs_RoutingWorks_ResourceAttribute(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeLogs: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): lExp,
		},
	})

	exp, err := newLogProcessor(noopTelemetrySettings, &Config{
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
		l := plog.NewLogs()
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "acme")

		assert.NoError(t, exp.ConsumeLogs(context.Background(), l))
		assert.Len(t, defaultExp.AllLogs(), 0,
			"log should not be routed to default exporter",
		)
		assert.Len(t, lExp.AllLogs(), 1,
			"log should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		l := plog.NewLogs()
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "some-custom-value")

		assert.NoError(t, exp.ConsumeLogs(context.Background(), l))
		assert.Len(t, defaultExp.AllLogs(), 1,
			"log should be routed to default exporter",
		)
		assert.Len(t, lExp.AllLogs(), 1,
			"log should not be routed to non default exporter",
		)
	})
}

func TestLogs_RoutingWorks_ResourceAttribute_DropsRoutingAttribute(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeLogs: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): lExp,
		},
	})

	exp, err := newLogProcessor(noopTelemetrySettings, &Config{
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

	l := plog.NewLogs()
	rm := l.ResourceLogs().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "acme")
	rm.Resource().Attributes().PutStr("attr", "acme")

	assert.NoError(t, exp.ConsumeLogs(context.Background(), l))
	logs := lExp.AllLogs()
	require.Len(t, logs, 1, "log should be routed to non-default exporter")
	require.Equal(t, 1, logs[0].ResourceLogs().Len())
	attrs := logs[0].ResourceLogs().At(0).Resource().Attributes()
	_, ok := attrs.Get("X-Tenant")
	assert.False(t, ok, "routing attribute should have been dropped")
	v, ok := attrs.Get("attr")
	assert.True(t, ok, "non routing attributes shouldn't be dropped")
	assert.Equal(t, "acme", v.Str())
}

func TestLogs_AreCorrectlySplitPerResourceAttributeRouting(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeLogs: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): lExp,
		},
	})

	exp, err := newLogProcessor(noopTelemetrySettings, &Config{
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

	l := plog.NewLogs()

	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "acme")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	rl = l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "acme")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	rl = l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "something-else")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ctx := context.Background()
	require.NoError(t, exp.Start(ctx, host))
	require.NoError(t, exp.ConsumeLogs(ctx, l))

	// The numbers below stem from the fact that data is routed and grouped
	// per resource attribute which is used for routing.
	// Hence the first 2 metrics are grouped together under one plog.Logs.
	assert.Len(t, defaultExp.AllLogs(), 1,
		"one log should be routed to default exporter",
	)
	assert.Len(t, lExp.AllLogs(), 1,
		"one log should be routed to non default exporter",
	)
}

func TestLogsAreCorrectlySplitPerResourceAttributeWithOTTL(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	firstExp := &mockLogsExporter{}
	secondExp := &mockLogsExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeLogs: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "1"): firstExp,
			component.NewIDWithName("otlp", "2"): secondExp,
		},
	})

	exp, err := newLogProcessor(noopTelemetrySettings, &Config{
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Statement: `route() where IsMatch(resource.attributes["X-Tenant"], ".*acme")`,
				Exporters: []string{"otlp/1"},
			},
			{
				Statement: `route() where IsMatch(resource.attributes["X-Tenant"], "_acme")`,
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, exp.Start(context.Background(), host))

	t.Run("logs matched by no expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		l := plog.NewLogs()
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "something-else")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, exp.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultExp.AllLogs(), 1)
		assert.Len(t, firstExp.AllLogs(), 0)
		assert.Len(t, secondExp.AllLogs(), 0)
	})

	t.Run("logs matched one of two expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "xacme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, exp.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultExp.AllLogs(), 0)
		assert.Len(t, firstExp.AllLogs(), 1)
		assert.Len(t, secondExp.AllLogs(), 0)
	})

	t.Run("logs matched by all expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "x_acme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		rl = l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "_acme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, exp.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultExp.AllLogs(), 0)
		assert.Len(t, firstExp.AllLogs(), 1)
		assert.Len(t, secondExp.AllLogs(), 1)

		assert.Equal(t, firstExp.AllLogs()[0].LogRecordCount(), 2)
		assert.Equal(t, secondExp.AllLogs()[0].LogRecordCount(), 2)
		assert.Equal(t, firstExp.AllLogs(), secondExp.AllLogs())
	})

	t.Run("one log matched by all expressions, other matched none", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "_acme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		rl = l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "something-else")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, exp.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultExp.AllLogs(), 1)
		assert.Len(t, firstExp.AllLogs(), 1)
		assert.Len(t, secondExp.AllLogs(), 1)

		assert.Equal(t, firstExp.AllLogs(), secondExp.AllLogs())

		rspan := defaultExp.AllLogs()[0].ResourceLogs().At(0)
		attr, ok := rspan.Resource().Attributes().Get("X-Tenant")
		assert.True(t, ok, "routing attribute must exists")
		assert.Equal(t, attr.AsString(), "something-else")
	})
}

// see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/26462
func TestLogsAttributeWithOTTLDoesNotCauseCrash(t *testing.T) {
	// prepare
	defaultExp := &mockLogsExporter{}
	firstExp := &mockLogsExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeLogs: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "1"): firstExp,
		},
	})

	exp, err := newLogProcessor(noopTelemetrySettings, &Config{
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Statement: `route() where attributes["value"] > 0`,
				Exporters: []string{"otlp/1"},
			},
		},
	})
	require.NoError(t, err)

	l := plog.NewLogs()

	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutInt("value", 1)
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	require.NoError(t, exp.Start(context.Background(), host))

	// test
	// before #26464, this would panic
	require.NoError(t, exp.ConsumeLogs(context.Background(), l))

	// verify
	assert.Len(t, defaultExp.AllLogs(), 1)
	assert.Len(t, firstExp.AllLogs(), 0)
}

type mockLogsExporter struct {
	mockComponent
	consumertest.LogsSink
}
