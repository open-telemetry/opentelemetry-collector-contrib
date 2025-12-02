package routingconnector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
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
