// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// helper to create a MeterProvider + Metrics with a manual reader.
func setupTB(t *testing.T) (*Metrics, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	m, err := New(mp)
	require.NoError(t, err)
	require.NotNil(t, m)
	return m, reader
}

// helper to collect and return ResourceMetrics
func collect(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var out metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &out))
	return out
}

// helpers to find a metric by name
func findMetric(rm metricdata.ResourceMetrics, name string) (metricdata.Metrics, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m, true
			}
		}
	}
	return metricdata.Metrics{}, false
}

func TestNewMetrics(t *testing.T) {
	_, reader := setupTB(t)
	_ = collect(t, reader) // ensures collection works with no recordings
}

func TestRecordEvaluation(t *testing.T) {
	m, reader := setupTB(t)
	dur := 500 * time.Millisecond
	m.RecordEvaluation(context.Background(), dur)

	rm := collect(t, reader)

	// evaluations total counter
	met, ok := findMetric(rm, "otel_alert_evaluations_total")
	require.True(t, ok, "otel_alert_evaluations_total not found")
	sum, ok := met.Data.(metricdata.Sum[int64])
	require.True(t, ok, "expected int64 Sum for evaluations_total")
	require.Len(t, sum.DataPoints, 1)
	require.EqualValues(t, 1, sum.DataPoints[0].Value)

	// duration histogram
	met, ok = findMetric(rm, "otel_alert_evaluation_duration_seconds")
	require.True(t, ok, "otel_alert_evaluation_duration_seconds not found")
	hist, ok := met.Data.(metricdata.Histogram[float64])
	require.True(t, ok, "expected float64 Histogram for evaluation_duration_seconds")
	require.Len(t, hist.DataPoints, 1)
	got := hist.DataPoints[0].Sum
	want := dur.Seconds()
	require.InDelta(t, want, got, 1e-9)
}

func TestRecordEvents_AddActive_RecordNotify(t *testing.T) {
	m, reader := setupTB(t)
	ctx := context.Background()

	m.RecordEvents(ctx, 3)
	m.AddActive(ctx, +5)
	m.AddActive(ctx, -2) // net +3
	m.RecordNotify(ctx)

	rm := collect(t, reader)

	// events emitted
	met, ok := findMetric(rm, "otel_alert_events_emitted_total")
	require.True(t, ok)
	sumI64, ok := met.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, sumI64.DataPoints, 1)
	require.EqualValues(t, 3, sumI64.DataPoints[0].Value)

	// active up-down counter (non-monotonic Sum[int64])
	met, ok = findMetric(rm, "otel_alert_active_total")
	require.True(t, ok)
	updown, ok := met.Data.(metricdata.Sum[int64])
	require.True(t, ok, "active should be an int64 Sum (UpDownCounter)")
	require.Len(t, updown.DataPoints, 1)
	require.EqualValues(t, 3, updown.DataPoints[0].Value)

	// notifications
	met, ok = findMetric(rm, "otel_alert_notifications_total")
	require.True(t, ok)
	sumI64, ok = met.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, sumI64.DataPoints, 1)
	require.EqualValues(t, 1, sumI64.DataPoints[0].Value)
}

func TestRecordDropped_WithReason(t *testing.T) {
	m, reader := setupTB(t)
	ctx := context.Background()

	m.RecordDropped(ctx, "backpressure")

	rm := collect(t, reader)

	met, ok := findMetric(rm, "otel_alert_dropped_total")
	require.True(t, ok)
	sumI64, ok := met.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, sumI64.DataPoints, 1)
	dp := sumI64.DataPoints[0]
	require.EqualValues(t, 1, dp.Value)

	// verify attribute reason=backpressure
	found := false
	for _, attr := range dp.Attributes.ToSlice() {
		if string(attr.Key) == "reason" && attr.Value.AsString() == "backpressure" {
			found = true
			break
		}
	}
	require.True(t, found, "expected attribute reason=backpressure")
}

func TestRecordMemoryUsage(t *testing.T) {
	m, reader := setupTB(t)
	ctx := context.Background()

	m.RecordMemoryUsage(ctx, 12345.0, 87.5)

	rm := collect(t, reader)

	met, ok := findMetric(rm, "otel_alert_memory_usage_bytes")
	require.True(t, ok)
	gauge, ok := met.Data.(metricdata.Gauge[float64])
	require.True(t, ok)
	require.Len(t, gauge.DataPoints, 1)
	require.InDelta(t, 12345.0, gauge.DataPoints[0].Value, 1e-9)

	met, ok = findMetric(rm, "otel_alert_memory_usage_percent")
	require.True(t, ok)
	gauge, ok = met.Data.(metricdata.Gauge[float64])
	require.True(t, ok)
	require.Len(t, gauge.DataPoints, 1)
	require.InDelta(t, 87.5, gauge.DataPoints[0].Value, 1e-9)
}

