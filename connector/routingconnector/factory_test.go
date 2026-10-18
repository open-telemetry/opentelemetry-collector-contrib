// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlotelcol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

func createTestFuncFactory[K any](name string) ottl.Factory[K] {
	type testFuncArguments[K any] struct{}
	createFunc := func(_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[K], error) {
		return func(context.Context, K) (any, error) {
			return true, nil
		}, nil
	}
	return ottl.NewFactory(name, &testFuncArguments[K]{}, createFunc)
}

func TestConnectorCreatedWithValidConfiguration(t *testing.T) {
	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where attributes["X-Tenant"] == "acme"`,
			Pipelines: []pipeline.ID{
				pipeline.NewIDWithName(pipeline.SignalTraces, "0"),
			},
		}},
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		pipeline.NewIDWithName(pipeline.SignalTraces, "default"): consumertest.NewNop(),
		pipeline.NewIDWithName(pipeline.SignalTraces, "0"):       consumertest.NewNop(),
	})

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestCreationFailsWithIncorrectConsumer(t *testing.T) {
	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where attributes["X-Tenant"] == "acme"`,
			Pipelines: []pipeline.ID{
				pipeline.NewIDWithName(pipeline.SignalTraces, "0"),
			},
		}},
	}

	// in the real world, the factory will always receive a consumer with a concrete type of a
	// connector router. this tests failure when a consumer of another type is passed in.
	consumer := &consumertest.TracesSink{}

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, consumer)

	assert.ErrorIs(t, err, errUnexpectedConsumer)
	assert.Nil(t, conn)
}

func TestDefaultErrorModeWithFeatureGate(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, ottl.IgnoreError, cfg.(*Config).ErrorMode)

	t.Cleanup(func() {
		_ = featuregate.GlobalRegistry().Set(metadata.ConnectorRoutingDefaultErrorModeIgnoreFeatureGate.ID(), true)
	})

	err := featuregate.GlobalRegistry().Set(metadata.ConnectorRoutingDefaultErrorModeIgnoreFeatureGate.ID(), false)
	require.NoError(t, err)

	cfg = factory.CreateDefaultConfig()
	assert.Equal(t, ottl.PropagateError, cfg.(*Config).ErrorMode)
}

func Test_FactoryWithFunctions_CreateTracesToTraces(t *testing.T) {
	errCustomRouteFunctionInvoked := errors.New("custom route function invoked")
	createFailingRouteOverrideFactory := func() ottl.Factory[*ottlresource.TransformContext] {
		return ottl.NewFactory(routeFunctionName, nil, func(ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[*ottlresource.TransformContext], error) {
			return nil, errCustomRouteFunctionInvoked
		})
	}

	tests := []struct {
		name           string
		table          []RoutingTableItem
		factoryOptions []FactoryOption
		wantErrorWith  string
	}{
		{
			name: "with resource functions: condition with added resource func",
			table: []RoutingTableItem{{
				Context:   "resource",
				Condition: `TestResourceFunc() and attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "0")},
			}},
			factoryOptions: []FactoryOption{
				WithResourceFunctions([]ottl.Factory[*ottlresource.TransformContext]{createTestFuncFactory[*ottlresource.TransformContext]("TestResourceFunc")}),
			},
		},
		{
			name: "with resource functions: condition with missing custom func",
			table: []RoutingTableItem{{
				Context:   "resource",
				Condition: `TestResourceFunc() and attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "0")},
			}},
			wantErrorWith: `undefined function "TestResourceFunc"`,
		},
		{
			name: "with span functions: condition with added span func",
			table: []RoutingTableItem{{
				Context:   "span",
				Condition: `TestSpanFunc() and attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "0")},
			}},
			factoryOptions: []FactoryOption{
				WithSpanFunctions([]ottl.Factory[*ottlspan.TransformContext]{createTestFuncFactory[*ottlspan.TransformContext]("TestSpanFunc")}),
			},
		},
		{
			name: "with span functions: condition with missing custom func",
			table: []RoutingTableItem{{
				Context:   "span",
				Condition: `TestSpanFunc() and attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "0")},
			}},
			wantErrorWith: `undefined function "TestSpanFunc"`,
		},
		{
			name: "with otelcol functions: condition with added otelcol func",
			table: []RoutingTableItem{{
				Context:   "otelcol",
				Condition: `TestOtelcolFunc() and client.metadata["foo"][0] == "bar"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "0")},
			}},
			factoryOptions: []FactoryOption{
				WithOtelColFunctions([]ottl.Factory[*ottlotelcol.TransformContext]{createTestFuncFactory[*ottlotelcol.TransformContext]("TestOtelcolFunc")}),
			},
		},
		{
			name: "with resource functions overridden: the route function remains available",
			table: []RoutingTableItem{{
				Context:   "resource",
				Condition: `attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "0")},
			}},
			factoryOptions: []FactoryOption{
				WithResourceFunctions([]ottl.Factory[*ottlresource.TransformContext]{createTestFuncFactory[*ottlresource.TransformContext]("TestResourceFunc")}),
			},
		},
		{
			name: "with resource functions: a custom route function replaces the default one",
			table: []RoutingTableItem{{
				Context:   "resource",
				Condition: `attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "0")},
			}},
			factoryOptions: []FactoryOption{
				WithResourceFunctions([]ottl.Factory[*ottlresource.TransformContext]{createFailingRouteOverrideFactory()}),
			},
			wantErrorWith: errCustomRouteFunctionInvoked.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactoryWithOptions(tt.factoryOptions...)
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Table = tt.table

			router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
				pipeline.NewIDWithName(pipeline.SignalTraces, "default"): consumertest.NewNop(),
				pipeline.NewIDWithName(pipeline.SignalTraces, "0"):       consumertest.NewNop(),
			})

			conn, err := factory.CreateTracesToTraces(t.Context(),
				connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))
			if tt.wantErrorWith != "" {
				assert.ErrorContains(t, err, tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, conn)
		})
	}
}

func Test_FactoryWithFunctions_LogsWhenOverridden(t *testing.T) {
	tests := []struct {
		name           string
		factoryOptions []FactoryOption
		wantLog        bool
	}{
		{
			name:           "no functions overridden: nothing logged",
			factoryOptions: nil,
			wantLog:        false,
		},
		{
			name: "resource functions overridden: logged",
			factoryOptions: []FactoryOption{
				WithResourceFunctions([]ottl.Factory[*ottlresource.TransformContext]{createTestFuncFactory[*ottlresource.TransformContext]("TestResourceFunc")}),
			},
			wantLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedZapCore, observedLogs := observer.New(zap.DebugLevel)
			settings := connectortest.NewNopSettings(metadata.Type)
			settings.Logger = zap.New(observedZapCore)

			factory := NewFactoryWithOptions(tt.factoryOptions...)
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Table = []RoutingTableItem{{
				Context:   "resource",
				Condition: `resource.attributes["X-Tenant"] == "acme"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalTraces, "0")},
			}}

			router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
				pipeline.NewIDWithName(pipeline.SignalTraces, "default"): consumertest.NewNop(),
				pipeline.NewIDWithName(pipeline.SignalTraces, "0"):       consumertest.NewNop(),
			})

			_, err := factory.CreateTracesToTraces(t.Context(), settings, cfg, router.(consumer.Traces))
			require.NoError(t, err)

			if !tt.wantLog {
				assert.Equal(t, 0, observedLogs.Len())
				return
			}
			require.Equal(t, 1, observedLogs.Len())
			entry := observedLogs.All()[0]
			assert.Equal(t, zap.DebugLevel, entry.Level)
			assert.Contains(t, entry.Message, `non-default OTTL functions have been registered in the "routing" connector`)
			assert.Equal(t, true, entry.ContextMap()["resource"])
		})
	}
}

