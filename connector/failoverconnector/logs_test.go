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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
)

var errLogsConsumer = errors.New("Error from ConsumeLogs")

func TestLogsRegisterConsumers(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.LogsSink
	logsFirst := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/first")
	logsSecond := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/second")
	logsThird := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{logsFirst}, {logsSecond}, {logsThird}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(context.Background(),
		connectortest.NewNopSettings(), cfg, router.(consumer.Logs))

	failoverConnector := conn.(*logsFailover)
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	require.NoError(t, err)
	require.NotNil(t, conn)

	lc, _, ok := failoverConnector.failover.getCurrentConsumer()
	lc1 := failoverConnector.failover.GetConsumerAtIndex(1)
	lc2 := failoverConnector.failover.GetConsumerAtIndex(2)

	assert.True(t, ok)
	require.Implements(t, (*consumer.Logs)(nil), lc)
	require.Implements(t, (*consumer.Logs)(nil), lc1)
	require.Implements(t, (*consumer.Logs)(nil), lc2)
}

func TestLogsWithValidFailover(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.LogsSink
	logsFirst := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/first")
	logsSecond := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/second")
	logsThird := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{logsFirst}, {logsSecond}, {logsThird}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(context.Background(),
		connectortest.NewNopSettings(), cfg, router.(consumer.Logs))

	require.NoError(t, err)

	failoverConnector := conn.(*logsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	ld := sampleLog()

	require.Eventually(t, func() bool {
		return consumeLogsAndCheckStable(failoverConnector, 1, ld)
	}, 3*time.Second, 5*time.Millisecond)
}

func TestLogsWithFailoverError(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.LogsSink
	logsFirst := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/first")
	logsSecond := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/second")
	logsThird := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{logsFirst}, {logsSecond}, {logsThird}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(context.Background(),
		connectortest.NewNopSettings(), cfg, router.(consumer.Logs))

	require.NoError(t, err)

	failoverConnector := conn.(*logsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errLogsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errLogsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	ld := sampleLog()

	assert.EqualError(t, conn.ConsumeLogs(context.Background(), ld), "All provided pipelines return errors")
}

func consumeLogsAndCheckStable(conn *logsFailover, idx int, lr plog.Logs) bool {
	_ = conn.ConsumeLogs(context.Background(), lr)
	stableIndex := conn.failover.pS.TestStableIndex()
	return stableIndex == idx
}

func sampleLog() plog.Logs {
	l := plog.NewLogs()
	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("test", "logs-test")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	return l
}
