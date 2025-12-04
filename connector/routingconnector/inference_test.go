// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
)

func TestContextInference(t *testing.T) {
	testCases := []struct {
		name            string
		routeTable      []RoutingTableItem
		input           plog.Logs
		expectedRoute   string // "prod" or "dev"
		expectedContext string // "resource", "log", etc.
	}{
		{
			name: "Explicit context qualified path",
			routeTable: []RoutingTableItem{
				{
					Condition: `resource.attributes["env"] == "prod"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "prod")},
				},
			},
			input: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("env", "prod")
				return ld
			}(),
			expectedRoute:   "prod",
			expectedContext: "resource",
		},
		{
			name: "Inferred context unqualified path (legacy behavior)",
			routeTable: []RoutingTableItem{
				{
					Condition: `attributes["env"] == "dev"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "dev")},
				},
			},
			input: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("env", "dev")
				return ld
			}(),
			expectedRoute:   "dev",
			expectedContext: "resource",
		},
		{
			name: "Inferred context explicit path (log context)",
			routeTable: []RoutingTableItem{
				{
					Condition: `log.body == "something"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "prod")},
				},
			},
			input: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				sl := rl.ScopeLogs().AppendEmpty()
				l := sl.LogRecords().AppendEmpty()
				l.Body().SetStr("something")
				return ld
			}(),
			expectedRoute:   "prod",
			expectedContext: "log",
		},
		{
			name: "Inferred log context with log.attributes",
			routeTable: []RoutingTableItem{
				{
					Condition: `log.attributes["level"] == "error"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "prod")},
				},
			},
			input: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				sl := rl.ScopeLogs().AppendEmpty()
				l := sl.LogRecords().AppendEmpty()
				l.Attributes().PutStr("level", "error")
				return ld
			}(),
			expectedRoute:   "prod",
			expectedContext: "log",
		},
		{
			name: "Explicit context field overrides inference",
			routeTable: []RoutingTableItem{
				{
					Context:   "resource",
					Condition: `attributes["env"] == "prod"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "prod")},
				},
			},
			input: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("env", "prod")
				return ld
			}(),
			expectedRoute:   "prod",
			expectedContext: "resource",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mocks
			prodConsumer := new(consumertest.LogsSink)
			devConsumer := new(consumertest.LogsSink)
			defaultConsumer := new(consumertest.LogsSink)

			provider := func(ids ...pipeline.ID) (consumer.Logs, error) {
				if len(ids) == 0 {
					return defaultConsumer, nil
				}
				switch ids[0].Name() {
				case "prod":
					return prodConsumer, nil
				case "dev":
					return devConsumer, nil
				default:
					return nil, pipeline.ErrSignalNotSupported
				}
			}

			// Create router
			router, err := newRouter(
				tc.routeTable,
				[]pipeline.ID{},
				provider,
				componenttest.NewNopTelemetrySettings(),
			)
			require.NoError(t, err)

			// Verify routing logic
			// Check if statements were created correctly
			require.Len(t, router.routes, 1)
			for _, route := range router.routes {
				assert.Equal(t, tc.expectedContext, route.statementContext)
			}
		})
	}
}

func TestContextInferenceForTraces(t *testing.T) {
	testCases := []struct {
		name            string
		routeTable      []RoutingTableItem
		expectedContext string
	}{
		{
			name: "Inferred span context from span.attributes",
			routeTable: []RoutingTableItem{
				{
					Condition: `span.attributes["http.method"] == "GET"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "http")},
				},
			},
			expectedContext: "span",
		},
		{
			name: "Inferred span context from span.name",
			routeTable: []RoutingTableItem{
				{
					Condition: `span.name == "HTTP GET"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "http")},
				},
			},
			expectedContext: "span",
		},
		{
			name: "Resource context with qualified path in traces",
			routeTable: []RoutingTableItem{
				{
					Condition: `resource.attributes["service.name"] == "frontend"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "frontend")},
				},
			},
			expectedContext: "resource",
		},
		{
			name: "Unqualified attributes defaults to resource in traces",
			routeTable: []RoutingTableItem{
				{
					Condition: `attributes["env"] == "prod"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "prod")},
				},
			},
			expectedContext: "resource",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sink := new(consumertest.TracesSink)

			provider := func(ids ...pipeline.ID) (consumer.Traces, error) {
				return sink, nil
			}

			router, err := newRouter(
				tc.routeTable,
				[]pipeline.ID{},
				provider,
				componenttest.NewNopTelemetrySettings(),
			)
			require.NoError(t, err)
			require.Len(t, router.routes, 1)

			for _, route := range router.routes {
				assert.Equal(t, tc.expectedContext, route.statementContext)
			}
		})
	}
}

func TestContextInferenceForMetrics(t *testing.T) {
	testCases := []struct {
		name            string
		routeTable      []RoutingTableItem
		expectedContext string
	}{
		{
			name: "Inferred metric context from metric.name",
			routeTable: []RoutingTableItem{
				{
					Condition: `metric.name == "http_requests_total"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalMetrics, "http")},
				},
			},
			expectedContext: "metric",
		},
		{
			name: "Inferred datapoint context from datapoint.attributes",
			routeTable: []RoutingTableItem{
				{
					Condition: `datapoint.attributes["host"] == "server1"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalMetrics, "server1")},
				},
			},
			expectedContext: "datapoint",
		},
		{
			name: "Resource context with qualified path in metrics",
			routeTable: []RoutingTableItem{
				{
					Condition: `resource.attributes["service.name"] == "backend"`,
					Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalMetrics, "backend")},
				},
			},
			expectedContext: "resource",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)

			provider := func(ids ...pipeline.ID) (consumer.Metrics, error) {
				return sink, nil
			}

			router, err := newRouter(
				tc.routeTable,
				[]pipeline.ID{},
				provider,
				componenttest.NewNopTelemetrySettings(),
			)
			require.NoError(t, err)
			require.Len(t, router.routes, 1)

			for _, route := range router.routes {
				assert.Equal(t, tc.expectedContext, route.statementContext)
			}
		})
	}
}

func TestMixedContextsInRoutingTable(t *testing.T) {
	// Test that a single routing table can have routes with different contexts
	routeTable := []RoutingTableItem{
		{
			Condition: `resource.attributes["env"] == "prod"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "prod")},
		},
		{
			Condition: `log.severity_text == "ERROR"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "errors")},
		},
		{
			Context:   "request",
			Condition: `request["X-Tenant"] == "acme"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "acme")},
		},
	}

	prodSink := new(consumertest.LogsSink)
	errorsSink := new(consumertest.LogsSink)
	acmeSink := new(consumertest.LogsSink)

	logsRouter := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		pipeline.NewIDWithName(pipeline.SignalLogs, "prod"):   prodSink,
		pipeline.NewIDWithName(pipeline.SignalLogs, "errors"): errorsSink,
		pipeline.NewIDWithName(pipeline.SignalLogs, "acme"):   acmeSink,
	})

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Table = routeTable

	conn, err := newLogsConnector(connector.Settings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}, cfg, logsRouter)
	require.NoError(t, err)

	// Verify the contexts were correctly inferred
	require.Len(t, conn.router.routeSlice, 3)
	assert.Equal(t, "resource", conn.router.routeSlice[0].statementContext)
	assert.Equal(t, "log", conn.router.routeSlice[1].statementContext)
	assert.Equal(t, "request", conn.router.routeSlice[2].statementContext)
}

