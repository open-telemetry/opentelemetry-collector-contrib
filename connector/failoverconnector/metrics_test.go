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
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var errMetricsConsumer = errors.New("Error from ConsumeMetrics")

func TestMetricsRegisterConsumers(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.MetricsSink
	metricsFirst := component.NewIDWithName(component.DataTypeMetrics, "metrics/first")
	metricsSecond := component.NewIDWithName(component.DataTypeMetrics, "metrics/second")
	metricsThird := component.NewIDWithName(component.DataTypeMetrics, "metrics/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{metricsFirst}, {metricsSecond}, {metricsThird}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewMetricsRouter(map[component.ID]consumer.Metrics{
		metricsFirst:  &sinkFirst,
		metricsSecond: &sinkSecond,
		metricsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Metrics))

	failoverConnector := conn.(*metricsFailover)
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	require.NoError(t, err)
	require.NotNil(t, conn)

	mc, _, ok := failoverConnector.failover.getCurrentConsumer()
	mc1 := failoverConnector.failover.GetConsumerAtIndex(1)
	mc2 := failoverConnector.failover.GetConsumerAtIndex(2)

	assert.True(t, ok)
	require.Implements(t, (*consumer.Metrics)(nil), mc)
	require.Implements(t, (*consumer.Metrics)(nil), mc1)
	require.Implements(t, (*consumer.Metrics)(nil), mc2)
}

func TestMetricsWithValidFailover(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.MetricsSink
	metricsFirst := component.NewIDWithName(component.DataTypeMetrics, "metrics/first")
	metricsSecond := component.NewIDWithName(component.DataTypeMetrics, "metrics/second")
	metricsThird := component.NewIDWithName(component.DataTypeMetrics, "metrics/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{metricsFirst}, {metricsSecond}, {metricsThird}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewMetricsRouter(map[component.ID]consumer.Metrics{
		metricsFirst:  &sinkFirst,
		metricsSecond: &sinkSecond,
		metricsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Metrics))

	require.NoError(t, err)

	failoverConnector := conn.(*metricsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errMetricsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	md := sampleMetric()

	require.Eventually(t, func() bool {
		return consumeMetricsAndCheckStable(failoverConnector, 1, md)
	}, 3*time.Second, 5*time.Millisecond)
}

func TestMetricsWithFailoverError(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.MetricsSink
	metricsFirst := component.NewIDWithName(component.DataTypeMetrics, "metrics/first")
	metricsSecond := component.NewIDWithName(component.DataTypeMetrics, "metrics/second")
	metricsThird := component.NewIDWithName(component.DataTypeMetrics, "metrics/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{metricsFirst}, {metricsSecond}, {metricsThird}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewMetricsRouter(map[component.ID]consumer.Metrics{
		metricsFirst:  &sinkFirst,
		metricsSecond: &sinkSecond,
		metricsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Metrics))

	require.NoError(t, err)

	failoverConnector := conn.(*metricsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errMetricsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errMetricsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errMetricsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	md := sampleMetric()

	assert.EqualError(t, conn.ConsumeMetrics(context.Background(), md), "All provided pipelines return errors")
}

