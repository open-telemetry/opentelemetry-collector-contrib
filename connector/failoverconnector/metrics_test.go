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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

var errMetricsConsumer = errors.New("Error from ConsumeMetrics")

type metricsCountingConsumer struct {
	err   error
	calls int
}

func (_ *metricsCountingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *metricsCountingConsumer) ConsumeMetrics(context.Context, pmetric.Metrics) error {
	c.calls++
	return c.err
}

func TestMetricsRegisterConsumers(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.MetricsSink
	metricsFirst := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/first")
	metricsSecond := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/second")
	metricsThird := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{metricsFirst}, {metricsSecond}, {metricsThird}},
		RetryInterval:    50 * time.Millisecond,
	}

	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
		metricsFirst:  &sinkFirst,
		metricsSecond: &sinkSecond,
		metricsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Metrics))

	failoverConnector := conn.(*metricsFailover)
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
	}()

	require.NoError(t, err)
	require.NotNil(t, conn)

	mc := failoverConnector.failover.TestGetConsumerAtIndex(0)
	mc1 := failoverConnector.failover.TestGetConsumerAtIndex(1)
	mc2 := failoverConnector.failover.TestGetConsumerAtIndex(2)

	require.Equal(t, mc, &sinkFirst)
	require.Equal(t, mc1, &sinkSecond)
	require.Equal(t, mc2, &sinkThird)
}

func TestMetricsWithValidFailover(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.MetricsSink
	metricsFirst := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/first")
	metricsSecond := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/second")
	metricsThird := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{metricsFirst}, {metricsSecond}, {metricsThird}},
		RetryInterval:    50 * time.Millisecond,
	}

	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
		metricsFirst:  &sinkFirst,
		metricsSecond: &sinkSecond,
		metricsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Metrics))

	require.NoError(t, err)

	failoverConnector := conn.(*metricsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errMetricsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
	}()

	md := sampleMetric()

	require.Eventually(t, func() bool {
		return consumeMetricsAndCheckStable(failoverConnector, 1, md)
	}, 3*time.Second, 5*time.Millisecond)
}

func TestMetricsWithFailoverError(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.MetricsSink
	metricsFirst := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/first")
	metricsSecond := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/second")
	metricsThird := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{metricsFirst}, {metricsSecond}, {metricsThird}},
		RetryInterval:    50 * time.Millisecond,
	}

	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
		metricsFirst:  &sinkFirst,
		metricsSecond: &sinkSecond,
		metricsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Metrics))

	require.NoError(t, err)

	failoverConnector := conn.(*metricsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errMetricsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errMetricsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errMetricsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
	}()

	md := sampleMetric()

	assert.EqualError(t, conn.ConsumeMetrics(t.Context(), md), "All provided pipelines return errors")
}

func TestMetricsPermanentErrorFailover(t *testing.T) {
	metricsFirst := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/first")
	metricsSecond := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/second")

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
				cfg.PipelinePriority = [][]pipeline.ID{{metricsFirst}, {metricsSecond}}
				cfg.RetryInterval = 50 * time.Millisecond
				return cfg
			}(),
			err:                     consumererror.NewPermanent(errMetricsConsumer),
			wantCurrentPipeline:     1,
			wantSecondConsumerCalls: 1,
		},
		{
			name: "permanent condition false returns permanent error",
			cfg: &Config{
				PipelinePriority: [][]pipeline.ID{{metricsFirst}, {metricsSecond}},
				RetryInterval:    50 * time.Millisecond,
				Condition: configoptional.Some(ConditionsConfig{
					ErrorCond: &ErrorCondition{
						Permanent: new(bool),
					},
				}),
			},
			err:                     consumererror.NewPermanent(errMetricsConsumer),
			wantErr:                 true,
			wantPermanentErr:        true,
			wantCurrentPipeline:     0,
			wantSecondConsumerCalls: 0,
			wantPermanentCondition:  new(bool),
		},
		{
			name: "permanent condition false still fails over on retryable errors",
			cfg: &Config{
				PipelinePriority: [][]pipeline.ID{{metricsFirst}, {metricsSecond}},
				RetryInterval:    50 * time.Millisecond,
				Condition: configoptional.Some(ConditionsConfig{
					ErrorCond: &ErrorCondition{
						Permanent: new(bool),
					},
				}),
			},
			err:                     errMetricsConsumer,
			wantCurrentPipeline:     1,
			wantSecondConsumerCalls: 1,
			wantPermanentCondition:  new(bool),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			first := &metricsCountingConsumer{err: tc.err}
			second := &metricsCountingConsumer{}
			router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
				metricsFirst:  first,
				metricsSecond: second,
			})

			conn, err := NewFactory().CreateMetricsToMetrics(t.Context(),
				connectortest.NewNopSettings(metadata.Type), tc.cfg, router.(consumer.Metrics))
			require.NoError(t, err)

			failoverConnector := conn.(*metricsFailover)
			defer func() {
				assert.NoError(t, failoverConnector.Shutdown(t.Context()))
			}()

			err = conn.ConsumeMetrics(t.Context(), sampleMetric())
			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, errMetricsConsumer)
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

func TestMetricsWithQueue(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.MetricsSink
	metricsFirst := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/first")
	metricsSecond := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/second")
	metricsThird := pipeline.NewIDWithName(pipeline.SignalMetrics, "metrics/third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{metricsFirst}, {metricsSecond}, {metricsThird}},
		RetryInterval:    50 * time.Millisecond,
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}

	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
		metricsFirst:  &sinkFirst,
		metricsSecond: &sinkSecond,
		metricsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Metrics))

	require.NoError(t, err)

	failoverConnector := conn.(*wrappedMetricsConnector)
	mRouter := failoverConnector.GetFailoverRouter()
	mRouter.ModifyConsumerAtIndex(0, consumertest.NewErr(errMetricsConsumer))
	mRouter.ModifyConsumerAtIndex(1, consumertest.NewErr(errMetricsConsumer))
	mRouter.ModifyConsumerAtIndex(2, consumertest.NewErr(errMetricsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(t.Context()))
	}()

	md := sampleMetric()

	assert.NoError(t, conn.ConsumeMetrics(t.Context(), md))
}

func consumeMetricsAndCheckStable(conn *metricsFailover, idx int, mr pmetric.Metrics) bool {
	_ = conn.ConsumeMetrics(context.Background(), mr)
	stableIndex := conn.failover.pS.CurrentPipeline()
	return stableIndex == idx
}

func sampleMetric() pmetric.Metrics {
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutInt("sample", 1)
	metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetEmptySum()
	metric.SetName("test")
	return m
}
