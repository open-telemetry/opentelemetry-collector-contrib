// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/telemetry"

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	// Core alerting metrics
	evalTotal     metric.Int64Counter
	evalDuration  metric.Float64Histogram
	eventsEmitted metric.Int64Counter
	activeGauge   metric.Int64UpDownCounter
	notifyTotal   metric.Int64Counter
	droppedTotal  metric.Int64Counter

	// Resource & runtime
	memoryUsageBytes   metric.Float64Gauge
	memoryUsagePercent metric.Float64Gauge
	bufferUtilization  metric.Float64Gauge
	droppedDataCounter metric.Int64Counter
	scaleEventsCounter metric.Int64Counter
}

// New creates and registers all metric instruments
func New(mp metric.MeterProvider) (*Metrics, error) {
	meter := mp.Meter("alertsgenconnector")

	evalTotal, err := meter.Int64Counter("otel_alert_evaluations_total",
		metric.WithDescription("Total number of rule evaluation passes"))
	if err != nil {
		return nil, err
	}

	evalDuration, err := meter.Float64Histogram("otel_alert_evaluation_duration_seconds",
		metric.WithDescription("Wall time of a rule evaluation pass in seconds"))
	if err != nil {
		return nil, err
	}

	eventsEmitted, err := meter.Int64Counter("otel_alert_events_emitted_total",
		metric.WithDescription("Number of alert events emitted"))
	if err != nil {
		return nil, err
	}

	activeGauge, err := meter.Int64UpDownCounter("otel_alert_active_total",
		metric.WithDescription("Active firing alerts (up/down)"))
	if err != nil {
		return nil, err
	}

	notifyTotal, err := meter.Int64Counter("otel_alert_notifications_total",
		metric.WithDescription("Number of notification batches sent"))
	if err != nil {
		return nil, err
	}

	droppedTotal, err := meter.Int64Counter("otel_alert_dropped_total",
		metric.WithDescription("Alerts dropped by limiter/dedup"))
	if err != nil {
		return nil, err
	}

	memoryUsageBytes, err := meter.Float64Gauge("otel_alert_memory_usage_bytes",
		metric.WithDescription("Current memory usage in bytes"))
	if err != nil {
		return nil, err
	}

	memoryUsagePercent, err := meter.Float64Gauge("otel_alert_memory_usage_percent",
		metric.WithDescription("Current memory usage as percentage of limit"))
	if err != nil {
		return nil, err
	}

	bufferUtilization, err := meter.Float64Gauge("otel_alert_buffer_utilization",
		metric.WithDescription("Buffer utilization by signal type"))
	if err != nil {
		return nil, err
	}

	droppedDataCounter, err := meter.Int64Counter("otel_alert_data_dropped_total",
		metric.WithDescription("Data dropped due to memory pressure or limits"))
	if err != nil {
		return nil, err
	}

	scaleEventsCounter, err := meter.Int64Counter("otel_alert_scale_events_total",
		metric.WithDescription("Buffer scaling events"))
	if err != nil {
		return nil, err
	}

	return &Metrics{
		evalTotal:          evalTotal,
		evalDuration:       evalDuration,
		eventsEmitted:      eventsEmitted,
		activeGauge:        activeGauge,
		notifyTotal:        notifyTotal,
		droppedTotal:       droppedTotal,
		memoryUsageBytes:   memoryUsageBytes,
		memoryUsagePercent: memoryUsagePercent,
		bufferUtilization:  bufferUtilization,
		droppedDataCounter: droppedDataCounter,
		scaleEventsCounter: scaleEventsCounter,
	}, nil
}

// ---- Recording methods ----

func (m *Metrics) RecordEvaluation(ctx context.Context, dur time.Duration) {
	if m == nil {
		return
	}
	m.evalTotal.Add(ctx, 1)
	m.evalDuration.Record(ctx, dur.Seconds())
}

func (m *Metrics) RecordEvents(ctx context.Context, n int) {
	if m == nil || n <= 0 {
		return
	}
	m.eventsEmitted.Add(ctx, int64(n))
}

func (m *Metrics) AddActive(ctx context.Context, delta int) {
	if m == nil || delta == 0 {
		return
	}
	m.activeGauge.Add(ctx, int64(delta))
}

func (m *Metrics) RecordNotify(ctx context.Context) {
	if m == nil {
		return
	}
	m.notifyTotal.Add(ctx, 1)
}

func (m *Metrics) RecordDropped(ctx context.Context, reason string) {
	if m == nil {
		return
	}
	m.droppedTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("reason", reason)))
}

func (m *Metrics) RecordMemoryUsage(ctx context.Context, current, percent float64) {
	if m == nil {
		return
	}
	m.memoryUsageBytes.Record(ctx, current)
	m.memoryUsagePercent.Record(ctx, percent)
}

func (m *Metrics) RecordBufferSizes(ctx context.Context, traces, logs, metrics int) {
	if m == nil {
		return
	}
	m.bufferUtilization.Record(ctx, float64(traces),
		metric.WithAttributes(attribute.String("signal_type", "traces")))
	m.bufferUtilization.Record(ctx, float64(logs),
		metric.WithAttributes(attribute.String("signal_type", "logs")))
	m.bufferUtilization.Record(ctx, float64(metrics),
		metric.WithAttributes(attribute.String("signal_type", "metrics")))
}

func (m *Metrics) RecordDroppedData(ctx context.Context, droppedTraces, droppedLogs, droppedMetrics int64) {
	if m == nil {
		return
	}
	if droppedTraces > 0 {
		m.droppedDataCounter.Add(ctx, droppedTraces,
			metric.WithAttributes(
				attribute.String("signal_type", "traces"),
				attribute.String("reason", "memory_pressure"),
			))
	}
	if droppedLogs > 0 {
		m.droppedDataCounter.Add(ctx, droppedLogs,
			metric.WithAttributes(
				attribute.String("signal_type", "logs"),
				attribute.String("reason", "memory_pressure"),
			))
	}
	if droppedMetrics > 0 {
		m.droppedDataCounter.Add(ctx, droppedMetrics,
			metric.WithAttributes(
				attribute.String("signal_type", "metrics"),
				attribute.String("reason", "memory_pressure"),
			))
	}
}

func (m *Metrics) RecordScaleEvent(ctx context.Context, eventType string, scaleFactor float64) {
	if m == nil {
		return
	}
	m.scaleEventsCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("event_type", eventType),
			attribute.Float64("scale_factor", scaleFactor),
		))
}
