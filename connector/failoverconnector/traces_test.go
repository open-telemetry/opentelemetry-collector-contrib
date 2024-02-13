// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"
import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var errTracesConsumer = errors.New("Error from ConsumeTraces")

func TestTracesRegisterConsumers(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.TracesSink
	tracesFirst := component.NewIDWithName(component.DataTypeTraces, "traces/first")
	tracesSecond := component.NewIDWithName(component.DataTypeTraces, "traces/second")
	tracesThird := component.NewIDWithName(component.DataTypeTraces, "traces/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    25 * time.Millisecond,
		RetryGap:         5 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewTracesRouter(map[component.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Traces))

	failoverConnector := conn.(*tracesFailover)
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	require.NoError(t, err)
	require.NotNil(t, conn)

	tc, _, ok := failoverConnector.failover.getCurrentConsumer()
	tc1 := failoverConnector.failover.GetConsumerAtIndex(1)
	tc2 := failoverConnector.failover.GetConsumerAtIndex(2)

	assert.True(t, ok)
	require.Implements(t, (*consumer.Traces)(nil), tc)
	require.Implements(t, (*consumer.Traces)(nil), tc1)
	require.Implements(t, (*consumer.Traces)(nil), tc2)
}

func TestTracesWithValidFailover(t *testing.T) {
	var sinkSecond, sinkThird consumertest.TracesSink

	tracesFirst := component.NewIDWithName(component.DataTypeTraces, "traces/first")
	tracesSecond := component.NewIDWithName(component.DataTypeTraces, "traces/second")
	tracesThird := component.NewIDWithName(component.DataTypeTraces, "traces/third")
	noOp := consumertest.NewNop()

	cfg := &Config{
		PipelinePriority: [][]component.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    25 * time.Millisecond,
		RetryGap:         5 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewTracesRouter(map[component.ID]consumer.Traces{
		tracesFirst:  noOp,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	tr := sampleTrace()

	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverConnector, 1, tr)
	}, 3*time.Second, 5*time.Millisecond)
}

func TestTracesWithFailoverError(t *testing.T) {
	var sinkSecond, sinkThird consumertest.TracesSink
	tracesFirst := component.NewIDWithName(component.DataTypeTraces, "traces/first")
	tracesSecond := component.NewIDWithName(component.DataTypeTraces, "traces/second")
	tracesThird := component.NewIDWithName(component.DataTypeTraces, "traces/third")
	noOp := consumertest.NewNop()

	cfg := &Config{
		PipelinePriority: [][]component.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    25 * time.Millisecond,
		RetryGap:         5 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewTracesRouter(map[component.ID]consumer.Traces{
		tracesFirst:  noOp,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errTracesConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	tr := sampleTrace()

	assert.EqualError(t, conn.ConsumeTraces(context.Background(), tr), "All provided pipelines return errors")
}

func TestTracesWithRecovery(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird, sinkFourth consumertest.TracesSink
	tracesFirst := component.NewIDWithName(component.DataTypeTraces, "traces/first")
	tracesSecond := component.NewIDWithName(component.DataTypeTraces, "traces/second")
	tracesThird := component.NewIDWithName(component.DataTypeTraces, "traces/third")
	tracesFourth := component.NewIDWithName(component.DataTypeTraces, "traces/fourth")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{tracesFirst}, {tracesSecond}, {tracesThird}, {tracesFourth}},
		RetryInterval:    25 * time.Millisecond,
		RetryGap:         5 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewTracesRouter(map[component.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
		tracesFourth: &sinkFourth,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)

	tr := sampleTrace()

	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	t.Run("single failover recovery to primary consumer: level 2 -> 1", func(t *testing.T) {
		defer func() {
			resetTracesConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))
		idx := failoverConnector.failover.pS.TestStableIndex()
		require.Equal(t, idx, 1)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 0, tr)
		}, 3*time.Second, 5*time.Millisecond)
	})

	t.Run("double failover recovery: level 3 -> 2 -> 1", func(t *testing.T) {
		defer func() {
			resetTracesConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))

		require.NoError(t, conn.ConsumeTraces(context.Background(), tr))
		idx := failoverConnector.failover.pS.TestStableIndex()
		require.Equal(t, idx, 2)

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
			resetTracesConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
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

	t.Run("failover with max retries exceeded: level 3 -> 1 -> 3 -> 1(Skipped due to max retries) -> 2", func(t *testing.T) {
		defer func() {
			resetTracesConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 2, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkSecond)
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 0, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))
		failoverConnector.failover.pS.SetRetryCountToMax(0)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 2, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkSecond)
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(failoverConnector, 1, tr)
		}, 3*time.Second, 5*time.Millisecond)

	})
}

func consumeTracesAndCheckStable(conn *tracesFailover, idx int, tr ptrace.Traces) bool {
	_ = conn.ConsumeTraces(context.Background(), tr)
	stableIndex := conn.failover.pS.TestStableIndex()
	return stableIndex == idx
}

func resetTracesConsumers(conn *tracesFailover, consumers ...consumer.Traces) {
	for i, sink := range consumers {

		conn.failover.ModifyConsumerAtIndex(i, sink)
	}
	conn.failover.pS.TestSetStableIndex(0)
}

func sampleTrace() ptrace.Traces {
	tr := ptrace.NewTraces()
	rl := tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().PutStr("conn", "failover")
	span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("SampleSpan")
	return tr
}
