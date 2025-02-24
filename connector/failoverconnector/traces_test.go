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
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
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
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

	failoverConnector := conn.(*tracesFailover)
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	require.NoError(t, err)
	require.NotNil(t, conn)

	tc := failoverConnector.failover.getConsumerAtIndex(0)
	tc1 := failoverConnector.failover.TestGetConsumerAtIndex(1)
	tc2 := failoverConnector.failover.TestGetConsumerAtIndex(2)

	require.Equal(t, tc, &sinkFirst)
	require.Equal(t, tc1, &sinkSecond)
	require.Equal(t, tc2, &sinkThird)
}

func TestTracesWithValidFailover(t *testing.T) {
	var sinkSecond, sinkThird consumertest.TracesSink

	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")
	noOp := consumertest.NewNop()

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    50 * time.Millisecond,
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  noOp,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

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
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/third")
	noOp := consumertest.NewNop()

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    50 * time.Millisecond,
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  noOp,
		tracesSecond: &sinkSecond,
		tracesThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

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

func consumeTracesAndCheckStable(conn *tracesFailover, idx int, tr ptrace.Traces) bool {
	_ = conn.ConsumeTraces(context.Background(), tr)
	stableIndex := conn.failover.pS.CurrentPipeline()
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
