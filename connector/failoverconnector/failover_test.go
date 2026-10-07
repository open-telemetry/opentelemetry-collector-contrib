// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"
import (
	"context"
	"runtime"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

func TestTracesReturnToPrimaryAfterImmediateFailure(t *testing.T) {
	// Multiple processors could let the old worker exit before the next failure.
	previousProcs := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previousProcs)

	synctest.Test(t, func(t *testing.T) {
		const retryInterval = 50 * time.Millisecond
		var primarySink, backupSink consumertest.TracesSink
		var failPrimary bool
		primary, err := consumer.NewTraces(func(ctx context.Context, td ptrace.Traces) error {
			if failPrimary {
				return errTracesConsumer
			}
			return primarySink.ConsumeTraces(ctx, td)
		})
		require.NoError(t, err)
		conn := newFailoverTracesConnector(t, primary, &backupSink, retryInterval)

		ctx := t.Context()
		tr := sampleTrace()
		failPrimary = true
		require.NoError(t, conn.ConsumeTraces(ctx, tr))
		require.Equal(t, 1, backupSink.SpanCount())
		require.Zero(t, primarySink.SpanCount())

		failPrimary = false
		synctest.Wait()
		time.Sleep(retryInterval)
		synctest.Wait()
		recoveryErr := conn.ConsumeTraces(ctx, tr)
		failPrimary = true
		failureErr := conn.ConsumeTraces(ctx, tr)
		require.NoError(t, recoveryErr)
		require.NoError(t, failureErr)
		require.Equal(t, 1, primarySink.SpanCount())
		require.Equal(t, 2, backupSink.SpanCount())

		failPrimary = false
		synctest.Wait()
		time.Sleep(retryInterval)
		synctest.Wait()
		require.NoError(t, conn.ConsumeTraces(ctx, tr))
		require.Equal(t, 2, primarySink.SpanCount(), "traces did not return to primary after the second failure")
		require.Equal(t, 2, backupSink.SpanCount())
	})
}

func newFailoverTracesConnector(t *testing.T, primary, backup consumer.Traces, retryInterval time.Duration) connector.Traces {
	t.Helper()
	primaryID := pipeline.NewIDWithName(pipeline.SignalTraces, "primary")
	backupID := pipeline.NewIDWithName(pipeline.SignalTraces, "backup")
	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{primaryID}, {backupID}},
		RetryInterval:    retryInterval,
	}
	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		primaryID: primary,
		backupID:  backup,
	})
	conn, err := NewFactory().CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Shutdown(t.Context()))
	})
	return conn
}

func TestFailoverRecovery(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird, sinkFourth consumertest.TracesSink
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")
	tracesFourth := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/fourth")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}, {tracesFourth}},
		RetryInterval:    50 * time.Millisecond,
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
		tracesFourth: &sinkFourth,
	})

	conn, err := NewFactory().CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)
	tRouter := failoverConnector.failover

	tr := sampleTrace()

	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
	}()

	t.Run("single failover recovery to primary consumer: level 2 -> 1", func(t *testing.T) {
		defer func() {
			resetConsumers(tRouter, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))

		require.NoError(t, conn.ConsumeTraces(t.Context(), tr))
		idx := failoverConnector.failover.TestGetCurrentConsumerIndex()
		require.Equal(t, 1, idx)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(tRouter, 0, tr)
		}, 3*time.Second, 5*time.Millisecond)
	})

	t.Run("double failover recovery: level 3 -> 2 -> 1", func(t *testing.T) {
		defer func() {
			resetConsumers(tRouter, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(tRouter, 2, tr)
		}, 3*time.Second, 5*time.Millisecond)

		// Simulate recovery of exporter
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(tRouter, 1, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(tRouter, 0, tr)
		}, 3*time.Second, 5*time.Millisecond)
	})

	t.Run("multiple failover recovery: level 3 -> 2 -> 4 -> 3 -> 1", func(t *testing.T) {
		defer func() {
			resetConsumers(tRouter, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(tRouter, 2, tr)
		}, 3*time.Second, 5*time.Millisecond)

		// Simulate recovery of exporter
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(tRouter, 1, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errTracesConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(tRouter, 3, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(2, &sinkThird)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(tRouter, 2, tr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkThird)

		require.Eventually(t, func() bool {
			return consumeTracesAndCheckStable(tRouter, 0, tr)
		}, 3*time.Second, 5*time.Millisecond)
	})
}

func resetConsumers(router *tracesRouter, consumers ...consumer.Traces) {
	for i, sink := range consumers {
		router.ModifyConsumerAtIndex(i, sink)
	}
	router.TestSetStableConsumerIndex(0)
}
