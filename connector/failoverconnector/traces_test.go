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
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
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
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
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

	require.NoError(t, conn.ConsumeTraces(context.Background(), tr))
	_, ch, ok := failoverConnector.failover.getCurrentConsumer()
	idx := failoverConnector.failover.pS.ChannelIndex(ch)
	assert.True(t, ok)
	require.Equal(t, idx, 1)
}

func TestTracesWithFailoverError(t *testing.T) {
	var sinkSecond, sinkThird consumertest.TracesSink
	tracesFirst := component.NewIDWithName(component.DataTypeTraces, "traces/first")
	tracesSecond := component.NewIDWithName(component.DataTypeTraces, "traces/second")
	tracesThird := component.NewIDWithName(component.DataTypeTraces, "traces/third")
	noOp := consumertest.NewNop()

	cfg := &Config{
		PipelinePriority: [][]component.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
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

func TestTracesWithFailoverRecovery(t *testing.T) {
	t.Skip("Flaky Test - See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31005")
	var sinkSecond, sinkThird consumertest.TracesSink
	tracesFirst := component.NewIDWithName(component.DataTypeTraces, "traces/first")
	tracesSecond := component.NewIDWithName(component.DataTypeTraces, "traces/second")
	tracesThird := component.NewIDWithName(component.DataTypeTraces, "traces/third")
	noOp := consumertest.NewNop()

	cfg := &Config{
		PipelinePriority: [][]component.ID{{tracesFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       1000,
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

	require.NoError(t, conn.ConsumeTraces(context.Background(), tr))
	_, ch, ok := failoverConnector.failover.getCurrentConsumer()
	idx := failoverConnector.failover.pS.ChannelIndex(ch)

	assert.True(t, ok)
	require.Equal(t, idx, 1)

	// Simulate recovery of exporter
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewNop())

	require.Eventually(t, func() bool {
		_, ch, ok = failoverConnector.failover.getCurrentConsumer()
		idx = failoverConnector.failover.pS.ChannelIndex(ch)
		return ok && idx == 0
	}, 3*time.Second, 100*time.Millisecond)
}

func sampleTrace() ptrace.Traces {
	tr := ptrace.NewTraces()
	rl := tr.ResourceSpans().AppendEmpty()
	rl.Resource().Attributes().PutStr("conn", "failover")
	span := rl.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("SampleSpan")
	return tr
}