func TestRouterInferenceIntegration(t *testing.T) {
	// This test verifies that the inference actually works when executing the router logic
	// We'll simulate the behavior of LogsRouter which uses the internal router

	routeTable := []RoutingTableItem{
		{
			Condition: `resource.attributes["env"] == "prod"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "prod")},
		},
		{
			Condition: `attributes["env"] == "dev"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "dev")},
		},
	}

	prodConsumer := new(consumertest.LogsSink)
	devConsumer := new(consumertest.LogsSink)

	// Use connector.NewLogsRouter to satisfy the interface
	logsRouter := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		pipeline.NewIDWithName(pipeline.SignalLogs, "prod"): prodConsumer,
		pipeline.NewIDWithName(pipeline.SignalLogs, "dev"):  devConsumer,
	})

	// We need to execute the routing.
	// We can create a logsConnector to test the routing logic
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Table = routeTable

	router, err := newLogsConnector(connector.Settings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}, cfg, logsRouter)
	require.NoError(t, err)

	// Case 1: Prod (Inferred resource context)
	ldProd := plog.NewLogs()
	rlProd := ldProd.ResourceLogs().AppendEmpty()
	rlProd.Resource().Attributes().PutStr("env", "prod")
	// Add a log record to ensure it's processed
	rlProd.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err = router.ConsumeLogs(context.Background(), ldProd)
	require.NoError(t, err)
	assert.Equal(t, 1, prodConsumer.LogRecordCount())
	assert.Equal(t, 0, devConsumer.LogRecordCount())

	// Case 2: Dev (Unqualified path -> Inferred resource context)
	ldDev := plog.NewLogs()
	rlDev := ldDev.ResourceLogs().AppendEmpty()
	rlDev.Resource().Attributes().PutStr("env", "dev")
	// Add a log record to ensure it's processed
	rlDev.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	// Reset consumers
	prodConsumer.Reset()
	devConsumer.Reset()

	err = router.ConsumeLogs(context.Background(), ldDev)
	require.NoError(t, err)
	assert.Equal(t, 0, prodConsumer.LogRecordCount())
	assert.Equal(t, 1, devConsumer.LogRecordCount())
}