func TestMetricsWithRecovery(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird, sinkFourth consumertest.MetricsSink
	metricsFirst := component.NewIDWithName(component.DataTypeMetrics, "metrics/first")
	metricsSecond := component.NewIDWithName(component.DataTypeMetrics, "metrics/second")
	metricsThird := component.NewIDWithName(component.DataTypeMetrics, "metrics/third")
	metricsFourth := component.NewIDWithName(component.DataTypeMetrics, "metrics/fourth")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{metricsFirst}, {metricsSecond}, {metricsThird}, {metricsFourth}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewMetricsRouter(map[component.ID]consumer.Metrics{
		metricsFirst:  &sinkFirst,
		metricsSecond: &sinkSecond,
		metricsThird:  &sinkThird,
		metricsFourth: &sinkFourth,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Metrics))

	require.NoError(t, err)

	failoverConnector := conn.(*metricsFailover)

	mr := sampleMetric()

	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	t.Run("single failover recovery to primary consumer: level 2 -> 1", func(t *testing.T) {
		defer func() {
			resetMetricsConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errMetricsConsumer))

		require.NoError(t, conn.ConsumeMetrics(context.Background(), mr))
		idx := failoverConnector.failover.pS.TestStableIndex()
		require.Equal(t, idx, 1)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)

		require.Eventually(t, func() bool {
			return consumeMetricsAndCheckStable(failoverConnector, 0, mr)
		}, 3*time.Second, 5*time.Millisecond)
	})

	t.Run("double failover recovery: level 3 -> 2 -> 1", func(t *testing.T) {
		defer func() {
			resetMetricsConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errMetricsConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errMetricsConsumer))

		require.NoError(t, conn.ConsumeMetrics(context.Background(), mr))
		idx := failoverConnector.failover.pS.TestStableIndex()
		require.Equal(t, idx, 2)

		// Simulate recovery of exporter
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return consumeMetricsAndCheckStable(failoverConnector, 1, mr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)

		require.Eventually(t, func() bool {
			return consumeMetricsAndCheckStable(failoverConnector, 0, mr)
		}, 3*time.Second, 5*time.Millisecond)
	})

	t.Run("multiple failover recovery: level 3 -> 2 -> 4 -> 3 -> 1", func(t *testing.T) {
		defer func() {
			resetMetricsConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errMetricsConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errMetricsConsumer))

		require.Eventually(t, func() bool {
			return consumeMetricsAndCheckStable(failoverConnector, 2, mr)
		}, 3*time.Second, 5*time.Millisecond)

		// Simulate recovery of exporter
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return consumeMetricsAndCheckStable(failoverConnector, 1, mr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errMetricsConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errMetricsConsumer))

		require.Eventually(t, func() bool {
			return consumeMetricsAndCheckStable(failoverConnector, 3, mr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(2, &sinkThird)

		require.Eventually(t, func() bool {
			return consumeMetricsAndCheckStable(failoverConnector, 2, mr)
		}, 3*time.Second, 5*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkThird)

		require.Eventually(t, func() bool {
			return consumeMetricsAndCheckStable(failoverConnector, 0, mr)
		}, 3*time.Second, 5*time.Millisecond)
	})
}

func TestMetricsWithRecovery_MaxRetries(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird, sinkFourth consumertest.MetricsSink
	metricsFirst := component.NewIDWithName(component.DataTypeMetrics, "metrics/first")
	metricsSecond := component.NewIDWithName(component.DataTypeMetrics, "metrics/second")
	metricsThird := component.NewIDWithName(component.DataTypeMetrics, "metrics/third")
	metricsFourth := component.NewIDWithName(component.DataTypeMetrics, "metrics/fourth")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{metricsFirst}, {metricsSecond}, {metricsThird}, {metricsFourth}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewMetricsRouter(map[component.ID]consumer.Metrics{
		metricsFirst:  &sinkFirst,
		metricsSecond: &sinkSecond,
		metricsThird:  &sinkThird,
		metricsFourth: &sinkFourth,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Metrics))

	require.NoError(t, err)

	failoverConnector := conn.(*metricsFailover)

	mr := sampleMetric()

	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errMetricsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errMetricsConsumer))

	require.Eventually(t, func() bool {
		return consumeMetricsAndCheckStable(failoverConnector, 2, mr)
	}, 3*time.Second, 5*time.Millisecond)

	failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkSecond)
	failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

	require.Eventually(t, func() bool {
		return consumeMetricsAndCheckStable(failoverConnector, 0, mr)
	}, 3*time.Second, 5*time.Millisecond)

	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errMetricsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errMetricsConsumer))
	failoverConnector.failover.pS.SetRetryCountToMax(0)

	require.Eventually(t, func() bool {
		return consumeMetricsAndCheckStable(failoverConnector, 2, mr)
	}, 3*time.Second, 5*time.Millisecond)

	failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkSecond)
	failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

	require.Eventually(t, func() bool {
		return consumeMetricsAndCheckStable(failoverConnector, 1, mr)
	}, 3*time.Second, 5*time.Millisecond)
}

func consumeMetricsAndCheckStable(conn *metricsFailover, idx int, mr pmetric.Metrics) bool {
	_ = conn.ConsumeMetrics(context.Background(), mr)
	stableIndex := conn.failover.pS.TestStableIndex()
	return stableIndex == idx
}

func resetMetricsConsumers(conn *metricsFailover, consumers ...consumer.Metrics) {
	for i, sink := range consumers {

		conn.failover.ModifyConsumerAtIndex(i, sink)
	}
	conn.failover.pS.TestSetStableIndex(0)
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
