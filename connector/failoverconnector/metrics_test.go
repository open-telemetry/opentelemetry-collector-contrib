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
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
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

	tc, idx, ok := failoverConnector.failover.getCurrentConsumer()
	tc1 := failoverConnector.failover.GetConsumerAtIndex(1)
	tc2 := failoverConnector.failover.GetConsumerAtIndex(2)

	assert.True(t, ok)
	require.Equal(t, idx, 0)
	require.Implements(t, (*consumer.Metrics)(nil), tc)
	require.Implements(t, (*consumer.Metrics)(nil), tc1)
	require.Implements(t, (*consumer.Metrics)(nil), tc2)
}

func TestMetricsWithValidFailover(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.MetricsSink
	metricsFirst := component.NewIDWithName(component.DataTypeMetrics, "metrics/first")
	metricsSecond := component.NewIDWithName(component.DataTypeMetrics, "metrics/second")
	metricsThird := component.NewIDWithName(component.DataTypeMetrics, "metrics/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{metricsFirst}, {metricsSecond}, {metricsThird}},
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
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

	tr := sampleMetric()

	require.NoError(t, conn.ConsumeMetrics(context.Background(), tr))
	_, idx, ok := failoverConnector.failover.getCurrentConsumer()
	assert.True(t, ok)
	require.Equal(t, idx, 1)
}

func TestMetricsWithFailoverError(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.MetricsSink
	metricsFirst := component.NewIDWithName(component.DataTypeMetrics, "metrics/first")
	metricsSecond := component.NewIDWithName(component.DataTypeMetrics, "metrics/second")
	metricsThird := component.NewIDWithName(component.DataTypeMetrics, "metrics/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{metricsFirst}, {metricsSecond}, {metricsThird}},
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
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

	tr := sampleMetric()

	assert.EqualError(t, conn.ConsumeMetrics(context.Background(), tr), "All provided pipelines return errors")
}

func TestMetricsWithFailoverRecovery(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.MetricsSink
	metricsFirst := component.NewIDWithName(component.DataTypeMetrics, "metrics/first")
	metricsSecond := component.NewIDWithName(component.DataTypeMetrics, "metrics/second")
	metricsThird := component.NewIDWithName(component.DataTypeMetrics, "metrics/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{metricsFirst}, {metricsSecond}, {metricsThird}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       1000,
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

	tr := sampleMetric()

	require.NoError(t, conn.ConsumeMetrics(context.Background(), tr))
	_, idx, ok := failoverConnector.failover.getCurrentConsumer()

	assert.True(t, ok)
	require.Equal(t, idx, 1)

	// Simulate recovery of exporter
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewNop())

	time.Sleep(100 * time.Millisecond)

	_, idx, ok = failoverConnector.failover.getCurrentConsumer()
	assert.True(t, ok)
	require.Equal(t, idx, 0)
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
