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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

var errMetricsConsumer = errors.New("Error from ConsumeMetrics")

func TestMetricsWithErrorCondition(t *testing.T) {
	var first, second consumertest.MetricsSink
	firstID := pipeline.NewIDWithName(pipeline.SignalMetrics, "first")
	secondID := pipeline.NewIDWithName(pipeline.SignalMetrics, "second")
	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{firstID}, {secondID}},
		RetryInterval:    time.Hour,
		Condition:        configoptional.Some(ConditionsConfig{ErrorCond: &ErrorCondition{Contains: []string{"network failure", "connection refused"}}}),
	}
	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{firstID: &first, secondID: &second})
	conn, err := NewFactory().CreateMetricsToMetrics(t.Context(), connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Metrics))
	require.NoError(t, err)
	f := conn.(*metricsFailover)
	defer func() {
		assert.NoError(t, f.Shutdown(t.Context()))
	}()
	data := sampleMetric()
	nonMatching := errors.New("queue full")
	f.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(nonMatching))
	require.ErrorIs(t, f.ConsumeMetrics(t.Context(), data), nonMatching)
	assert.Equal(t, 0, f.failover.TestGetCurrentConsumerIndex())
	assert.Empty(t, second.AllMetrics())
	f.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errors.New("CONNECTION REFUSED")))
	require.NoError(t, f.ConsumeMetrics(t.Context(), data))
	assert.Equal(t, 1, f.failover.TestGetCurrentConsumerIndex())
	require.NoError(t, f.ConsumeMetrics(t.Context(), data))
	assert.Len(t, second.AllMetrics(), 2)
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
