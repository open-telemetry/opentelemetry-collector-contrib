// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
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

func TestLogsPreservedAcrossFailoverAttempts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		retry bool
	}{
		{name: "failover"},
		{name: "recovery_probe", retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			downstreamErr := errors.New("connection refused")
			primary, err := consumer.NewLogs(func(_ context.Context, data plog.Logs) error {
				data.MoveTo(plog.NewLogs())
				return downstreamErr
			}, consumer.WithCapabilities(consumer.Capabilities{MutatesData: true}))
			require.NoError(t, err)

			firstID := pipeline.NewIDWithName(pipeline.SignalLogs, "first")
			secondID := pipeline.NewIDWithName(pipeline.SignalLogs, "second")
			cfg := &Config{
				PipelinePriority: [][]pipeline.ID{{firstID}, {secondID}},
				RetryInterval:    time.Hour,
			}

			var backup consumertest.LogsSink
			router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
				firstID:  primary,
				secondID: &backup,
			})

			conn, err := NewFactory().CreateLogsToLogs(
				t.Context(), connectortest.NewNopSettings(metadata.Type), cfg, router,
			)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, conn.Shutdown(t.Context()))
			}()

			f := conn.(*logsFailover).failover
			if tc.retry {
				f.reportConsumerError(0, downstreamErr)
				f.notifyRetry <- struct{}{}
			}

			for i := range 2 {
				data := sampleLog()
				require.NoError(t, conn.ConsumeLogs(t.Context(), data))

				require.Len(t, backup.AllLogs(), i+1)
				require.Equal(t, 1, backup.AllLogs()[i].LogRecordCount(), "the healthy backup must receive the original data")

				require.Equal(t, 1, data.LogRecordCount(), "the connector must preserve its non-mutating contract")
			}
		})
	}
}

func TestTracesPreservedAcrossFailoverAttempts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		retry bool
	}{
		{name: "failover"},
		{name: "recovery_probe", retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			downstreamErr := errors.New("connection refused")
			primary, err := consumer.NewTraces(func(_ context.Context, data ptrace.Traces) error {
				data.MoveTo(ptrace.NewTraces())
				return downstreamErr
			}, consumer.WithCapabilities(consumer.Capabilities{MutatesData: true}))
			require.NoError(t, err)

			firstID := pipeline.NewIDWithName(pipeline.SignalTraces, "first")
			secondID := pipeline.NewIDWithName(pipeline.SignalTraces, "second")
			cfg := &Config{
				PipelinePriority: [][]pipeline.ID{{firstID}, {secondID}},
				RetryInterval:    time.Hour,
			}

			var backup consumertest.TracesSink
			router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
				firstID:  primary,
				secondID: &backup,
			})

			conn, err := NewFactory().CreateTracesToTraces(
				t.Context(), connectortest.NewNopSettings(metadata.Type), cfg, router,
			)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, conn.Shutdown(t.Context()))
			}()

			f := conn.(*tracesFailover).failover
			if tc.retry {
				f.reportConsumerError(0, downstreamErr)
				f.notifyRetry <- struct{}{}
			}

			for i := range 2 {
				data := sampleTrace()
				require.NoError(t, conn.ConsumeTraces(t.Context(), data))

				require.Len(t, backup.AllTraces(), i+1)
				require.Equal(t, 1, backup.AllTraces()[i].SpanCount(), "the healthy backup must receive the original data")

				require.Equal(t, 1, data.SpanCount(), "the connector must preserve its non-mutating contract")
			}
		})
	}
}