func TestLogContextInferenceIntegration(t *testing.T) {
	// Test that log context inference works for actual routing
	routeTable := []RoutingTableItem{
		{
			Condition: `log.severity_text == "ERROR"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "errors")},
		},
	}

	errorsSink := new(consumertest.LogsSink)
	defaultSink := new(consumertest.LogsSink)

	logsRouter := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		pipeline.NewIDWithName(pipeline.SignalLogs, "errors"):  errorsSink,
		pipeline.NewIDWithName(pipeline.SignalLogs, "default"): defaultSink,
	})

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Table = routeTable
	cfg.DefaultPipelines = []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "default")}

	conn, err := newLogsConnector(connector.Settings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}, cfg, logsRouter)
	require.NoError(t, err)

	// Create log with ERROR severity
	ldError := plog.NewLogs()
	rl := ldError.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.SetSeverityText("ERROR")

	err = conn.ConsumeLogs(context.Background(), ldError)
	require.NoError(t, err)
	assert.Equal(t, 1, errorsSink.LogRecordCount())
	assert.Equal(t, 0, defaultSink.LogRecordCount())

	// Reset and test non-matching log
	errorsSink.Reset()
	defaultSink.Reset()

	ldInfo := plog.NewLogs()
	rl = ldInfo.ResourceLogs().AppendEmpty()
	sl = rl.ScopeLogs().AppendEmpty()
	lr = sl.LogRecords().AppendEmpty()
	lr.SetSeverityText("INFO")

	err = conn.ConsumeLogs(context.Background(), ldInfo)
	require.NoError(t, err)
	assert.Equal(t, 0, errorsSink.LogRecordCount())
	assert.Equal(t, 1, defaultSink.LogRecordCount())
}

func TestSpanContextInferenceIntegration(t *testing.T) {
	// Test that span context inference works for actual routing
	routeTable := []RoutingTableItem{
		{
			Condition: `span.attributes["http.method"] == "GET"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "http_get")},
		},
	}

	httpGetSink := new(consumertest.TracesSink)
	defaultSink := new(consumertest.TracesSink)

	tracesRouter := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		pipeline.NewIDWithName(pipeline.SignalTraces, "http_get"): httpGetSink,
		pipeline.NewIDWithName(pipeline.SignalTraces, "default"):  defaultSink,
	})

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Table = routeTable
	cfg.DefaultPipelines = []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "default")}

	conn, err := newTracesConnector(connector.Settings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}, cfg, tracesRouter)
	require.NoError(t, err)

	// Create trace with GET method
	tdGet := ptrace.NewTraces()
	rs := tdGet.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "GET")

	err = conn.ConsumeTraces(context.Background(), tdGet)
	require.NoError(t, err)
	assert.Equal(t, 1, httpGetSink.SpanCount())
	assert.Equal(t, 0, defaultSink.SpanCount())

	// Reset and test non-matching span
	httpGetSink.Reset()
	defaultSink.Reset()

	tdPost := ptrace.NewTraces()
	rs = tdPost.ResourceSpans().AppendEmpty()
	ss = rs.ScopeSpans().AppendEmpty()
	span = ss.Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "POST")

	err = conn.ConsumeTraces(context.Background(), tdPost)
	require.NoError(t, err)
	assert.Equal(t, 0, httpGetSink.SpanCount())
	assert.Equal(t, 1, defaultSink.SpanCount())
}

