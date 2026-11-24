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
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

var errTracesConsumer = errors.New("Error from ConsumeTraces")

type tracesCountingConsumer struct {
	err   error
	calls int
}

func (*tracesCountingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *tracesCountingConsumer) ConsumeTraces(context.Context, ptrace.Traces) error {
	c.calls++
	return c.err
}

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

func TestTracesPermanentErrorFailover(t *testing.T) {
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")

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
				cfg.PipelinePriority = [][]pipeline.ID{{tracesFirst}, {tracesSecond}}
				cfg.RetryInterval = 50 * time.Millisecond
				return cfg
			}(),
			err:                     consumererror.NewPermanent(errTracesConsumer),
			wantCurrentPipeline:     1,
			wantSecondConsumerCalls: 1,
		},
		{
			name: "permanent condition false returns permanent error",
			cfg: &Config{
				PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}},
				RetryInterval:    50 * time.Millisecond,
				Condition: configoptional.Some(ConditionsConfig{
					ErrorCond: &ErrorCondition{
						Contains:  []string{"error from"},
						Permanent: new(bool),
					},
				}),
			},
			err:                     consumererror.NewPermanent(errTracesConsumer),
			wantErr:                 true,
			wantPermanentErr:        true,
			wantCurrentPipeline:     0,
			wantSecondConsumerCalls: 0,
			wantPermanentCondition:  new(bool),
		},
		{
			name: "permanent condition false still fails over on retryable errors",
			cfg: &Config{
				PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}},
				RetryInterval:    50 * time.Millisecond,
				Condition: configoptional.Some(ConditionsConfig{
					ErrorCond: &ErrorCondition{
						Contains:  []string{"error from"},
						Permanent: new(bool),
					},
				}),
			},
			err:                     errTracesConsumer,
			wantCurrentPipeline:     1,
			wantSecondConsumerCalls: 1,
			wantPermanentCondition:  new(bool),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			first := &tracesCountingConsumer{err: tc.err}
			second := &tracesCountingConsumer{}
			router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
				tracesFirst:  first,
				tracesSecond: second,
			})

			conn, err := NewFactory().CreateTracesToTraces(t.Context(),
				connectortest.NewNopSettings(metadata.Type), tc.cfg, router.(consumer.Traces))
			require.NoError(t, err)

			failoverConnector := conn.(*tracesFailover)
			defer func() {
				assert.NoError(t, failoverConnector.Shutdown(t.Context()))
			}()

			err = conn.ConsumeTraces(t.Context(), sampleTrace())
			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, errTracesConsumer)
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

func TestTracesWithErrorCondition(t *testing.T) {
	var sinkFirst, sinkSecond consumertest.TracesSink

	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}},
		RetryInterval:    50 * time.Millisecond,
		Condition: configoptional.Some(ConditionsConfig{
			ErrorCond: &ErrorCondition{Contains: []string{"network failure", "connection refused"}},
		}),
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
	})

	conn, err := NewFactory().CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))
	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
	}()

	tr := sampleTrace()

	// A non-matching error must not trigger failover: it is returned directly
	// and the stable pipeline stays at index 0.
	nonMatching := errors.New("sending_queue is full: data dropped")
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(nonMatching))
	require.ErrorIs(t, failoverConnector.ConsumeTraces(t.Context(), tr), nonMatching)
	assert.Equal(t, 0, failoverConnector.failover.TestGetCurrentConsumerIndex())
	assert.Empty(t, sinkSecond.AllTraces(), "failover pipeline must not receive data on non-matching error")

	// A matching error must trigger failover to the next priority level.
	matching := errors.New("connection reset: network failure while exporting")
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(matching))
	require.NoError(t, failoverConnector.ConsumeTraces(t.Context(), tr))
	assert.Equal(t, 1, failoverConnector.failover.TestGetCurrentConsumerIndex())
	require.NoError(t, failoverConnector.ConsumeTraces(t.Context(), tr))
	assert.Len(t, sinkSecond.AllTraces(), 2, "subsequent data must reach the second pipeline")
}

func TestTracesRetryWithNonMatchingError(t *testing.T) {
	var first, second consumertest.TracesSink
	firstID := pipeline.NewIDWithName(pipeline.SignalTraces, "first")
	secondID := pipeline.NewIDWithName(pipeline.SignalTraces, "second")
	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{firstID}, {secondID}},
		RetryInterval:    time.Hour,
		Condition:        configoptional.Some(ConditionsConfig{ErrorCond: &ErrorCondition{Contains: []string{"network failure"}}}),
	}
	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{firstID: &first, secondID: &second})
	conn, err := NewFactory().CreateTracesToTraces(t.Context(), connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))
	require.NoError(t, err)
	f := conn.(*tracesFailover)
	defer func() {
		assert.NoError(t, f.Shutdown(t.Context()))
	}()
	data := sampleTrace()
	f.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errors.New("network failure")))
	require.NoError(t, f.ConsumeTraces(t.Context(), data))
	assert.Equal(t, 1, f.failover.TestGetCurrentConsumerIndex())
	nonMatching := errors.New("queue full")
	f.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(nonMatching))
	f.failover.notifyRetry <- struct{}{}
	require.ErrorIs(t, f.ConsumeTraces(t.Context(), data), nonMatching)
	assert.Equal(t, 0, f.failover.TestGetCurrentConsumerIndex())
	assert.Len(t, second.AllTraces(), 1, "retry batch must not reach the second pipeline")
}
