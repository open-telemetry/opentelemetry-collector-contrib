// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"
import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

var errLogsConsumer = errors.New("Error from ConsumeLogs")

type logsCountingConsumer struct {
	err   error
	calls int
}

func (_ *logsCountingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *logsCountingConsumer) ConsumeLogs(context.Context, plog.Logs) error {
	c.calls++
	return c.err
}

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

func TestLogsPermanentErrorFailover(t *testing.T) {
	logsFirst := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/first")
	logsSecond := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/second")

	testcases := []struct {
		name                    string
		cfg                     *Config
		err                     error
		wantErr                 bool
		wantPermanentErr        bool
		wantCurrentPipeline     int
		wantSecondConsumerCalls int
		wantPermanentCondition  *bool
	}{
		{
			name: "default config fails over on permanent errors",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.QueueSettings = configoptional.None[exporterhelper.QueueBatchConfig]()
				cfg.PipelinePriority = [][]pipeline.ID{{logsFirst}, {logsSecond}}
				cfg.RetryInterval = 50 * time.Millisecond
				return cfg
			}(),
			err:                     consumererror.NewPermanent(errLogsConsumer),
			wantCurrentPipeline:     1,
			wantSecondConsumerCalls: 1,
		},
		{
			name: "permanent condition false returns permanent error",
			cfg: &Config{
				PipelinePriority: [][]pipeline.ID{{logsFirst}, {logsSecond}},
				RetryInterval:    50 * time.Millisecond,
				Condition: configoptional.Some(ConditionsConfig{
					ErrorCond: &ErrorCondition{
						Permanent: new(bool),
					},
				}),
			},
			err:                     consumererror.NewPermanent(errLogsConsumer),
			wantErr:                 true,
			wantPermanentErr:        true,
			wantCurrentPipeline:     0,
			wantSecondConsumerCalls: 0,
			wantPermanentCondition:  new(bool),
		},
		{
			name: "permanent condition false still fails over on retryable errors",
			cfg: &Config{
				PipelinePriority: [][]pipeline.ID{{logsFirst}, {logsSecond}},
				RetryInterval:    50 * time.Millisecond,
				Condition: configoptional.Some(ConditionsConfig{
					ErrorCond: &ErrorCondition{
						Permanent: new(bool),
					},
				}),
			},
			err:                     errLogsConsumer,
			wantCurrentPipeline:     1,
			wantSecondConsumerCalls: 1,
			wantPermanentCondition:  new(bool),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			first := &logsCountingConsumer{err: tc.err}
			second := &logsCountingConsumer{}
			router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
				logsFirst:  first,
				logsSecond: second,
			})

			conn, err := NewFactory().CreateLogsToLogs(t.Context(),
				connectortest.NewNopSettings(metadata.Type), tc.cfg, router.(consumer.Logs))
			require.NoError(t, err)

			failoverConnector := conn.(*logsFailover)
			defer func() {
				assert.NoError(t, failoverConnector.Shutdown(t.Context()))
			}()

			err = conn.ConsumeLogs(t.Context(), sampleLog())
			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, errLogsConsumer)
				assert.Equal(t, tc.wantPermanentErr, consumererror.IsPermanent(err))
			} else {
				require.NoError(t, err)
			}
			if tc.wantPermanentCondition == nil {
				require.False(t, tc.cfg.Condition.HasValue())
			} else {
				require.True(t, tc.cfg.Condition.HasValue())
				require.NotNil(t, tc.cfg.Condition.Get().ErrorCond)
				require.NotNil(t, tc.cfg.Condition.Get().ErrorCond.Permanent)
				assert.Equal(t, *tc.wantPermanentCondition, *tc.cfg.Condition.Get().ErrorCond.Permanent)
			}
			assert.Equal(t, tc.wantCurrentPipeline, failoverConnector.failover.TestGetCurrentConsumerIndex())
			assert.Equal(t, 1, first.calls)
			assert.Equal(t, tc.wantSecondConsumerCalls, second.calls)
		})
	}
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

// deadlineCapturingLogsConsumer records ctx.Deadline() observed during ConsumeLogs.
type deadlineCapturingLogsConsumer struct {
	consumer.Logs
	once        sync.Once
	called      chan struct{}
	hasDeadline bool
	deadline    time.Time
}

func (c *deadlineCapturingLogsConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	c.once.Do(func() {
		c.deadline, c.hasDeadline = ctx.Deadline()
		close(c.called)
	})
	return c.Logs.ConsumeLogs(ctx, ld)
}

// TestLogsQueueDoesNotImposeDownstreamDeadline verifies that when the failover connector's
// sending_queue is enabled, the wrapped exporterhelper does not impose its default 5s timeout
// on the downstream pipeline's ctx — see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/48567.
func TestLogsQueueDoesNotImposeDownstreamDeadline(t *testing.T) {
	sink := &consumertest.LogsSink{}
	capture := &deadlineCapturingLogsConsumer{Logs: sink, called: make(chan struct{})}

	logsFirst := pipeline.NewIDWithName(pipeline.SignalLogs, "logs/first")
	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{logsFirst}},
		RetryInterval:    50 * time.Millisecond,
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}
	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		logsFirst: capture,
	})

	conn, err := NewFactory().CreateLogsToLogs(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Logs))
	require.NoError(t, err)

	failoverConnector := conn.(*wrappedLogsConnector)
	require.NoError(t, failoverConnector.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
	}()

	require.NoError(t, conn.ConsumeLogs(t.Context(), sampleLog()))

	select {
	case <-capture.called:
	case <-time.After(2 * time.Second):
		t.Fatal("downstream consumer was not called within 2s")
	}

	assert.False(t, capture.hasDeadline,
		"downstream ctx should not carry a deadline imposed by the failover connector, got %v (in %s)",
		capture.deadline, time.Until(capture.deadline))
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
