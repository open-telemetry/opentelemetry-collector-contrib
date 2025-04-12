// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package roundrobinconnector

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/roundrobinconnector/internal/metadata"
)

func newPipelineMap[T any](signal pipeline.Signal, consumers ...T) map[pipeline.ID]T {
	ret := make(map[pipeline.ID]T, len(consumers))
	for i, cons := range consumers {
		ret[pipeline.NewIDWithName(signal, strconv.Itoa(i))] = cons
	}
	return ret
}

func TestLogsRoundRobin(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)

	ctx := context.Background()
	set := connectortest.NewNopSettings(metadata.Type)
	host := componenttest.NewNopHost()

	sink1 := new(consumertest.LogsSink)
	sink2 := new(consumertest.LogsSink)
	sink3 := new(consumertest.LogsSink)
	logs, err := f.CreateLogsToLogs(ctx, set, cfg, connector.NewLogsRouter(newPipelineMap[consumer.Logs](pipeline.SignalLogs, sink1, sink2, sink3)))
	assert.NoError(t, err)
	assert.NotNil(t, logs)

	assert.NoError(t, logs.Start(ctx, host))

	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))

	assert.Len(t, sink1.AllLogs(), 1)
	assert.Len(t, sink2.AllLogs(), 1)
	assert.Len(t, sink3.AllLogs(), 1)

	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, logs.ConsumeLogs(ctx, plog.NewLogs()))

	assert.Len(t, sink1.AllLogs(), 2)
	assert.Len(t, sink2.AllLogs(), 2)
	assert.Len(t, sink3.AllLogs(), 2)

	assert.NoError(t, logs.Shutdown(ctx))
}

func TestMetricsRoundRobin(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)

	ctx := context.Background()
	set := connectortest.NewNopSettings(metadata.Type)
	host := componenttest.NewNopHost()

	sink1 := new(consumertest.MetricsSink)
	sink2 := new(consumertest.MetricsSink)
	sink3 := new(consumertest.MetricsSink)
	metrics, err := f.CreateMetricsToMetrics(ctx, set, cfg, connector.NewMetricsRouter(newPipelineMap[consumer.Metrics](pipeline.SignalMetrics, sink1, sink2, sink3)))
	assert.NoError(t, err)
	assert.NotNil(t, metrics)

	assert.NoError(t, metrics.Start(ctx, host))

	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))

	assert.Len(t, sink1.AllMetrics(), 1)
	assert.Len(t, sink2.AllMetrics(), 1)
	assert.Len(t, sink3.AllMetrics(), 1)

	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, metrics.ConsumeMetrics(ctx, pmetric.NewMetrics()))

	assert.Len(t, sink1.AllMetrics(), 2)
	assert.Len(t, sink2.AllMetrics(), 2)
	assert.Len(t, sink3.AllMetrics(), 2)

	assert.NoError(t, metrics.Shutdown(ctx))
}

func TestTracesRoundRobin(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)

	ctx := context.Background()
	set := connectortest.NewNopSettings(metadata.Type)
	host := componenttest.NewNopHost()

	sink1 := new(consumertest.TracesSink)
	sink2 := new(consumertest.TracesSink)
	sink3 := new(consumertest.TracesSink)
	traces, err := f.CreateTracesToTraces(ctx, set, cfg, connector.NewTracesRouter(newPipelineMap[consumer.Traces](pipeline.SignalTraces, sink1, sink2, sink3)))
	assert.NoError(t, err)
	assert.NotNil(t, traces)

	assert.NoError(t, traces.Start(ctx, host))

	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Len(t, sink1.AllTraces(), 1)
	assert.Len(t, sink2.AllTraces(), 1)
	assert.Len(t, sink3.AllTraces(), 1)

	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, traces.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Len(t, sink1.AllTraces(), 2)
	assert.Len(t, sink2.AllTraces(), 2)
	assert.Len(t, sink3.AllTraces(), 2)

	assert.NoError(t, traces.Shutdown(ctx))
}
