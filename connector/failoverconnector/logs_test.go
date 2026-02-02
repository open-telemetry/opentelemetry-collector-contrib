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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
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
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Logs))

	wrappedConn := conn.(*wrappedLogsConnector)
	failoverRouter := wrappedConn.GetFailoverRouter()
	defer func() {
		assert.NoError(t, wrappedConn.Shutdown(t.Context()))
	}()

	require.NoError(t, err)
	require.NotNil(t, conn)

	lc := failoverRouter.TestGetConsumerAtIndex(0)
	lc1 := failoverRouter.TestGetConsumerAtIndex(1)
	lc2 := failoverRouter.TestGetConsumerAtIndex(2)

	require.Equal(t, lc, &sinkFirst)
	require.Equal(t, lc1, &sinkSecond)
	require.Equal(t, lc2, &sinkThird)
}

func TestLogsWithValidFailover(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.LogsSink
	logsFirst := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/first")
	logsSecond := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/second")
	logsThird := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{logsFirst}, {logsSecond}, {logsThird}},
		RetryInterval:    50 * time.Millisecond,
	}

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Logs))

	require.NoError(t, err)

	failoverConnector := conn.(*logsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
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
	}

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Logs))

	require.NoError(t, err)

	failoverConnector := conn.(*logsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errLogsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errLogsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
	}()

	ld := sampleLog()

	assert.EqualError(t, conn.ConsumeLogs(t.Context(), ld), "All provided pipelines return errors")
}

func TestLogsWithQueue(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.LogsSink
	logsFirst := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/first")
	logsSecond := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/second")
	logsThird := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{logsFirst}, {logsSecond}, {logsThird}},
		RetryInterval:    50 * time.Millisecond,
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}

	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Logs))

	require.NoError(t, err)

	failoverConnector := conn.(*wrappedLogsConnector)
	lRouter := failoverConnector.GetFailoverRouter()
	lRouter.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
	lRouter.ModifyConsumerAtIndex(1, consumertest.NewErr(errLogsConsumer))
	lRouter.ModifyConsumerAtIndex(2, consumertest.NewErr(errLogsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
	}()

	ld := sampleLog()

	assert.NoError(t, conn.ConsumeLogs(t.Context(), ld))
}

func consumeLogsAndCheckStable(conn *logsFailover, idx int, lr plog.Logs) bool {
	_ = conn.ConsumeLogs(context.Background(), lr)
	stableIndex := conn.failover.pS.CurrentPipeline()
	return stableIndex == idx
}

func sampleLog() plog.Logs {
	l := plog.NewLogs()
	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("test", "logs-test")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	return l
}