func TestMetricsPreservedAcrossFailoverAttempts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		retry bool
	}{
		{name: "failover"},
		{name: "recovery_probe", retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			downstreamErr := errors.New("connection refused")
			primary, err := consumer.NewMetrics(func(_ context.Context, data pmetric.Metrics) error {
				data.MoveTo(pmetric.NewMetrics())
				return downstreamErr
			}, consumer.WithCapabilities(consumer.Capabilities{MutatesData: true}))
			require.NoError(t, err)

			firstID := pipeline.NewIDWithName(pipeline.SignalMetrics, "first")
			secondID := pipeline.NewIDWithName(pipeline.SignalMetrics, "second")
			cfg := &Config{
				PipelinePriority: [][]pipeline.ID{{firstID}, {secondID}},
				RetryInterval:    time.Hour,
			}

			var backup consumertest.MetricsSink
			router := connector.NewMetricsRouter(map[pipeline.ID]consumer.Metrics{
				firstID:  primary,
				secondID: &backup,
			})

			conn, err := NewFactory().CreateMetricsToMetrics(
				t.Context(), connectortest.NewNopSettings(metadata.Type), cfg, router,
			)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, conn.Shutdown(t.Context()))
			}()

			f := conn.(*metricsFailover).failover
			if tc.retry {
				f.reportConsumerError(0, downstreamErr)
				f.notifyRetry <- struct{}{}
			}

			for i := range 2 {
				data := sampleMetric()
				data.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().AppendEmpty().SetIntValue(1)
				require.NoError(t, conn.ConsumeMetrics(t.Context(), data))

				require.Len(t, backup.AllMetrics(), i+1)
				require.Equal(t, 1, backup.AllMetrics()[i].DataPointCount(), "the healthy backup must receive the original data")

				require.Equal(t, 1, data.DataPointCount(), "the connector must preserve its non-mutating contract")
			}
		})
	}
}

func TestLogsFanoutPreservedAcrossFailoverAttempts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		readOnly bool
		retry    bool
	}{
		{name: "all_mutating/failover"},
		{name: "all_mutating/recovery_probe", retry: true},
		{name: "mixed/failover", readOnly: true},
		{name: "mixed/recovery_probe", readOnly: true, retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			downstreamErr := errors.New("connection refused")
			primary, err := consumer.NewLogs(func(_ context.Context, data plog.Logs) error {
				data.MoveTo(plog.NewLogs())
				return downstreamErr
			}, consumer.WithCapabilities(consumer.Capabilities{MutatesData: true}))
			require.NoError(t, err)

			firstID := pipeline.NewIDWithName(pipeline.SignalLogs, "first")
			otherID := pipeline.NewIDWithName(pipeline.SignalLogs, "other")
			backupID := pipeline.NewIDWithName(pipeline.SignalLogs, "backup")
			var readOnly, backup consumertest.LogsSink
			other := primary
			if tc.readOnly {
				other = &readOnly
			}

			cfg := createDefaultConfig().(*Config)
			cfg.PipelinePriority = [][]pipeline.ID{{firstID, otherID}, {backupID}}
			cfg.RetryInterval = time.Hour

			router := connector.NewLogsRouter(map[pipeline.ID]consumer.Logs{
				firstID:  primary,
				otherID:  other,
				backupID: &backup,
			})

			conn, err := NewFactory().CreateLogsToLogs(
				t.Context(), connectortest.NewNopSettings(metadata.Type), cfg, router,
			)
			require.NoError(t, err)
			shutdown := sync.OnceFunc(func() {
				require.NoError(t, conn.Shutdown(t.Context()))
			})
			defer shutdown()

			f := conn.(*wrappedLogsConnector).GetFailoverRouter()
			if tc.retry {
				f.reportConsumerError(0, downstreamErr)
				f.notifyRetry <- struct{}{}
			}

			require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))

			data := sampleLog()
			require.NoError(t, conn.ConsumeLogs(t.Context(), data))
			// Shutdown drains the queue before the sink assertions.
			shutdown()

			if tc.readOnly {
				require.Len(t, readOnly.AllLogs(), 1)
				require.Equal(t, 1, readOnly.AllLogs()[0].LogRecordCount())
			}

			require.Len(t, backup.AllLogs(), 1)
			require.Equal(t, 1, backup.AllLogs()[0].LogRecordCount(), "the healthy backup must receive the original data")

			require.Equal(t, 1, data.LogRecordCount(), "the connector must preserve its non-mutating contract")
		})
	}
}
