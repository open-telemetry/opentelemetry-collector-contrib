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

	telemetry.record(t.Context(), centralQueueSnapshot{
		compressedBytes:      50,
		compressedCapacity:   100,
		items:                3,
		inflightUncompressed: 80,
	})
	telemetry.recordRejected(t.Context(), 7)
	telemetry.recordRetry(t.Context())

	attrs := attribute.NewSet(attribute.String("signal", string(signalKindLogs)))
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_compressed_bytes", "By", attrs, 50)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_compressed_capacity", "By", attrs, 100)
	requireCentralQueueFloatGauge(t, reader, "otelcol_loadbalancer_central_queue_saturation", "1", attrs, 0.5)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_items", "{items}", attrs, 3)
	requireCentralQueueIntGauge(t, reader, "otelcol_loadbalancer_central_queue_inflight_uncompressed_bytes", "By", attrs, 80)
	requireCentralQueueIntSum(t, reader, "otelcol_loadbalancer_central_queue_rejected_compressed_bytes", "By", attrs, 7)
	requireCentralQueueIntSum(t, reader, "otelcol_loadbalancer_central_queue_retries", "{retries}", attrs, 1)
}

func requireCentralQueueIntGauge(t *testing.T, reader *componenttest.Telemetry, name, unit string, attrs attribute.Set, value int64) {
	t.Helper()
	metric, err := reader.GetMetric(name)
	require.NoError(t, err)
	require.Equal(t, unit, metric.Unit)
	gauge, ok := metric.Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Len(t, gauge.DataPoints, 1)
	require.Equal(t, attrs, gauge.DataPoints[0].Attributes)
	require.Equal(t, value, gauge.DataPoints[0].Value)
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
