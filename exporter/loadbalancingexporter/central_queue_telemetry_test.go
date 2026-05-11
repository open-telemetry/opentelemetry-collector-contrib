// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestCentralQueueTelemetryRecordsInstruments(t *testing.T) {
	reader := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, reader.Shutdown(context.WithoutCancel(t.Context())))
	})
	telemetry, err := newCentralQueueTelemetry(reader.NewTelemetrySettings(), signalKindLogs)
	require.NoError(t, err)
	telemetry.observeOldestItemAge(func() int64 { return 125 })
	telemetry.observeSchedulerState(func() centralQueueSchedulerSnapshot {
		return centralQueueSchedulerSnapshot{
			readyWindows:      2,
			readyWindowLimit:  4,
			readyUncompressed: 48,
			state:             centralQueueSchedulerStateWaiting,
		}
	})

	telemetry.record(t.Context(), centralQueueSnapshot{
		compressedBytes:      50,
		compressedCapacity:   100,
		items:                3,
		inflightUncompressed: 80,
		oldestItemAgeMillis:  125,
	})
	telemetry.recordRejected(t.Context(), 7)
	telemetry.recordRetry(t.Context())
	telemetry.recordDecodeFailure(t.Context(), 5)
	telemetry.recordConfiguredConsumers(t.Context(), 30)
	telemetry.recordActiveConsumers(t.Context(), 3)
	telemetry.recordLanes(t.Context(), 64)
	telemetry.recordWindow(t.Context(), centralQueueWindow{
		items:             []centralQueueItem{{}, {}},
		compressedBytes:   32,
		uncompressedBytes: 128,
		count:             11,
		flushReason:       centralQueueFlushReasonMaxDelayLowTraffic,
	}, 64)

	attrs := attribute.NewSet(attribute.String("signal", string(signalKindLogs)))
	flushAttrs := attribute.NewSet(attribute.String("signal", string(signalKindLogs)), attribute.String("reason", string(centralQueueFlushReasonMaxDelayLowTraffic)))
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_compressed_bytes", "By", attrs, 50)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_compressed_capacity", "By", attrs, 100)
	requireCentralQueueFloatGauge(t, reader, "otelcol_loadbalancer_central_queue_saturation", "1", attrs, 0.5)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_items", "{items}", attrs, 3)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_inflight_uncompressed_bytes", "By", attrs, 80)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_ready_windows", "{windows}", attrs, 2)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_ready_window_limit", "{windows}", attrs, 4)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_ready_uncompressed_bytes", "By", attrs, 48)
	requireCentralQueueSchedulerState(t, reader, centralQueueSchedulerStateWaiting)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_configured_consumers", "{workers}", attrs, 30)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_active_consumers", "{workers}", attrs, 3)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_lanes", "{lanes}", attrs, 64)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_oldest_item_age", "ms", attrs, 125)
	requireCentralQueueIntSum(t, reader, "otelcol_loadbalancer_central_queue_rejected_compressed_bytes", "By", attrs, 7)
	requireCentralQueueIntSum(t, reader, "otelcol_loadbalancer_central_queue_retries", "{retries}", attrs, 1)
	requireCentralQueueIntSum(t, reader, "otelcol_loadbalancer_central_queue_decode_failures", "{items}", attrs, 5)
	requireCentralQueueIntHistogram(t, reader, "otelcol_loadbalancer_central_queue_window_compressed_bytes", "By", attrs, 32)
	requireCentralQueueIntHistogram(t, reader, "otelcol_loadbalancer_central_queue_window_uncompressed_bytes", "By", attrs, 128)
	requireCentralQueueIntHistogram(t, reader, "otelcol_loadbalancer_central_queue_window_items", "{items}", attrs, 11)
	requireCentralQueueIntHistogram(t, reader, "otelcol_loadbalancer_central_queue_window_payloads", "{payloads}", attrs, 2)
	requireCentralQueueIntSum(t, reader, "otelcol_loadbalancer_central_queue_window_flush_total", "{windows}", flushAttrs, 1)
	requireCentralQueueIntSum(t, reader, "otelcol_loadbalancer_central_queue_window_underfilled_total", "{windows}", flushAttrs, 1)
}

