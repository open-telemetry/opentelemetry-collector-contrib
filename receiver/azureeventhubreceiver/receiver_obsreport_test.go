// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

// fixedUnmarshaler returns the same telemetry for every event.
type fixedUnmarshaler struct {
	logs    plog.Logs
	metrics pmetric.Metrics
	traces  ptrace.Traces
}

func (u fixedUnmarshaler) UnmarshalLogs(*azureEvent) (plog.Logs, error) { return u.logs, nil }
func (u fixedUnmarshaler) UnmarshalMetrics(*azureEvent) (pmetric.Metrics, error) {
	return u.metrics, nil
}

func (u fixedUnmarshaler) UnmarshalTraces(*azureEvent) (ptrace.Traces, error) { return u.traces, nil }

// TestReceiverReportsItemCounts checks that the accepted-items metrics count log
// records, data points and spans, not Event Hub events.
func TestReceiverReportsItemCounts(t *testing.T) {
	logs := plog.NewLogs()
	records := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	for range 3 {
		records.AppendEmpty()
	}
	metrics := pmetric.NewMetrics()
	dps := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints()
	for range 4 {
		dps.AppendEmpty()
	}
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	for range 5 {
		spans.AppendEmpty()
	}
	u := fixedUnmarshaler{logs: logs, metrics: metrics, traces: traces}

	tests := []struct {
		signal pipeline.Signal
		metric string
		want   int64
	}{
		{signal: pipeline.SignalLogs, metric: "otelcol_receiver_accepted_log_records", want: 3},
		{signal: pipeline.SignalMetrics, metric: "otelcol_receiver_accepted_metric_points", want: 4},
		{signal: pipeline.SignalTraces, metric: "otelcol_receiver_accepted_spans", want: 5},
	}
	for _, tt := range tests {
		t.Run(tt.signal.String(), func(t *testing.T) {
			tel := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.WithoutCancel(t.Context()))) })
			set := receivertest.NewNopSettings(metadata.Type)
			set.TelemetrySettings = tel.NewTelemetrySettings()

			rcv, err := newReceiver(tt.signal, nil, u, u, u, newEventhubHandler(&Config{}, set), set)
			require.NoError(t, err)
			r := rcv.(*eventhubReceiver)
			r.setNextLogsConsumer(consumertest.NewNop())
			r.setNextMetricsConsumer(consumertest.NewNop())
			r.setNextTracesConsumer(consumertest.NewNop())

			// Two events, each carrying the fixed telemetry.
			for range 2 {
				require.NoError(t, r.consume(t.Context(), &azureEvent{}))
			}

			got, err := tel.GetMetric(tt.metric)
			require.NoError(t, err)
			sum, ok := got.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Len(t, sum.DataPoints, 1)
			assert.Equal(t, 2*tt.want, sum.DataPoints[0].Value)
		})
	}
}
