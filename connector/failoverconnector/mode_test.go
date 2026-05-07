// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

func TestStrategies(t *testing.T) {
	strategies := []struct {
		name     string
		strategy Strategy
	}{
		{name: "default", strategy: Strategy{}},
		{name: "standard", strategy: Strategy{Standard: &StandardConfig{}}},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			t.Run("traces", func(t *testing.T) { runTracesStrategyTest(t, s.strategy) })
			t.Run("metrics", func(t *testing.T) { runMetricsStrategyTest(t, s.strategy) })
			t.Run("logs", func(t *testing.T) { runLogsStrategyTest(t, s.strategy) })
		})
	}
}

func runTracesStrategyTest(t *testing.T, strategy Strategy) {
	t.Helper()

	var sinkFirst, sinkSecond consumertest.TracesSink
	first := pipeline.NewIDWithName(pipeline.SignalTraces, "first")
	second := pipeline.NewIDWithName(pipeline.SignalTraces, "second")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{first}, {second}},
		Strategy:         strategy,
		RetryInterval:    durationPtr(50 * time.Millisecond),
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		first:  &sinkFirst,
		second: &sinkSecond,
	})

	conn, err := NewFactory().CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))
	require.NoError(t, err)
	defer shutdown(t, conn)

	tr := sampleTrace()
	require.NoError(t, conn.ConsumeTraces(t.Context(), tr))
	assert.Len(t, sinkFirst.AllTraces(), 1)
	assert.Empty(t, sinkSecond.AllTraces())

	sinkFirst.Reset()
	sinkSecond.Reset()

	conn.(*tracesFailover).failover.ModifyConsumerAtIndex(0, consumertest.NewErr(assert.AnError))
	require.NoError(t, conn.ConsumeTraces(t.Context(), tr))
	assert.Empty(t, sinkFirst.AllTraces())
	assert.Len(t, sinkSecond.AllTraces(), 1)
}

func runMetricsStrategyTest(t *testing.T, strategy Strategy) {
	t.Helper()

	var sinkFirst, sinkSecond consumertest.MetricsSink
	first := pipeline.NewIDWithName(pipeline.SignalMetrics, "first")
	second := pipeline.NewIDWithName(pipeline.SignalMetrics, "second")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{first}, {second}},
		Strategy:         strategy,
		RetryInterval:    durationPtr(50 * time.Millisecond),
	}

	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
		first:  &sinkFirst,
		second: &sinkSecond,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Metrics))
	require.NoError(t, err)
	defer shutdown(t, conn)

	md := sampleMetric()
	require.NoError(t, conn.ConsumeMetrics(t.Context(), md))
	assert.Len(t, sinkFirst.AllMetrics(), 1)
	assert.Empty(t, sinkSecond.AllMetrics())

	sinkFirst.Reset()
	sinkSecond.Reset()

	conn.(*metricsFailover).failover.ModifyConsumerAtIndex(0, consumertest.NewErr(assert.AnError))
	require.NoError(t, conn.ConsumeMetrics(t.Context(), md))
	assert.Empty(t, sinkFirst.AllMetrics())
	assert.Len(t, sinkSecond.AllMetrics(), 1)
}

func runLogsStrategyTest(t *testing.T, strategy Strategy) {
	t.Helper()

	var sinkFirst, sinkSecond consumertest.LogsSink
	first := pipeline.NewIDWithName(pipeline.SignalLogs, "first")
	second := pipeline.NewIDWithName(pipeline.SignalLogs, "second")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{first}, {second}},
		Strategy:         strategy,
		RetryInterval:    durationPtr(50 * time.Millisecond),
	}

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		first:  &sinkFirst,
		second: &sinkSecond,
	})

	conn, err := NewFactory().CreateLogsToLogs(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Logs))
	require.NoError(t, err)
	defer shutdown(t, conn)

	ld := sampleLog()
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	assert.Len(t, sinkFirst.AllLogs(), 1)
	assert.Empty(t, sinkSecond.AllLogs())

	sinkFirst.Reset()
	sinkSecond.Reset()

	conn.(*logsFailover).failover.ModifyConsumerAtIndex(0, consumertest.NewErr(assert.AnError))
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	assert.Empty(t, sinkFirst.AllLogs())
	assert.Len(t, sinkSecond.AllLogs(), 1)
}

func shutdown(t *testing.T, c component.Component) {
	t.Helper()
	assert.NoError(t, c.Shutdown(t.Context()))
}
