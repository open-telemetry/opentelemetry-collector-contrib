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
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/grpc/metadata"
)

func TestTraces_RegisterExportersForValidRoute(t *testing.T) {
	// prepare
	exp, err := newTracesProcessor(noopTelemetrySettings, &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})
	require.NoError(t, err)

	otlpExpFactory := otlpexporter.NewFactory()
	otlpID := component.NewID("otlp")
	otlpConfig := &otlpexporter.Config{
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: "example.com:1234",
		},
	}
	otlpExp, err := otlpExpFactory.CreateTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), otlpConfig)
	require.NoError(t, err)

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeTraces: {
			otlpID: otlpExp,
		},
	})

	// test
	require.NoError(t, exp.Start(context.Background(), host))

	// verify
	assert.Contains(t, exp.router.getExporters("acme"), otlpExp)
}

func TestTraces_InvalidExporter(t *testing.T) {
	//  prepare
	exp, err := newTracesProcessor(noopTelemetrySettings, &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})
	require.NoError(t, err)

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeTraces: {
			component.NewID("otlp"): &mockComponent{},
		},
	})

	// test
	err = exp.Start(context.Background(), host)

	// verify
	assert.Error(t, err)
}

func TestTraces_AreCorrectlySplitPerResourceAttributeRouting(t *testing.T) {
	defaultExp := &mockTracesExporter{}
	tExp := &mockTracesExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeTraces: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): tExp,
		},
	})

	exp, err := newTracesProcessor(noopTelemetrySettings, &Config{
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

	tr := ptrace.NewTraces()

	rl := tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "acme")
	span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("span")

	rl = tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "acme")
	span = rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("span1")

	rl = tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "something-else")
	span = rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("span2")

	ctx := context.Background()
	require.NoError(t, exp.Start(ctx, host))
	require.NoError(t, exp.ConsumeTraces(ctx, tr))

	// The numbers below stem from the fact that data is routed and grouped
	// per resource attribute which is used for routing.
	// Hence the first 2 traces are grouped together under one ptrace.Traces.
	assert.Len(t, defaultExp.AllTraces(), 1,
		"one trace should be routed to default exporter",
	)
	assert.Len(t, tExp.AllTraces(), 1,
		"one trace should be routed to non default exporter",
	)
}

func TestTraces_RoutingWorks_Context(t *testing.T) {
	defaultExp := &mockTracesExporter{}
	tExp := &mockTracesExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeTraces: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): tExp,
		},
	})

	exp, err := newTracesProcessor(noopTelemetrySettings, &Config{
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

	tr := ptrace.NewTraces()
	rs := tr.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("X-Tenant", "acme")

	t.Run("grpc metadata: non default route is properly used", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeTraces(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "acme",
			})),
			tr,
		))
		assert.Len(t, defaultExp.AllTraces(), 0,
			"trace should not be routed to default exporter",
		)
		assert.Len(t, tExp.AllTraces(), 1,
			"trace should be routed to non default exporter",
		)
	})

	t.Run("grpc metadata: default route is taken when no matching route can be found", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeTraces(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "some-custom-value1",
			})),
			tr,
		))
		assert.Len(t, defaultExp.AllTraces(), 1,
			"trace should be routed to default exporter",
		)
		assert.Len(t, tExp.AllTraces(), 1,
			"trace should not be routed to non default exporter",
		)
	})

	t.Run("client.Info metadata: non default route is properly used", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeTraces(
			client.NewContext(context.Background(),
				client.Info{Metadata: client.NewMetadata(map[string][]string{
					"X-Tenant": {"acme"},
				})}),
			tr,
		))
		assert.Len(t, defaultExp.AllTraces(), 1,
			"trace should not be routed to default exporter",
		)
		assert.Len(t, tExp.AllTraces(), 2,
			"trace should be routed to non default exporter",
		)
	})

	t.Run("client.Info metadata: default route is taken when no matching route can be found", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeTraces(
			client.NewContext(context.Background(),
				client.Info{Metadata: client.NewMetadata(map[string][]string{
					"X-Tenant": {"some-custom-value1"},
				})}),
			tr,
		))
		assert.Len(t, defaultExp.AllTraces(), 2,
			"trace should be routed to default exporter",
		)
		assert.Len(t, tExp.AllTraces(), 2,
			"trace should not be routed to non default exporter",
		)
	})
}

func TestTraces_RoutingWorks_ResourceAttribute(t *testing.T) {
	defaultExp := &mockTracesExporter{}
	tExp := &mockTracesExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeTraces: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): tExp,
		},
	})

	exp, err := newTracesProcessor(noopTelemetrySettings, &Config{
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
		tr := ptrace.NewTraces()
		rs := tr.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("X-Tenant", "acme")

		assert.NoError(t, exp.ConsumeTraces(context.Background(), tr))
		assert.Len(t, defaultExp.AllTraces(), 0,
			"trace should not be routed to default exporter",
		)
		assert.Len(t, tExp.AllTraces(), 1,
			"trace should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		tr := ptrace.NewTraces()
		rs := tr.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("X-Tenant", "some-custom-value")

		assert.NoError(t, exp.ConsumeTraces(context.Background(), tr))
		assert.Len(t, defaultExp.AllTraces(), 1,
			"trace should be routed to default exporter",
		)
		assert.Len(t, tExp.AllTraces(), 1,
			"trace should not be routed to non default exporter",
		)
	})
}