func TestMetricContextInferenceIntegration(t *testing.T) {
	// Test that metric context inference works for actual routing
	routeTable := []RoutingTableItem{
		{
			Condition: `metric.name == "http_requests_total"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalMetrics, "http")},
		},
	}

	httpSink := new(consumertest.MetricsSink)
	defaultSink := new(consumertest.MetricsSink)

	metricsRouter := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
		pipeline.NewIDWithName(pipeline.SignalMetrics, "http"):    httpSink,
		pipeline.NewIDWithName(pipeline.SignalMetrics, "default"): defaultSink,
	})

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Table = routeTable
	cfg.DefaultPipelines = []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalMetrics, "default")}

	conn, err := newMetricsConnector(connector.Settings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}, cfg, metricsRouter)
	require.NoError(t, err)

	// Create metric with matching name
	mdHttp := pmetric.NewMetrics()
	rm := mdHttp.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("http_requests_total")
	m.SetEmptyGauge().DataPoints().AppendEmpty()

	err = conn.ConsumeMetrics(context.Background(), mdHttp)
	require.NoError(t, err)
	assert.Len(t, httpSink.AllMetrics(), 1)
	assert.Empty(t, defaultSink.AllMetrics())

	// Reset and test non-matching metric
	httpSink.Reset()
	defaultSink.Reset()

	mdOther := pmetric.NewMetrics()
	rm = mdOther.ResourceMetrics().AppendEmpty()
	sm = rm.ScopeMetrics().AppendEmpty()
	m = sm.Metrics().AppendEmpty()
	m.SetName("other_metric")
	m.SetEmptyGauge().DataPoints().AppendEmpty()

	err = conn.ConsumeMetrics(context.Background(), mdOther)
	require.NoError(t, err)
	assert.Empty(t, httpSink.AllMetrics())
	assert.Len(t, defaultSink.AllMetrics(), 1)
}

func TestStatementCountMismatchError(t *testing.T) {
	// Verify that the error type can be used with errors.Is for testing purposes.
	// This error is a safeguard in singleStatementConverter that triggers if
	// somehow multiple statements are passed to the parser (which shouldn't
	// happen in normal routing connector usage).
	err := fmt.Errorf("%w: got %d", errStatementCountMismatch, 2)
	assert.ErrorIs(t, err, errStatementCountMismatch)
	assert.Contains(t, err.Error(), "expected exactly one statement")
	assert.Contains(t, err.Error(), "got 2")
}
