// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func TestTraces_RegisterExportersForValidRoute(t *testing.T) {
	// prepare
	exp := newTracesProcessor(component.TelemetrySettings{}, &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})

	otlpExpFactory := otlpexporter.NewFactory()
	otlpConfig := &otlpexporter.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("otlp")),
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: "example.com:1234",
		},
	}
	otlpExp, err := otlpExpFactory.CreateTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), otlpConfig)
	require.NoError(t, err)

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					otlpConfig.ID(): otlpExp,
				},
			}
		},
	}

	// test
	require.NoError(t, exp.Start(context.Background(), host))

	// verify
	assert.Contains(t, exp.router.getExporters("acme"), otlpExp)
}

func TestTraces_InvalidExporter(t *testing.T) {
	//  prepare
	exp := newTracesProcessor(component.TelemetrySettings{}, &Config{
		DefaultExporters: []string{"otlp"},
		FromAttribute:    "X-Tenant",
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp"},
			},
		},
	})

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp"): &mockComponent{},
				},
			}
		},
	}

	// test
	err := exp.Start(context.Background(), host)

	// verify
	assert.Error(t, err)
}

func TestTraces_AreCorrectlySplitPerResourceAttributeRouting(t *testing.T) {
	defaultExp := &mockTracesExporter{}
	tExp := &mockTracesExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp"):              defaultExp,
					config.NewComponentIDWithName("otlp", "2"): tExp,
				},
			}
		},
	}

	exp := newTracesProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, &Config{
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

	tr := ptrace.NewTraces()

	rl := tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().PutString("X-Tenant", "acme")
	span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("span")

	rl = tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().PutString("X-Tenant", "acme")
	span = rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("span1")

	rl = tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().PutString("X-Tenant", "something-else")
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

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp"):              defaultExp,
					config.NewComponentIDWithName("otlp", "2"): tExp,
				},
			}
		},
	}

	exp := newTracesProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, &Config{
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
	require.NoError(t, exp.Start(context.Background(), host))

	tr := ptrace.NewTraces()
	rs := tr.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutString("X-Tenant", "acme")

	t.Run("non default route is properly used", func(t *testing.T) {
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

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
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
}

func TestTraces_RoutingWorks_ResourceAttribute(t *testing.T) {
	defaultExp := &mockTracesExporter{}
	tExp := &mockTracesExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp"):              defaultExp,
					config.NewComponentIDWithName("otlp", "2"): tExp,
				},
			}
		},
	}

	exp := newTracesProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, &Config{
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
	require.NoError(t, exp.Start(context.Background(), host))

	t.Run("non default route is properly used", func(t *testing.T) {
		tr := ptrace.NewTraces()
		rs := tr.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutString("X-Tenant", "acme")

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
		rs.Resource().Attributes().PutString("X-Tenant", "some-custom-value")

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

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp"):              defaultExp,
					config.NewComponentIDWithName("otlp", "2"): tExp,
				},
			}
		},
	}

	exp := newTracesProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, &Config{
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
	require.NoError(t, exp.Start(context.Background(), host))

	tr := ptrace.NewTraces()
	rm := tr.ResourceSpans().AppendEmpty()
	rm.Resource().Attributes().PutString("X-Tenant", "acme")
	rm.Resource().Attributes().PutString("attr", "acme")

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

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[config.DataType]map[config.ComponentID]component.Exporter {
			return map[config.DataType]map[config.ComponentID]component.Exporter{
				config.TracesDataType: {
					config.NewComponentID("otlp"):              defaultExp,
					config.NewComponentIDWithName("otlp", "1"): firstExp,
					config.NewComponentIDWithName("otlp", "2"): secondExp,
				},
			}
		},
	}

	exp := newTracesProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, &Config{
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Expression: `route() where resource.attributes["value"] > 0 and resource.attributes["value"] < 4`,
				Exporters:  []string{"otlp/1"},
			},
			{
				Expression: `route() where resource.attributes["value"] > 1 and resource.attributes["value"] < 4`,
				Exporters:  []string{"otlp/2"},
			},
		},
	})
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
	p := newTracesProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, config)
	require.NotNil(t, p)

	// verify
	assert.Equal(t, false, p.Capabilities().MutatesData)
}

type mockTracesExporter struct {
	mockComponent
	consumertest.TracesSink
}
