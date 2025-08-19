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
	notifyErrors  metric.Int64Counter
	droppedTotal  metric.Int64Counter
	activeAlerts  metric.Int64UpDownCounter
}

// New initializes metrics using provided meter provider.
func New(meterProvider metric.MeterProvider) (*Metrics, error) {
	if meterProvider == nil {
		meterProvider = global.MeterProvider()
	}
	meter := meterProvider.Meter("otelcol.connector.alertsgen")

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
		metric.WithDescription("Total number of alert notifications sent"))
	if err != nil {
		return nil, err
	}

	notifyErrors, err := meter.Int64Counter("otel_alert_notification_errors_total",
		metric.WithDescription("Total number of alert notification errors"))
	if err != nil {
		return nil, err
	}

	droppedTotal, err := meter.Int64Counter("otel_alert_dropped_total",
		metric.WithDescription("Total number of alerts dropped"))
	if err != nil {
		return nil, err
	}

	activeAlerts, err := meter.Int64UpDownCounter("otel_alert_active_total",
		metric.WithDescription("Number of active alerts"))
	if err != nil {
		return nil, err
	}

	return &Metrics{
		evalTotal:     evalTotal,
		evalDuration:  evalDuration,
		eventsEmitted: eventsEmitted,
		notifyTotal:   notifyTotal,
		notifyErrors:  notifyErrors,
		droppedTotal:  droppedTotal,
		activeAlerts:  activeAlerts,
	}, nil
}

// --- Helper methods ---

func (m *Metrics) RecordEvaluation(ctx context.Context, ruleID string, result string, duration time.Duration) {
	if m == nil {
		return
	}
	labels := []metric.AddOption{
		metric.WithAttributes(),
	}
	m.evalTotal.Add(ctx, 1, labels...)
	m.evalDuration.Record(ctx, duration.Seconds(), labels...)
}

func (m *Metrics) RecordEvents(ctx context.Context, count int, ruleID string, state string) {
	if m == nil {
		return
	}
	m.eventsEmitted.Add(ctx, int64(count))
}

func (m *Metrics) RecordNotify(ctx context.Context, success bool) {
	if m == nil {
		return
	}
	if success {
		m.notifyTotal.Add(ctx, 1)
	} else {
		m.notifyErrors.Add(ctx, 1)
	}
}

func (m *Metrics) RecordDrop(ctx context.Context, count int) {
	if m == nil {
		return
	}
	m.droppedTotal.Add(ctx, int64(count))
}

func (m *Metrics) SetActive(ctx context.Context, delta int) {
	if m == nil {
		return
	}
	m.activeAlerts.Add(ctx, int64(delta))
}