func TestTraces_RoutingWorks_ResourceAttribute_DropsRoutingAttribute(t *testing.T) {
	defaultExp := &mockTracesExporter{}
	tExp := &mockTracesExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeTraces: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "2"): tExp,
		},
	})

	exp, err := newTracesProcessor(noopTelemetrySettings, &Config{
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

	tr := ptrace.NewTraces()
	rm := tr.ResourceSpans().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "acme")
	rm.Resource().Attributes().PutStr("attr", "acme")

	assert.NoError(t, exp.ConsumeTraces(context.Background(), tr))
	traces := tExp.AllTraces()
	require.Len(t, traces, 1,
		"trace should be routed to non default exporter",
	)
	require.Equal(t, 1, traces[0].ResourceSpans().Len())
	attrs := traces[0].ResourceSpans().At(0).Resource().Attributes()
	_, ok := attrs.Get("X-Tenant")
	assert.False(t, ok, "routing attribute should have been dropped")
	v, ok := attrs.Get("attr")
	assert.True(t, ok, "non-routing attributes shouldn't have been dropped")
	assert.Equal(t, "acme", v.Str())
}

func TestTracesAreCorrectlySplitPerResourceAttributeWithOTTL(t *testing.T) {
	defaultExp := &mockTracesExporter{}
	firstExp := &mockTracesExporter{}
	secondExp := &mockTracesExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeTraces: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "1"): firstExp,
			component.NewIDWithName("otlp", "2"): secondExp,
		},
	})

	exp, err := newTracesProcessor(noopTelemetrySettings, &Config{
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Statement: `route() where resource.attributes["value"] > 0 and resource.attributes["value"] < 4`,
				Exporters: []string{"otlp/1"},
			},
			{
				Statement: `route() where resource.attributes["value"] > 1 and resource.attributes["value"] < 4`,
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, exp.Start(context.Background(), host))

	t.Run("span by matched no expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 10)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		require.NoError(t, exp.ConsumeTraces(context.Background(), tr))

		assert.Len(t, defaultExp.AllTraces(), 1)
		assert.Len(t, firstExp.AllTraces(), 0)
		assert.Len(t, secondExp.AllTraces(), 0)
	})

	t.Run("span matched by one of two expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 1)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		require.NoError(t, exp.ConsumeTraces(context.Background(), tr))

		assert.Len(t, defaultExp.AllTraces(), 0)
		assert.Len(t, firstExp.AllTraces(), 1)
		assert.Len(t, secondExp.AllTraces(), 0)
	})

	t.Run("spans matched by all expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 2)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		rl = tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 3)
		span = rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span1")

		require.NoError(t, exp.ConsumeTraces(context.Background(), tr))

		assert.Len(t, defaultExp.AllTraces(), 0)
		assert.Len(t, firstExp.AllTraces(), 1)
		assert.Len(t, secondExp.AllTraces(), 1)

		assert.Equal(t, firstExp.AllTraces()[0].SpanCount(), 2)
		assert.Equal(t, secondExp.AllTraces()[0].SpanCount(), 2)
		assert.Equal(t, firstExp.AllTraces(), secondExp.AllTraces())
	})

	t.Run("one span matched by all expressions, other matched by none", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		tr := ptrace.NewTraces()
		rl := tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", 2)
		span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span")

		rl = tr.ResourceSpans().AppendEmpty()
		rl.Resource().Attributes().PutInt("value", -1)
		span = rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span1")

		require.NoError(t, exp.ConsumeTraces(context.Background(), tr))

		assert.Len(t, defaultExp.AllTraces(), 1)
		assert.Len(t, firstExp.AllTraces(), 1)
		assert.Len(t, secondExp.AllTraces(), 1)

		assert.Equal(t, firstExp.AllTraces(), secondExp.AllTraces())

		rspan := defaultExp.AllTraces()[0].ResourceSpans().At(0)
		attr, ok := rspan.Resource().Attributes().Get("value")
		assert.True(t, ok, "routing attribute must exists")
		assert.Equal(t, attr.Int(), int64(-1))
	})
}

// see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/26462
func TestTracesAttributeWithOTTLDoesNotCauseCrash(t *testing.T) {
	// prepare
	defaultExp := &mockTracesExporter{}
	firstExp := &mockTracesExporter{}

	host := newMockHost(map[component.DataType]map[component.ID]component.Component{
		component.DataTypeTraces: {
			component.NewID("otlp"):              defaultExp,
			component.NewIDWithName("otlp", "1"): firstExp,
		},
	})

	exp, err := newTracesProcessor(noopTelemetrySettings, &Config{
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Statement: `route() where attributes["value"] > 0`,
				Exporters: []string{"otlp/1"},
			},
		},
	})
	require.NoError(t, err)

	tr := ptrace.NewTraces()
	rl := tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().PutInt("value", 1)
	span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("span")

	require.NoError(t, exp.Start(context.Background(), host))

	// test
	// before #26464, this would panic
	require.NoError(t, exp.ConsumeTraces(context.Background(), tr))

	// verify
	assert.Len(t, defaultExp.AllTraces(), 1)
	assert.Len(t, firstExp.AllTraces(), 0)

}

func TestTraceProcessorCapabilities(t *testing.T) {
	// prepare
	config := &Config{
		FromAttribute: "X-Tenant",
		Table: []RoutingTableItem{{
			Value:     "acme",
			Exporters: []string{"otlp"},
		}},
	}

	// test
	p, err := newTracesProcessor(noopTelemetrySettings, config)
	require.NoError(t, err)
	require.NotNil(t, p)

	// verify
	assert.Equal(t, false, p.Capabilities().MutatesData)
}

type mockTracesExporter struct {
	mockComponent
	consumertest.TracesSink
}
