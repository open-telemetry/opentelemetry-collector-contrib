// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

type preservationTest struct {
	connector    component.Component
	prepareRetry func()
	consume      func() (inputCount int, err error)
	batchCount   func() int
	itemCount    func(int) int
}

func TestDataPreservedAcrossFailoverAttempts(t *testing.T) {
	for _, signalTest := range []struct {
		signal pipeline.Signal
		setup  func(*testing.T, *Config, error) preservationTest
	}{
		{signal: pipeline.SignalLogs, setup: setupLogsPreservationTest},
		{signal: pipeline.SignalTraces, setup: setupTracesPreservationTest},
		{signal: pipeline.SignalMetrics, setup: setupMetricsPreservationTest},
	} {
		t.Run(signalTest.signal.String(), func(t *testing.T) {
			for _, tc := range []struct {
				name  string
				retry bool
			}{
				{name: "failover"},
				{name: "recovery_probe", retry: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					firstID := pipeline.NewIDWithName(signalTest.signal, "first")
					secondID := pipeline.NewIDWithName(signalTest.signal, "second")
					cfg := &Config{
						PipelinePriority: [][]pipeline.ID{{firstID}, {secondID}},
						RetryInterval:    time.Hour,
					}
					test := signalTest.setup(t, cfg, errors.New("connection refused"))
					defer func() {
						require.NoError(t, test.connector.Shutdown(t.Context()))
					}()
					if tc.retry {
						test.prepareRetry()
					}

					for i := range 2 {
						inputCount, err := test.consume()
						require.NoError(t, err)

						require.Equal(t, i+1, test.batchCount())
						require.Equal(t, 1, test.itemCount(i), "the healthy backup must receive the original data")
						require.Equal(t, 1, inputCount, "the connector must preserve its non-mutating contract")
					}
				})
			}
		})
	}
}

func setupLogsPreservationTest(t *testing.T, cfg *Config, downstreamErr error) preservationTest {
	t.Helper()
	primary, err := consumer.NewLogs(func(_ context.Context, data plog.Logs) error {
		data.MoveTo(plog.NewLogs())
		return downstreamErr
	}, consumer.WithCapabilities(consumer.Capabilities{MutatesData: true}))
	require.NoError(t, err)

	var backup consumertest.LogsSink
	router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
		cfg.PipelinePriority[0][0]: primary,
		cfg.PipelinePriority[1][0]: &backup,
	})

	conn, err := NewFactory().CreateLogsToLogs(
		t.Context(), connectortest.NewNopSettings(metadata.Type), cfg, router,
	)
	require.NoError(t, err)

	f := conn.(*logsFailover).failover
	return preservationTest{
		connector: conn,
		prepareRetry: func() {
			f.reportConsumerError(0, downstreamErr)
			f.notifyRetry <- struct{}{}
		},
		consume: func() (int, error) {
			data := sampleLog()
			err := conn.ConsumeLogs(t.Context(), data)
			return data.LogRecordCount(), err
		},
		batchCount: func() int { return len(backup.AllLogs()) },
		itemCount:  func(i int) int { return backup.AllLogs()[i].LogRecordCount() },
	}
}

func setupTracesPreservationTest(t *testing.T, cfg *Config, downstreamErr error) preservationTest {
	t.Helper()
	primary, err := consumer.NewTraces(func(_ context.Context, data ptrace.Traces) error {
		data.MoveTo(ptrace.NewTraces())
		return downstreamErr
	}, consumer.WithCapabilities(consumer.Capabilities{MutatesData: true}))
	require.NoError(t, err)

	var backup consumertest.TracesSink
	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		cfg.PipelinePriority[0][0]: primary,
		cfg.PipelinePriority[1][0]: &backup,
	})

	conn, err := NewFactory().CreateTracesToTraces(
		t.Context(), connectortest.NewNopSettings(metadata.Type), cfg, router,
	)
	require.NoError(t, err)

	f := conn.(*tracesFailover).failover
	return preservationTest{
		connector: conn,
		prepareRetry: func() {
			f.reportConsumerError(0, downstreamErr)
			f.notifyRetry <- struct{}{}
		},
		consume: func() (int, error) {
			data := sampleTrace()
			err := conn.ConsumeTraces(t.Context(), data)
			return data.SpanCount(), err
		},
		batchCount: func() int { return len(backup.AllTraces()) },
		itemCount:  func(i int) int { return backup.AllTraces()[i].SpanCount() },
	}
}

func setupMetricsPreservationTest(t *testing.T, cfg *Config, downstreamErr error) preservationTest {
	t.Helper()
	primary, err := consumer.NewMetrics(func(_ context.Context, data pmetric.Metrics) error {
		data.MoveTo(pmetric.NewMetrics())
		return downstreamErr
	}, consumer.WithCapabilities(consumer.Capabilities{MutatesData: true}))
	require.NoError(t, err)

	var backup consumertest.MetricsSink
	router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
		cfg.PipelinePriority[0][0]: primary,
		cfg.PipelinePriority[1][0]: &backup,
	})

	conn, err := NewFactory().CreateMetricsToMetrics(
		t.Context(), connectortest.NewNopSettings(metadata.Type), cfg, router,
	)
	require.NoError(t, err)

	f := conn.(*metricsFailover).failover
	return preservationTest{
		connector: conn,
		prepareRetry: func() {
			f.reportConsumerError(0, downstreamErr)
			f.notifyRetry <- struct{}{}
		},
		consume: func() (int, error) {
			data := sampleMetric()
			data.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().AppendEmpty().SetIntValue(1)
			err := conn.ConsumeMetrics(t.Context(), data)
			return data.DataPointCount(), err
		},
		batchCount: func() int { return len(backup.AllMetrics()) },
		itemCount:  func(i int) int { return backup.AllMetrics()[i].DataPointCount() },
	}
}
