// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/notify"

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metric instrument names. Exact strings from the spec.
const (
	MetricNameSent         = "notifications.sent"
	MetricNameDropped      = "notifications.dropped"
	MetricNameSendDuration = "notifications.send.duration"
)

// Attribute keys.
const (
	attrOutcome     = "outcome"
	attrReason      = "reason"
	attrStatusClass = "status_class"
)

// outcomeSuccess is the only outcome this package records on the sent
// counter. It exists to make the attribute contract explicit.
const outcomeSuccess = "success"

// Reasons surfaced on the dropped counter. Every place in this package that
// records a drop must use one of these constants so the attribute space
// stays bounded and operators can build deterministic alerts.
const (
	ReasonQueueFull        = "queue_full"
	ReasonPermanent4xx     = "permanent_4xx"
	ReasonRetriesExhausted = "retries_exhausted"
	ReasonShutdown         = "shutdown"
)

// Status classes surfaced on the duration histogram. One sample is recorded
// per HTTP round-trip attempt (including retries); the class reflects the
// outcome of that attempt rather than the final batch disposition.
const (
	StatusClass2xx          = "2xx"
	StatusClass4xx          = "4xx"
	StatusClass5xx          = "5xx"
	StatusClassNetworkError = "network_error"
)

// instruments bundles the hand-rolled OTel metrics surfaced by the notifier.
// The zero value is unusable; use newInstruments instead.
type instruments struct {
	sent     metric.Int64Counter
	dropped  metric.Int64Counter
	duration metric.Float64Histogram
}

func newInstruments(meter metric.Meter) (*instruments, error) {
	sent, err := meter.Int64Counter(
		MetricNameSent,
		metric.WithDescription("Successfully delivered S3 upload notifications, counted per record."),
	)
	if err != nil {
		return nil, err
	}
	dropped, err := meter.Int64Counter(
		MetricNameDropped,
		metric.WithDescription("Notifications that did not reach the receiver, counted per record."),
	)
	if err != nil {
		return nil, err
	}
	duration, err := meter.Float64Histogram(
		MetricNameSendDuration,
		metric.WithDescription("Per-attempt HTTP POST duration, in seconds."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}
	return &instruments{sent: sent, dropped: dropped, duration: duration}, nil
}

// recordSent records n successful per-record deliveries. The caller supplies
// the count (not 1) because batches are sent as a unit and the counter is
// defined per-record so operators can directly reason about event rates.
func (i *instruments) recordSent(ctx context.Context, n int64) {
	i.sent.Add(ctx, n, metric.WithAttributes(attribute.String(attrOutcome, outcomeSuccess)))
}

// recordDropped records n drops with the supplied reason attribute. Use one
// of the Reason* constants in this file; anything else is a caller bug.
func (i *instruments) recordDropped(ctx context.Context, n int64, reason string) {
	i.dropped.Add(ctx, n, metric.WithAttributes(attribute.String(attrReason, reason)))
}

// recordDuration records one HTTP attempt's wall-clock duration with the
// supplied status_class attribute. Use one of the StatusClass* constants.
func (i *instruments) recordDuration(ctx context.Context, d time.Duration, statusClass string) {
	i.duration.Record(ctx, d.Seconds(), metric.WithAttributes(attribute.String(attrStatusClass, statusClass)))
}
