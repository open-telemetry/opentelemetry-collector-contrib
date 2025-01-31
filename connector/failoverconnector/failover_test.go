// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"
import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
)

func TestFailoverRecovery(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird, sinkFourth consumertest.TracesSink
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")
	tracesFourth := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/fourth")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}, {tracesFourth}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
		tracesFourth: &sinkFourth,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)

	tr := sampleTrace()

	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	t.Run("single failover recovery to primary consumer: level 2 -> 1", func(t *testing.T) {
		defer func() {
			resetConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))
		idx := failoverConnector.failover.pS.TestStableIndex()
		require.Equal(t, 1, idx)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 0, tr)
		}, 3*time.Second, 5*time.Millisecond)
	})

	t.Run("single failover and stays current", func(t *testing.T) {
		defer func() {
			resetConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))
		idx := failoverConnector.failover.pS.TestStableIndex()
		require.Equal(t, 1, idx)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 0, tr)
		}, 3*time.Second, 5*time.Millisecond)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckCurrent(failoverConnector, 0, tr)
		}, 3*time.Second, 5*time.Millisecond)
	})

	t.Run("double failover recovery: level 3 -> 2 -> 1", func(t *testing.T) {
		defer func() {
			resetConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))
		idx := failoverConnector.failover.pS.TestStableIndex()
		require.Equal(t, 2, idx)

		// Simulate recovery of exporter
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 1, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 0, tr)
		}, 3*time.Second, 5*time.Millisecond)
	})

	t.Run("multiple failover recovery: level 3 -> 2 -> 4 -> 3 -> 1", func(t *testing.T) {
		defer func() {
			resetConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 2, tr)
		}, 3*time.Second, 5*time.Millisecond)

		// Simulate recovery of exporter
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 1, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errTracesConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 3, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(2, &sinkThird)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 2, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkThird)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 0, tr)
		}, 3*time.Second, 5*time.Millisecond)
	})
}

func TestFailoverRecovery_MaxRetries(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird, sinkFourth consumertest.TracesSink
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")
	tracesFourth := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/fourth")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}, {tracesFourth}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
		tracesFourth: &sinkFourth,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)

	tr := sampleTrace()

	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))

	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 2, tr)
	}, 3*time.Second, 5*time.Millisecond)

	failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)
	failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 0, tr)
	}, 3*time.Second, 5*time.Millisecond)

	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))
	failoverConnector.failover.pS.SetRetryCountToValue(0, cfg.MaxRetries)

	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 2, tr)
	}, 3*time.Second, 5*time.Millisecond)

	failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)
	failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

	// Check that level 0 is skipped because max retry value is hit
	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 1, tr)
	}, 3*time.Second, 5*time.Millisecond)
}

func TestFailoverRecovery_MaxRetriesDisabled(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird, sinkFourth consumertest.TracesSink
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")
	tracesFourth := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/fourth")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}, {tracesFourth}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       0,
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
		tracesFourth: &sinkFourth,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)

	tr := sampleTrace()

	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))

	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 2, tr)
	}, 3*time.Second, 5*time.Millisecond)

	failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)
	failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 0, tr)
	}, 3*time.Second, 5*time.Millisecond)

	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))
	failoverConnector.failover.pS.SetRetryCountToValue(0, cfg.MaxRetries)

	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 2, tr)
	}, 3*time.Second, 5*time.Millisecond)

	failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)
	failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

	// Check that still resets to level 0 even though max retry value is hit
	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 0, tr)
	}, 3*time.Second, 5*time.Millisecond)
}

func TestFailoverRecovery_RetryBackoff(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird, sinkFourth consumertest.TracesSink
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")
	tracesFourth := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/fourth")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}, {tracesFourth}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       1,
		RetryBackoff:     10 * time.Millisecond,
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
		tracesFourth: &sinkFourth,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)

	tr := sampleTrace()

	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))

	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 1, tr)
	}, 3*time.Second, 5*time.Millisecond)

	go func() {
		time.Sleep(100 * time.Millisecond)
		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)
	}()

	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 0, tr)
	}, 3*time.Second, 5*time.Millisecond)
}

func resetConsumers(conn *tracesFailover, consumers ...consumer.Traces) {
	for i, sink := range consumers {
		conn.failover.ModifyConsumerAtIndex(i, sink)
	}
	conn.failover.pS.TestSetStableIndex(0)
}
