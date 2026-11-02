// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"
import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadatatest"
)

const activeLevelMetric = "otelcol_connector_failover_active_level"

func activeLevelDataPoints(current int64) []metricdata.DataPoint[int64] {
	return []metricdata.DataPoint[int64]{{
		Attributes: attribute.NewSet(attribute.String("connector", "failover")),
		Value:      current,
	}}
}

func TestActiveLevelMetric(t *testing.T) {
	tel := componenttest.NewTelemetry()
	defer func() {
		require.NoError(t, tel.Shutdown(t.Context()))
	}()

	var sinkFirst, sinkAlsoFirst, sinkSecond, sinkThird consumertest.TracesSink
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "first")
	tracesAlsoFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "also_first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "second")
	tracesThird := pipeline.NewIDWithName(pipeline.SignalTraces, "third")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst, tracesAlsoFirst}, {tracesSecond}, {tracesThird}},
		RetryInterval:    10 * time.Minute,
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:     &sinkFirst,
		tracesAlsoFirst: &sinkAlsoFirst,
		tracesSecond:    &sinkSecond,
		tracesThird:     &sinkThird,
	})

	conn, err := NewFactory().CreateTracesToTraces(t.Context(),
		metadatatest.NewSettings(tel), cfg, router.(consumer.Traces))
	require.NoError(t, err)

	failoverConnector := conn.(*tracesFailover)
	metadatatest.AssertEqualConnectorFailoverActiveLevel(t, tel,
		activeLevelDataPoints(0), metricdatatest.IgnoreTimestamp())

	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	require.NoError(t, conn.ConsumeTraces(t.Context(), sampleTrace()))
	metadatatest.AssertEqualConnectorFailoverActiveLevel(t, tel,
		activeLevelDataPoints(1), metricdatatest.IgnoreTimestamp())

	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errTracesConsumer))
	require.NoError(t, conn.ConsumeTraces(t.Context(), sampleTrace()))
	metadatatest.AssertEqualConnectorFailoverActiveLevel(t, tel,
		activeLevelDataPoints(2), metricdatatest.IgnoreTimestamp())

	failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errTracesConsumer))
	require.ErrorIs(t, conn.ConsumeTraces(t.Context(), sampleTrace()), errNoValidPipeline)
	metadatatest.AssertEqualConnectorFailoverActiveLevel(t, tel,
		activeLevelDataPoints(-1), metricdatatest.IgnoreTimestamp())

	failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)
	failoverConnector.failover.notifyRetry <- struct{}{}
	require.NoError(t, conn.ConsumeTraces(t.Context(), sampleTrace()))
	metadatatest.AssertEqualConnectorFailoverActiveLevel(t, tel,
		activeLevelDataPoints(0), metricdatatest.IgnoreTimestamp())

	require.NoError(t, conn.Shutdown(t.Context()))
	_, err = tel.GetMetric(activeLevelMetric)
	assert.Error(t, err)
}

func TestActiveLevelMetricWithQueue(t *testing.T) {
	tel := componenttest.NewTelemetry()
	defer func() {
		require.NoError(t, tel.Shutdown(t.Context()))
	}()

	var sinkFirst, sinkSecond consumertest.TracesSink
	tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "first")
	tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "second")

	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}},
		RetryInterval:    10 * time.Minute,
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		tracesFirst:  &sinkFirst,
		tracesSecond: &sinkSecond,
	})

	conn, err := NewFactory().CreateTracesToTraces(t.Context(),
		metadatatest.NewSettings(tel), cfg, router.(consumer.Traces))
	require.NoError(t, err)
	require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))

	wrappedConn := conn.(*wrappedTracesConnector)
	wrappedConn.GetFailoverRouter().ModifyConsumerAtIndex(0, consumertest.NewErr(errTracesConsumer))
	require.NoError(t, conn.ConsumeTraces(t.Context(), sampleTrace()))

	require.Eventually(t, func() bool {
		return sinkSecond.SpanCount() == sampleTrace().SpanCount()
	}, 3*time.Second, 5*time.Millisecond)
	metadatatest.AssertEqualConnectorFailoverActiveLevel(t, tel,
		activeLevelDataPoints(1), metricdatatest.IgnoreTimestamp())

	require.NoError(t, conn.Shutdown(t.Context()))
	_, err = tel.GetMetric(activeLevelMetric)
	assert.Error(t, err)
}

type failingExporterMeterProvider struct{ metric.MeterProvider }

func (p failingExporterMeterProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	m := p.MeterProvider.Meter(name, opts...)
	if name == "go.opentelemetry.io/collector/exporter/exporterhelper" {
		return failingExporterMeter{Meter: m}
	}
	return m
}

type failingExporterMeter struct{ metric.Meter }

func (failingExporterMeter) Int64Counter(string, ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return nil, errors.New("exporterhelper telemetry initialization failed")
}

func TestActiveLevelMetricCleanupOnCreateError(t *testing.T) {
	tel := componenttest.NewTelemetry()
	defer func() {
		require.NoError(t, tel.Shutdown(t.Context()))
	}()
	set := metadatatest.NewSettings(tel)
	set.MeterProvider = failingExporterMeterProvider{MeterProvider: set.MeterProvider}

	var sink consumertest.TracesSink
	pid := pipeline.NewIDWithName(pipeline.SignalTraces, "primary")
	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{pid: &sink})
	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{pid}},
		RetryInterval:    time.Minute,
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}

	conn, err := NewFactory().CreateTracesToTraces(t.Context(), set, cfg, router)
	require.ErrorContains(t, err, "exporterhelper telemetry initialization failed")
	require.Nil(t, conn)
	_, err = tel.GetMetric(activeLevelMetric)
	assert.Error(t, err)
}
