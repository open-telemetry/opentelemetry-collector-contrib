// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
)

func TestContextInference(t *testing.T) {
	testCases := []struct {
		name            string
		signal          pipeline.Signal
		context         string // explicit context, empty means infer
		expectedContext string
		condition       string
	}{
		// Logs context inference
		{
			name:            "logs/qualified resource path",
			condition:       `resource.attributes["env"] == "prod"`,
			signal:          pipeline.SignalLogs,
			context:         "",
			expectedContext: "resource",
		},
		{
			name:            "logs/unqualified attributes defaults to resource",
			condition:       `attributes["env"] == "dev"`,
			signal:          pipeline.SignalLogs,
			context:         "",
			expectedContext: "resource",
		},
		{
			name:            "logs/qualified log.body infers log context",
			condition:       `log.body == "something"`,
			signal:          pipeline.SignalLogs,
			context:         "",
			expectedContext: "log",
		},
		{
			name:            "logs/qualified log.attributes infers log context",
			condition:       `log.attributes["level"] == "error"`,
			signal:          pipeline.SignalLogs,
			context:         "",
			expectedContext: "log",
		},
		// Without explicit context, attributes["level"] would infer to "resource" due to priority ordering.
		// Explicit "log" context overrides this.
		{
			name:            "logs/explicit context overrides inference",
			condition:       `attributes["level"] == "error"`,
			signal:          pipeline.SignalLogs,
			context:         "log",
			expectedContext: "log",
		},

		// Traces context inference
		{
			name:            "traces/qualified span.attributes infers span context",
			condition:       `span.attributes["http.method"] == "GET"`,
			signal:          pipeline.SignalTraces,
			context:         "",
			expectedContext: "span",
		},
		{
			name:            "traces/qualified span.name infers span context",
			condition:       `span.name == "HTTP GET"`,
			signal:          pipeline.SignalTraces,
			context:         "",
			expectedContext: "span",
		},
		{
			name:            "traces/qualified resource path",
			condition:       `resource.attributes["service.name"] == "frontend"`,
			signal:          pipeline.SignalTraces,
			context:         "",
			expectedContext: "resource",
		},
		{
			name:            "traces/unqualified attributes defaults to resource",
			condition:       `attributes["env"] == "prod"`,
			signal:          pipeline.SignalTraces,
			context:         "",
			expectedContext: "resource",
		},
		{
			name:            "traces/explicit context overrides inference",
			condition:       `attributes["http.method"] == "GET"`,
			signal:          pipeline.SignalTraces,
			context:         "span",
			expectedContext: "span",
		},

		// Metrics context inference
		{
			name:            "metrics/qualified metric.name infers metric context",
			condition:       `metric.name == "http_requests_total"`,
			signal:          pipeline.SignalMetrics,
			context:         "",
			expectedContext: "metric",
		},
		{
			name:            "metrics/qualified datapoint.attributes infers datapoint context",
			condition:       `datapoint.attributes["host"] == "server1"`,
			signal:          pipeline.SignalMetrics,
			context:         "",
			expectedContext: "datapoint",
		},
		{
			name:            "metrics/qualified resource path",
			condition:       `resource.attributes["service.name"] == "backend"`,
			signal:          pipeline.SignalMetrics,
			context:         "",
			expectedContext: "resource",
		},
		{
			name:            "metrics/explicit context overrides inference",
			condition:       `attributes["host"] == "server1"`,
			signal:          pipeline.SignalMetrics,
			context:         "datapoint",
			expectedContext: "datapoint",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			routeTable := []RoutingTableItem{{
				Context:   tc.context,
				Condition: tc.condition,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(tc.signal, "test")},
			}}
			actualContext, routeCount, err := routerBuilders[tc.signal](routeTable)
			require.NoError(t, err)
			require.Equal(t, 1, routeCount)
			assert.Equal(t, tc.expectedContext, actualContext)
		})
	}
}