func Test_FactoryWithFunctions_CreateMetricsToMetrics(t *testing.T) {
	tests := []struct {
		name           string
		table          []RoutingTableItem
		factoryOptions []FactoryOption
		wantErrorWith  string
	}{
		{
			name: "with metric functions: condition with added metric func",
			table: []RoutingTableItem{{
				Context:   "metric",
				Condition: `TestMetricFunc() and metric.name == "http_requests_total"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalMetrics, "0")},
			}},
			factoryOptions: []FactoryOption{
				WithMetricFunctions([]ottl.Factory[*ottlmetric.TransformContext]{createTestFuncFactory[*ottlmetric.TransformContext]("TestMetricFunc")}),
			},
		},
		{
			name: "with metric functions: condition with missing custom func",
			table: []RoutingTableItem{{
				Context:   "metric",
				Condition: `TestMetricFunc() and metric.name == "http_requests_total"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalMetrics, "0")},
			}},
			wantErrorWith: `undefined function "TestMetricFunc"`,
		},
		{
			name: "with datapoint functions: condition with added datapoint func",
			table: []RoutingTableItem{{
				Context:   "datapoint",
				Condition: `TestDataPointFunc() and datapoint.attributes["host"] == "server1"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalMetrics, "0")},
			}},
			factoryOptions: []FactoryOption{
				WithDataPointFunctions([]ottl.Factory[*ottldatapoint.TransformContext]{createTestFuncFactory[*ottldatapoint.TransformContext]("TestDataPointFunc")}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactoryWithOptions(tt.factoryOptions...)
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Table = tt.table

			router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
				pipeline.NewIDWithName(pipeline.SignalMetrics, "default"): consumertest.NewNop(),
				pipeline.NewIDWithName(pipeline.SignalMetrics, "0"):       consumertest.NewNop(),
			})

			conn, err := factory.CreateMetricsToMetrics(t.Context(),
				connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Metrics))
			if tt.wantErrorWith != "" {
				assert.ErrorContains(t, err, tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, conn)
		})
	}
}

func Test_FactoryWithFunctions_CreateLogsToLogs(t *testing.T) {
	tests := []struct {
		name           string
		table          []RoutingTableItem
		factoryOptions []FactoryOption
		wantErrorWith  string
	}{
		{
			name: "with log functions: condition with added log func",
			table: []RoutingTableItem{{
				Context:   "log",
				Condition: `TestLogFunc() and log.severity_text == "ERROR"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "0")},
			}},
			factoryOptions: []FactoryOption{
				WithLogFunctions([]ottl.Factory[*ottllog.TransformContext]{createTestFuncFactory[*ottllog.TransformContext]("TestLogFunc")}),
			},
		},
		{
			name: "with log functions: condition with missing custom func",
			table: []RoutingTableItem{{
				Context:   "log",
				Condition: `TestLogFunc() and log.severity_text == "ERROR"`,
				Pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "0")},
			}},
			wantErrorWith: `undefined function "TestLogFunc"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactoryWithOptions(tt.factoryOptions...)
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Table = tt.table

			router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
				pipeline.NewIDWithName(pipeline.SignalLogs, "default"): consumertest.NewNop(),
				pipeline.NewIDWithName(pipeline.SignalLogs, "0"):       consumertest.NewNop(),
			})

			conn, err := factory.CreateLogsToLogs(t.Context(),
				connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Logs))
			if tt.wantErrorWith != "" {
				assert.ErrorContains(t, err, tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, conn)
		})
	}
}
