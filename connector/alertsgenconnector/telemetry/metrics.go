
package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

// Metrics wraps OTel instruments for self-observability.
type Metrics struct {
	evalTotal     metric.Int64Counter
	evalDuration  metric.Float64Histogram
	eventsEmitted metric.Int64Counter
	notifyTotal   metric.Int64Counter
	activeGauge   metric.Int64UpDownCounter
	droppedTotal  metric.Int64Counter
}

// New creates a metrics bundle using the provided meter provider (global if nil).
func New(mp metric.MeterProvider) (*Metrics, error) {
	if mp == nil {
		mp = global.MeterProvider()
	}
	meter := mp.Meter("alertsgenconnector")

	evalTotal, err := meter.Int64Counter("otel_alert_evaluations_total",
		metric.WithDescription("Total number of rule evaluations"))
	if err != nil {
		return nil, err
	}

	evalDuration, err := meter.Float64Histogram("otel_alert_evaluation_duration_seconds",
		metric.WithDescription("Duration of rule evaluations in seconds"))
	if err != nil {
		return nil, err
	}

	eventsEmitted, err := meter.Int64Counter("otel_alert_events_emitted_total",
		metric.WithDescription("Total number of alert events emitted"))
	if err != nil {
		return nil, err
	}

	notifyTotal, err := meter.Int64Counter("otel_alert_notifications_total",
		metric.WithDescription("Total number of alert notification batches"))
	if err != nil {
		return nil, err
	}

	activeGauge, err := meter.Int64UpDownCounter("otel_alert_active_total",
		metric.WithDescription("Number of active firing alerts (up/down)"))
	if err != nil {
		return nil, err
	}

	droppedTotal, err := meter.Int64Counter("otel_alert_dropped_total",
		metric.WithDescription("Number of alerts dropped by limiter/dedup"))
	if err != nil {
		return nil, err
	}

	return &Metrics{
		evalTotal:     evalTotal,
		evalDuration:  evalDuration,
		eventsEmitted: eventsEmitted,
		notifyTotal:   notifyTotal,
		activeGauge:   activeGauge,
		droppedTotal:  droppedTotal,
	}, nil
}

func (m *Metrics) RecordEvaluation(ctx context.Context, rule string, status string, dur time.Duration) {
	if m == nil {
		return
	}
	m.evalTotal.Add(ctx, 1, metric.WithAttributes())
	m.evalDuration.Record(ctx, dur.Seconds(), metric.WithAttributes())
}

func (m *Metrics) RecordEvents(ctx context.Context, n int, rule string, sev string) {
	if m == nil || n <= 0 {
		return
	}
	m.eventsEmitted.Add(ctx, int64(n), metric.WithAttributes())
}

func (m *Metrics) AddActive(ctx context.Context, delta int, rule string, sev string) {
	if m == nil || delta == 0 {
		return
	}
	m.activeGauge.Add(ctx, int64(delta), metric.WithAttributes())
}

func (m *Metrics) RecordNotify(ctx context.Context, ok bool) {
	if m == nil {
		return
	}
	m.notifyTotal.Add(ctx, 1, metric.WithAttributes())
}

func (m *Metrics) RecordDropped(ctx context.Context, n int, reason string) {
	if m == nil || n <= 0 {
		return
	}
	m.droppedTotal.Add(ctx, int64(n), metric.WithAttributes())
}