func TestMixedContextsInRoutingTable(t *testing.T) {
	routeTable := []RoutingTableItem{
		{
			Condition: `resource.attributes["env"] == "prod"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "0")},
		},
		{
			Condition: `log.severity_text == "ERROR"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "1")},
		},
		{
			Context:   "request",
			Condition: `request["X-Tenant"] == "acme"`,
			Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "2")},
		},
	}

	sink := new(consumertest.LogsSink)
	router, err := newRouter(routeTable, nil,
		func(...pipeline.ID) (consumer.Logs, error) { return sink, nil },
		componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.Len(t, router.routeSlice, 3)

	assert.Equal(t, "resource", router.routeSlice[0].statementContext)
	assert.Equal(t, "log", router.routeSlice[1].statementContext)
	assert.Equal(t, "request", router.routeSlice[2].statementContext)
}

func TestDuplicateRouteIsIgnored(t *testing.T) {
	// This test verifies that duplicate routing table entries are truly ignored.
	// The original route's consumer should be preserved, not overwritten by the duplicate's.
	originalPipeline := pipeline.NewIDWithName(pipeline.SignalLogs, "original")
	duplicatePipeline := pipeline.NewIDWithName(pipeline.SignalLogs, "duplicate")

	routeTable := []RoutingTableItem{
		{Condition: `resource.attributes["env"] == "prod"`, Pipelines: []pipeline.ID{originalPipeline}},
		{Condition: `resource.attributes["env"] == "prod"`, Pipelines: []pipeline.ID{duplicatePipeline}}, // duplicate
	}

	originalSink := new(consumertest.LogsSink)
	duplicateSink := new(consumertest.LogsSink)

	// Consumer provider returns different sinks based on pipeline ID
	consumerProvider := func(pipelineIDs ...pipeline.ID) (consumer.Logs, error) {
		if len(pipelineIDs) > 0 && pipelineIDs[0] == originalPipeline {
			return originalSink, nil
		}
		return duplicateSink, nil
	}

	router, err := newRouter(routeTable, nil, consumerProvider, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	// Only one route should exist (duplicate was ignored)
	require.Len(t, router.routeSlice, 1)
	require.Len(t, router.routes, 1)

	// The route should use the original pipeline's consumer, not the duplicate's
	// We verify this by checking the route exists and has the expected context
	route := router.routeSlice[0]
	assert.Equal(t, "resource", route.statementContext)

	// The consumer should be the original sink, not the duplicate sink
	// Since we can't directly compare consumers, we verify by using the router
	assert.Same(t, originalSink, route.consumer)
}

// routerBuilders builds a router for each signal type. In the test table, we use this to build a router for each test case
// based on which signal the statement maps to.
var routerBuilders = map[pipeline.Signal]func([]RoutingTableItem) (string, int, error){
	pipeline.SignalLogs: func(routeTable []RoutingTableItem) (string, int, error) {
		sink := new(consumertest.LogsSink)
		router, err := newRouter(routeTable, nil,
			func(...pipeline.ID) (consumer.Logs, error) { return sink, nil },
			componenttest.NewNopTelemetrySettings())
		if err != nil {
			return "", 0, err
		}
		for _, route := range router.routes {
			return route.statementContext, len(router.routes), nil
		}
		return "", 0, nil
	},
	pipeline.SignalTraces: func(routeTable []RoutingTableItem) (string, int, error) {
		sink := new(consumertest.TracesSink)
		router, err := newRouter(routeTable, nil,
			func(...pipeline.ID) (consumer.Traces, error) { return sink, nil },
			componenttest.NewNopTelemetrySettings())
		if err != nil {
			return "", 0, err
		}
		for _, route := range router.routes {
			return route.statementContext, len(router.routes), nil
		}
		return "", 0, nil
	},
	pipeline.SignalMetrics: func(routeTable []RoutingTableItem) (string, int, error) {
		sink := new(consumertest.MetricsSink)
		router, err := newRouter(routeTable, nil,
			func(...pipeline.ID) (consumer.Metrics, error) { return sink, nil },
			componenttest.NewNopTelemetrySettings())
		if err != nil {
			return "", 0, err
		}
		for _, route := range router.routes {
			return route.statementContext, len(router.routes), nil
		}
		return "", 0, nil
	},
}
