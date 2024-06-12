// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package roundrobinconnector

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func newPipelineMap[T any](tp component.Type, consumers ...T) map[component.ID]T {
	ret := make(map[component.ID]T, len(consumers))
	for i, cons := range consumers {
		ret[component.NewIDWithName(tp, strconv.Itoa(i))] = cons
	}
	return ret
}

func TestLogsRoundRobin(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)

	ctx := context.Background()
	set := connectortest.NewNopSettings()
	host := componenttest.NewNopHost()

	sink1 := new(consumertest.LogsSink)
	sink2 := new(consumertest.LogsSink)
	sink3 := new(consumertest.LogsSink)
	logs, err := f.CreateLogsToLogs(ctx, set, cfg, connector.NewLogsRouter(newPipelineMap[consumer.Logs](component.DataTypeLogs, sink1, sink2, sink3)))
	assert.NoError(t, err)
	assert.NotNil(t, logs)

	assert.NoError(t, logs.Start(ctx, host))

	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))

	assert.Equal(t, 1, len(sink1.AllLogs()))
	assert.Equal(t, 1, len(sink2.AllLogs()))
	assert.Equal(t, 1, len(sink3.AllLogs()))

	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))

	assert.Equal(t, 2, len(sink1.AllLogs()))
	assert.Equal(t, 2, len(sink2.AllLogs()))
	assert.Equal(t, 2, len(sink3.AllLogs()))

	assert.NoError(t, logs.Shutdown(ctx))
}

func TestMetricsRoundRobin(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)

	ctx := context.Background()
	set := connectortest.NewNopSettings()
	host := componenttest.NewNopHost()

	sink1 := new(consumertest.MetricsSink)
	sink2 := new(consumertest.MetricsSink)
	sink3 := new(consumertest.MetricsSink)
	metrics, err := f.CreateMetricsToMetrics(ctx, set, cfg, connector.NewMetricsRouter(newPipelineMap[consumer.Metrics](component.DataTypeMetrics, sink1, sink2, sink3)))
	assert.NoError(t, err)
	assert.NotNil(t, metrics)

	assert.NoError(t, metrics.Start(ctx, host))

	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))

	assert.Equal(t, 1, len(sink1.AllMetrics()))
	assert.Equal(t, 1, len(sink2.AllMetrics()))
	assert.Equal(t, 1, len(sink3.AllMetrics()))

	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))

	assert.Equal(t, 2, len(sink1.AllMetrics()))
	assert.Equal(t, 2, len(sink2.AllMetrics()))
	assert.Equal(t, 2, len(sink3.AllMetrics()))

	assert.NoError(t, metrics.Shutdown(ctx))
}

func TestTracesRoundRobin(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)

	ctx := context.Background()
	set := connectortest.NewNopSettings()
	host := componenttest.NewNopHost()

	sink1 := new(consumertest.TracesSink)
	sink2 := new(consumertest.TracesSink)
	sink3 := new(consumertest.TracesSink)
	traces, err := f.CreateTracesToTraces(ctx, set, cfg, connector.NewTracesRouter(newPipelineMap[consumer.Traces](component.DataTypeTraces, sink1, sink2, sink3)))
	assert.NoError(t, err)
	assert.NotNil(t, traces)

	assert.NoError(t, traces.Start(ctx, host))

	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 1, len(sink1.AllTraces()))
	assert.Equal(t, 1, len(sink2.AllTraces()))
	assert.Equal(t, 1, len(sink3.AllTraces()))

	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 2, len(sink1.AllTraces()))
	assert.Equal(t, 2, len(sink2.AllTraces()))
	assert.Equal(t, 2, len(sink3.AllTraces()))

	assert.NoError(t, traces.Shutdown(ctx))
}
