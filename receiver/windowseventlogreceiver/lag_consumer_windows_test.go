// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowseventlogreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadatatest"
)

const testChannel = "Application"

func makeLog(ts, obs time.Time) plog.Logs {
	logs := plog.NewLogs()
	lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	if !ts.IsZero() {
		lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	}
	if !obs.IsZero() {
		lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(obs))
	}
	return logs
}

func newTestLagConsumer(t *testing.T) (*lagTrackingConsumer, *consumertest.LogsSink, *componenttest.Telemetry) {
	t.Helper()
	tel := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)
	sink := &consumertest.LogsSink{}
	cfg := MetricsConfig{ReceiverWindowsEventLogLag: MetricConfig{Enabled: true}}
	lc, err := newLagTrackingConsumer(sink, tb, testChannel, cfg)
	require.NoError(t, err)
	return lc, sink, tel
}

func channelAttr(ch string) attribute.KeyValue {
	return attribute.String("channel", ch)
}

func TestLagConsumer_Capabilities(t *testing.T) {
	lc, _, tel := newTestLagConsumer(t)
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()
	require.Equal(t, consumer.Capabilities{MutatesData: false}, lc.Capabilities())
}

func TestLagConsumer_EmptyBatch(t *testing.T) {
	lc, sink, tel := newTestLagConsumer(t)
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()

	require.NoError(t, lc.ConsumeLogs(t.Context(), plog.NewLogs()))
	require.Len(t, sink.AllLogs(), 1) // empty logs still forwarded

	// Observable gauge reports 0 when no lag was stored
	metadatatest.AssertEqualReceiverWindowsEventLogLag(t, tel,
		[]metricdata.DataPoint[float64]{{
			Value:      0,
			Attributes: attribute.NewSet(channelAttr(testChannel)),
		}},
		metricdatatest.IgnoreTimestamp())
}

func TestLagConsumer_NoTimestamp(t *testing.T) {
	lc, _, tel := newTestLagConsumer(t)
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()

	logs := makeLog(time.Time{}, time.Time{}) // both zero — no lag stored
	require.NoError(t, lc.ConsumeLogs(t.Context(), logs))

	// Gauge should still report 0
	metadatatest.AssertEqualReceiverWindowsEventLogLag(t, tel,
		[]metricdata.DataPoint[float64]{{
			Value:      0,
			Attributes: attribute.NewSet(channelAttr(testChannel)),
		}},
		metricdatatest.IgnoreTimestamp())
}

func TestLagConsumer_RecordsLag(t *testing.T) {
	lc, _, tel := newTestLagConsumer(t)
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()

	now := time.Now()
	eventTime := now.Add(-5 * time.Second)
	logs := makeLog(eventTime, now)

	require.NoError(t, lc.ConsumeLogs(t.Context(), logs))

	got, err := tel.GetMetric("otelcol_receiver_windows_event_log_lag")
	require.NoError(t, err)
	dp := got.Data.(metricdata.Gauge[float64]).DataPoints
	require.Len(t, dp, 1)
	require.InDelta(t, 5.0, dp[0].Value, 0.1)
}

func TestLagConsumer_MaxLagAcrossRecords(t *testing.T) {
	lc, _, tel := newTestLagConsumer(t)
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()

	now := time.Now()
	logs := plog.NewLogs()
	sl := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()

	// Record 1: 2s lag
	lr1 := sl.LogRecords().AppendEmpty()
	lr1.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(-2 * time.Second)))
	lr1.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))

	// Record 2: 10s lag (should be the max)
	lr2 := sl.LogRecords().AppendEmpty()
	lr2.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(-10 * time.Second)))
	lr2.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))

	// Record 3: 1s lag
	lr3 := sl.LogRecords().AppendEmpty()
	lr3.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Second)))
	lr3.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))

	require.NoError(t, lc.ConsumeLogs(t.Context(), logs))

	got, err := tel.GetMetric("otelcol_receiver_windows_event_log_lag")
	require.NoError(t, err)
	dp := got.Data.(metricdata.Gauge[float64]).DataPoints
	require.Len(t, dp, 1)
	require.InDelta(t, 10.0, dp[0].Value, 0.1)
}

func TestLagConsumer_ObservedTimestampZeroFallsBackToNow(t *testing.T) {
	lc, _, tel := newTestLagConsumer(t)
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()

	eventTime := time.Now().Add(-3 * time.Second)
	logs := makeLog(eventTime, time.Time{}) // ObservedTimestamp is zero

	require.NoError(t, lc.ConsumeLogs(t.Context(), logs))

	got, err := tel.GetMetric("otelcol_receiver_windows_event_log_lag")
	require.NoError(t, err)
	dp := got.Data.(metricdata.Gauge[float64]).DataPoints
	require.Len(t, dp, 1)
	// lag should be ~3s; give a generous delta to account for test execution time
	require.InDelta(t, 3.0, dp[0].Value, 1.0)
}

// TestLagConsumer_ResetsToZeroAfterCollection verifies the async gauge resets to 0
// after a collection cycle, so an idle receiver reports 0 lag rather than holding
// the last non-zero value.
func TestLagConsumer_ResetsToZeroAfterCollection(t *testing.T) {
	lc, _, tel := newTestLagConsumer(t)
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()

	now := time.Now()
	logs := makeLog(now.Add(-5*time.Second), now)
	require.NoError(t, lc.ConsumeLogs(t.Context(), logs))

	// First collection: should show ~5s lag
	got, err := tel.GetMetric("otelcol_receiver_windows_event_log_lag")
	require.NoError(t, err)
	dp := got.Data.(metricdata.Gauge[float64]).DataPoints
	require.Len(t, dp, 1)
	require.InDelta(t, 5.0, dp[0].Value, 0.1)

	// Second collection without any ConsumeLogs call: atomic was swapped to 0, so gauge reads 0
	metadatatest.AssertEqualReceiverWindowsEventLogLag(t, tel,
		[]metricdata.DataPoint[float64]{{
			Value:      0,
			Attributes: attribute.NewSet(channelAttr(testChannel)),
		}},
		metricdatatest.IgnoreTimestamp())
}

func TestLagConsumer_ForwardsLogsToNext(t *testing.T) {
	lc, sink, tel := newTestLagConsumer(t)
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()

	now := time.Now()
	logs := makeLog(now.Add(-1*time.Second), now)

	require.NoError(t, lc.ConsumeLogs(t.Context(), logs))
	require.Equal(t, 1, sink.LogRecordCount())
}
