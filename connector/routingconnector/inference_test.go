// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
)

// routerBuilders maps signal types to functions that create routers and return (context, routeCount, error).
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

func TestContextInference(t *testing.T) {
	testCases := []struct {
		name            string
		condition       string
		context         string // explicit context, empty means infer
		signal          pipeline.Signal
		expectedContext string
	}{
		// Logs context inference
		{"logs/qualified resource path", `resource.attributes["env"] == "prod"`, "", pipeline.SignalLogs, "resource"},
		{"logs/unqualified attributes defaults to resource", `attributes["env"] == "dev"`, "", pipeline.SignalLogs, "resource"},
		{"logs/qualified log.body infers log context", `log.body == "something"`, "", pipeline.SignalLogs, "log"},
		{"logs/qualified log.attributes infers log context", `log.attributes["level"] == "error"`, "", pipeline.SignalLogs, "log"},
		// Without explicit context, attributes["level"] would infer to "resource" due to priority ordering.
		// Explicit "log" context overrides this.
		{"logs/explicit context overrides inference", `attributes["level"] == "error"`, "log", pipeline.SignalLogs, "log"},

		// Traces context inference
		{"traces/qualified span.attributes infers span context", `span.attributes["http.method"] == "GET"`, "", pipeline.SignalTraces, "span"},
		{"traces/qualified span.name infers span context", `span.name == "HTTP GET"`, "", pipeline.SignalTraces, "span"},
		{"traces/qualified resource path", `resource.attributes["service.name"] == "frontend"`, "", pipeline.SignalTraces, "resource"},
		{"traces/unqualified attributes defaults to resource", `attributes["env"] == "prod"`, "", pipeline.SignalTraces, "resource"},
		{"traces/explicit context overrides inference", `attributes["http.method"] == "GET"`, "span", pipeline.SignalTraces, "span"},

		// Metrics context inference
		{"metrics/qualified metric.name infers metric context", `metric.name == "http_requests_total"`, "", pipeline.SignalMetrics, "metric"},
		{"metrics/qualified datapoint.attributes infers datapoint context", `datapoint.attributes["host"] == "server1"`, "", pipeline.SignalMetrics, "datapoint"},
		{"metrics/qualified resource path", `resource.attributes["service.name"] == "backend"`, "", pipeline.SignalMetrics, "resource"},
		{"metrics/explicit context overrides inference", `attributes["host"] == "server1"`, "datapoint", pipeline.SignalMetrics, "datapoint"},
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
		{Condition: `resource.attributes["env"] == "prod"`, Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "0")}},
		{Condition: `log.severity_text == "ERROR"`, Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "1")}},
		{Context: "request", Condition: `request["X-Tenant"] == "acme"`, Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "2")}},
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

func TestStatementCountMismatchError(t *testing.T) {
	err := fmt.Errorf("%w: got %d", errStatementCountMismatch, 2)
	assert.ErrorIs(t, err, errStatementCountMismatch)
	assert.Contains(t, err.Error(), "expected exactly one statement")
	assert.Contains(t, err.Error(), "got 2")
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