func TestCentralQueueTelemetryOldestItemAgeReportsMultipleSignals(t *testing.T) {
	reader := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, reader.Shutdown(context.WithoutCancel(t.Context())))
	})
	logsTelemetry, err := newCentralQueueTelemetry(reader.NewTelemetrySettings(), signalKindLogs)
	require.NoError(t, err)
	metricsTelemetry, err := newCentralQueueTelemetry(reader.NewTelemetrySettings(), signalKindMetrics)
	require.NoError(t, err)
	logsTelemetry.observeOldestItemAge(func() int64 { return 125 })
	metricsTelemetry.observeOldestItemAge(func() int64 { return 250 })

	metric, err := reader.GetMetric("otelcol_loadbalancer_central_queue_oldest_item_age")
	require.NoError(t, err)
	require.Equal(t, "ms", metric.Unit)
	gauge, ok := metric.Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Len(t, gauge.DataPoints, 2)
	requireCentralQueueIntGaugeDatapoint(t, gauge.DataPoints, attribute.NewSet(attribute.String("signal", string(signalKindLogs))), 125)
	requireCentralQueueIntGaugeDatapoint(t, gauge.DataPoints, attribute.NewSet(attribute.String("signal", string(signalKindMetrics))), 250)
}

func requireCentralQueueSchedulerState(t *testing.T, reader *componenttest.Telemetry, activeState centralQueueSchedulerState) {
	t.Helper()
	metric, err := reader.GetMetric("otelcol_loadbalancer_central_queue_scheduler_state")
	require.NoError(t, err)
	require.Equal(t, "1", metric.Unit)
	gauge, ok := metric.Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Len(t, gauge.DataPoints, len(centralQueueSchedulerStates))
	for _, datapoint := range gauge.DataPoints {
		value, ok := datapoint.Attributes.Value("state")
		require.True(t, ok)
		if value.AsString() == string(activeState) {
			require.EqualValues(t, 1, datapoint.Value)
			continue
		}
		require.Zero(t, datapoint.Value)
	}
}

func requireCentralQueueIntGauge(t *testing.T, reader *componenttest.Telemetry, name, unit string, attrs attribute.Set, value int64) {
	t.Helper()
	metric, err := reader.GetMetric(name)
	require.NoError(t, err)
	require.Equal(t, unit, metric.Unit)
	gauge, ok := metric.Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Len(t, gauge.DataPoints, 1)
	requireCentralQueueIntGaugeDatapoint(t, gauge.DataPoints, attrs, value)
}

func requireCentralQueueIntGaugeDatapoint(t *testing.T, datapoints []metricdata.DataPoint[int64], attrs attribute.Set, value int64) {
	t.Helper()
	for _, datapoint := range datapoints {
		if datapoint.Attributes.Equals(&attrs) {
			require.Equal(t, value, datapoint.Value)
			return
		}
	}
	require.Failf(t, "missing datapoint", "attributes: %v", attrs)
}

func requireCentralQueueFloatGauge(t *testing.T, reader *componenttest.Telemetry, name, unit string, attrs attribute.Set, value float64) {
	t.Helper()
	metric, err := reader.GetMetric(name)
	require.NoError(t, err)
	require.Equal(t, unit, metric.Unit)
	gauge, ok := metric.Data.(metricdata.Gauge[float64])
	require.True(t, ok)
	require.Len(t, gauge.DataPoints, 1)
	require.Equal(t, attrs, gauge.DataPoints[0].Attributes)
	require.Equal(t, value, gauge.DataPoints[0].Value)
}

func requireCentralQueueIntSum(t *testing.T, reader *componenttest.Telemetry, name, unit string, attrs attribute.Set, value int64) {
	t.Helper()
	metric, err := reader.GetMetric(name)
	require.NoError(t, err)
	require.Equal(t, unit, metric.Unit)
	sum, ok := metric.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, sum.DataPoints, 1)
	require.Equal(t, attrs, sum.DataPoints[0].Attributes)
	require.Equal(t, value, sum.DataPoints[0].Value)
}

func requireCentralQueueIntHistogram(t *testing.T, reader *componenttest.Telemetry, name, unit string, attrs attribute.Set, value int64) {
	t.Helper()
	metric, err := reader.GetMetric(name)
	require.NoError(t, err)
	require.Equal(t, unit, metric.Unit)
	histogram, ok := metric.Data.(metricdata.Histogram[int64])
	require.True(t, ok)
	require.Len(t, histogram.DataPoints, 1)
	require.Equal(t, attrs, histogram.DataPoints[0].Attributes)
	require.Equal(t, value, histogram.DataPoints[0].Sum)
	require.EqualValues(t, 1, histogram.DataPoints[0].Count)
}
