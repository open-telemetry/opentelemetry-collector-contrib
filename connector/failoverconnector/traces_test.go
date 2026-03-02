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
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

var errTracesConsumer = errors.New("Error from ConsumeTraces")

func TestTracesRegisterConsumers(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.TracesSink
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    25 * time.Millisecond,
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

	wrappedConn := conn.(*wrappedTracesConnector)
	failoverRouter := wrappedConn.GetFailoverRouter()
	defer func() {
		assert.NoError(t, wrappedConn.Shutdown(t.Context()))
	}()

	require.NoError(t, err)
	require.NotNil(t, conn)

	tc := failoverRouter.getConsumerAtIndex(0)
	tc1 := failoverRouter.TestGetConsumerAtIndex(1)
	tc2 := failoverRouter.TestGetConsumerAtIndex(2)

	require.Equal(t, tc, &sinkFirst)
	require.Equal(t, tc1, &sinkSecond)
	require.Equal(t, tc2, &sinkThird)
}

func TestTracesWithValidFailover(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.TracesSink

	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    50 * time.Millisecond,
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	wrappedConn := conn.(*wrappedTracesConnector)
	failoverRouter := wrappedConn.GetFailoverRouter()
	failoverRouter.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	defer func() {
		assert.NoError(t, wrappedConn.Shutdown(t.Context()))
	}()

	tr := sampleTrace()

	require.Eventually(t, func() bool {
		return consumeTracesAndCheckStable(failoverRouter, 1, tr)
	}, 3*time.Second, 5*time.Millisecond)
}

func TestTracesWithFailoverError(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.TracesSink
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    50 * time.Millisecond,
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errTracesConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
	}()

	tr := sampleTrace()

	assert.EqualError(t, failoverConnector.ConsumeTraces(t.Context(), tr), "All provided pipelines return errors")
}

func TestTracesWithQueue(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.TracesSink
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    50 * time.Millisecond,
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

	require.NoError(t, err)

	wrappedConn := conn.(*wrappedTracesConnector)
	failoverRouter := wrappedConn.GetFailoverRouter()
	failoverRouter.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	failoverRouter.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))
	failoverRouter.ModifyConsumerAtIndex(2, consumertest.NewErr(errTracesConsumer))
	defer func() {
		assert.NoError(t, wrappedConn.Shutdown(t.Context()))
	}()

	tr := sampleTrace()

	assert.NoError(t, wrappedConn.ConsumeTraces(t.Context(), tr))
}

func consumeTracesAndCheckStable(router *tracesRouter, idx int, tr ptrace.Traces) bool {
	_ = router.Consume(context.Background(), tr)
	stableIndex := router.pS.CurrentPipeline()
	return stableIndex == idx
}

func sampleTrace() ptrace.Traces {
	tr := ptrace.NewTraces()
	rl := tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().PutStr("conn", "failover")
	span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("SampleSpan")
	return tr
}
