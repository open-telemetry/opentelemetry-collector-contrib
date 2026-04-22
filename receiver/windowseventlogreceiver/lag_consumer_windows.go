// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowseventlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"

import (
	"context"
	"math"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
)

// lagTrackingConsumer wraps a logs consumer to record the receiver lag metric.
// Lag is defined as the maximum difference between ObservedTimestamp and Timestamp
// across all log records in a batch, where Timestamp is the original Windows event
// creation time (TimeCreated/@SystemTime) and ObservedTimestamp is when the collector
// ingested the event.
//
// The metric is implemented as an observable (async) gauge: the stored lag is atomically
// swapped to zero on each metric collection. This ensures the gauge reads zero when the
// receiver is idle or fully caught up, rather than holding the last non-zero value.
type lagTrackingConsumer struct {
	next             consumer.Logs
	telemetryBuilder *metadata.TelemetryBuilder
	channel          string
	currentLag       atomic.Uint64 // float64 bits via math.Float64bits
}

func newLagTrackingConsumer(next consumer.Logs, tb *metadata.TelemetryBuilder, channel string, cfg MetricsConfig) (*lagTrackingConsumer, error) {
	lc := &lagTrackingConsumer{next: next, telemetryBuilder: tb, channel: channel}
	if cfg.ReceiverWindowsEventLogLag.Enabled {
		err := tb.RegisterReceiverWindowsEventLogLagCallback(func(_ context.Context, o metric.Float64Observer) error {
			// Atomically read and reset to 0. If no batches arrived since the last
			// collection, this reports 0, signaling the receiver is caught up.
			bits := lc.currentLag.Swap(0)
			o.Observe(math.Float64frombits(bits), metric.WithAttributes(attribute.String("channel", lc.channel)))
			return nil
		})
		return lc, err
	}
	return lc, nil
}

// Capabilities returns the consumer capabilities of the wrapped consumer.
func (l *lagTrackingConsumer) Capabilities() consumer.Capabilities {
	return l.next.Capabilities()
}

// ConsumeLogs records the maximum lag across all log records in the batch, then
// forwards the logs to the wrapped consumer.
func (l *lagTrackingConsumer) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	now := time.Now()
	var maxLag float64

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				if lr.Timestamp() == 0 {
					continue
				}
				ts := lr.Timestamp().AsTime()
				var obs time.Time
				if lr.ObservedTimestamp() == 0 {
					obs = now
				} else {
					obs = lr.ObservedTimestamp().AsTime()
				}
				if lag := obs.Sub(ts).Seconds(); lag > maxLag {
					maxLag = lag
				}
			}
		}
	}

	if maxLag > 0 {
		l.currentLag.Store(math.Float64bits(maxLag))
	}

	return l.next.ConsumeLogs(ctx, logs)
}