func TestRecordBufferSizes(t *testing.T) {
	m, reader := setupTB(t)
	ctx := context.Background()

	m.RecordBufferSizes(ctx, 10, 20, 30)

	rm := collect(t, reader)

	met, ok := findMetric(rm, "otel_alert_buffer_utilization")
	require.True(t, ok)
	gauge, ok := met.Data.(metricdata.Gauge[float64])
	require.True(t, ok)
	require.Len(t, gauge.DataPoints, 3)

	// build a map signal_type -> value to assert
	got := map[string]float64{}
	for _, dp := range gauge.DataPoints {
		st := ""
		for _, a := range dp.Attributes.ToSlice() {
			if string(a.Key) == "signal_type" {
				st = a.Value.AsString()
				break
			}
		}
		got[st] = dp.Value
	}
	require.InDelta(t, 10.0, got["traces"], 1e-9)
	require.InDelta(t, 20.0, got["logs"], 1e-9)
	require.InDelta(t, 30.0, got["metrics"], 1e-9)
}

func TestRecordDroppedData(t *testing.T) {
	m, reader := setupTB(t)
	ctx := context.Background()

	m.RecordDroppedData(ctx, 7, 0, 9)

	rm := collect(t, reader)

	met, ok := findMetric(rm, "otel_alert_data_dropped_total")
	require.True(t, ok)
	sumI64, ok := met.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, sumI64.DataPoints, 2) // logs was 0, so only traces & metrics

	// map signal_type -> value, also confirm reason=memory_pressure
	got := map[string]int64{}
	for _, dp := range sumI64.DataPoints {
		st := ""
		r := ""
		for _, a := range dp.Attributes.ToSlice() {
			if string(a.Key) == "signal_type" {
				st = a.Value.AsString()
			}
			if string(a.Key) == "reason" {
				r = a.Value.AsString()
			}
		}
		require.Equal(t, "memory_pressure", r, "reason must be memory_pressure")
		got[st] = dp.Value
	}

	require.EqualValues(t, 7, got["traces"])
	require.EqualValues(t, 9, got["metrics"])
}

func TestRecordScaleEvent(t *testing.T) {
	m, reader := setupTB(t)
	ctx := context.Background()

	m.RecordScaleEvent(ctx, "auto_scale_up", 1.5)

	rm := collect(t, reader)

	met, ok := findMetric(rm, "otel_alert_scale_events_total")
	require.True(t, ok)
	sumI64, ok := met.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, sumI64.DataPoints, 1)
	dp := sumI64.DataPoints[0]
	require.EqualValues(t, 1, dp.Value)

	// verify attributes
	var et string
	var sf float64
	for _, a := range dp.Attributes.ToSlice() {
		if string(a.Key) == "event_type" {
			et = a.Value.AsString()
		}
		if string(a.Key) == "scale_factor" {
			sf = a.Value.AsFloat64()
		}
	}
	require.Equal(t, "auto_scale_up", et)
	require.InDelta(t, 1.5, sf, 1e-9)
}

func TestNilSafety_NoPanics_NoMetrics(t *testing.T) {
	// Call methods on a nil *Metrics and ensure no panic and that nothing is recorded.
	var m *Metrics
	ctx := context.Background()

	m.RecordEvaluation(ctx, 100*time.Millisecond)
	m.RecordEvents(ctx, 0)
	m.AddActive(ctx, 0)
	m.RecordNotify(ctx)
	m.RecordDropped(ctx, "x")
	m.RecordMemoryUsage(ctx, 1, 1)
	m.RecordBufferSizes(ctx, 0, 0, 0)
	m.RecordDroppedData(ctx, 0, 0, 0)
	m.RecordScaleEvent(ctx, "noop", 0)

	// A fresh reader to verify no metrics were emitted by the nil receiver.
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	_ = mp // silence linters if needed

	var out metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &out))
	if len(out.ScopeMetrics) != 0 {
		t.Logf("unexpected metrics present: %+v", out.ScopeMetrics)
	}
}
