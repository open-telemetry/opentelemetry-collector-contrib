// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

type centralQueueTelemetry struct {
	signalAttrs          metric.MeasurementOption
	compressedBytes      metric.Int64Gauge
	compressedCapacity   metric.Int64Gauge
	saturation           metric.Float64Gauge
	items                metric.Int64Gauge
	rejectedBytes        metric.Int64Counter
	retries              metric.Int64Counter
	inflightUncompressed metric.Int64Gauge
	oldestItemAge        metric.Int64ObservableGauge
	oldestItemAgeMu      sync.RWMutex
	oldestItemAgeMillis  func() int64
}

type centralQueueSnapshot struct {
	compressedBytes      int64
	compressedCapacity   int64
	items                int64
	inflightUncompressed int64
	oldestItemAgeMillis  int64
}

func newCentralQueueTelemetry(settings component.TelemetrySettings, signal signalKind) (*centralQueueTelemetry, error) {
	meter := metadata.Meter(settings)
	var err, errs error
	t := &centralQueueTelemetry{
		signalAttrs: metric.WithAttributeSet(attribute.NewSet(attribute.String("signal", string(signal)))),
	}
	t.compressedBytes, err = meter.Int64Gauge(
		"otelcol_loadbalancer_central_queue_compressed_bytes",
		metric.WithDescription("Current compressed bytes in the central load-balancing queue."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.compressedCapacity, err = meter.Int64Gauge(
		"otelcol_loadbalancer_central_queue_compressed_capacity",
		metric.WithDescription("Configured compressed byte capacity of the central load-balancing queue."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.saturation, err = meter.Float64Gauge(
		"otelcol_loadbalancer_central_queue_saturation",
		metric.WithDescription("Central load-balancing queue compressed byte saturation."),
		metric.WithUnit("1"),
	)
	errs = errors.Join(errs, err)
	t.items, err = meter.Int64Gauge(
		"otelcol_loadbalancer_central_queue_items",
		metric.WithDescription("Current items in the central load-balancing queue."),
		metric.WithUnit("{items}"),
	)
	errs = errors.Join(errs, err)
	t.rejectedBytes, err = meter.Int64Counter(
		"otelcol_loadbalancer_central_queue_rejected_compressed_bytes",
		metric.WithDescription("Compressed bytes rejected by the central load-balancing queue."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.retries, err = meter.Int64Counter(
		"otelcol_loadbalancer_central_queue_retries",
		metric.WithDescription("Central load-balancing queue item retries."),
		metric.WithUnit("{retries}"),
	)
	errs = errors.Join(errs, err)
	t.inflightUncompressed, err = meter.Int64Gauge(
		"otelcol_loadbalancer_central_queue_inflight_uncompressed_bytes",
		metric.WithDescription("Uncompressed bytes currently leased from the central load-balancing queue."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.oldestItemAge, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_central_queue_oldest_item_age",
		metric.WithDescription("Age in ms of the oldest queued central load-balancing item."),
		metric.WithUnit("ms"),
		metric.WithInt64Callback(func(_ context.Context, observer metric.Int64Observer) error {
			t.oldestItemAgeMu.RLock()
			oldestItemAgeMillis := t.oldestItemAgeMillis
			t.oldestItemAgeMu.RUnlock()
			if oldestItemAgeMillis != nil {
				observer.Observe(oldestItemAgeMillis(), t.signalAttrs)
			}
			return nil
		}),
	)
	errs = errors.Join(errs, err)
	return t, errs
}

func (t *centralQueueTelemetry) record(ctx context.Context, snapshot centralQueueSnapshot) {
	if t == nil {
		return
	}
	t.compressedBytes.Record(ctx, snapshot.compressedBytes, t.signalAttrs)
	t.compressedCapacity.Record(ctx, snapshot.compressedCapacity, t.signalAttrs)
	if snapshot.compressedCapacity > 0 {
		t.saturation.Record(ctx, float64(snapshot.compressedBytes)/float64(snapshot.compressedCapacity), t.signalAttrs)
	}
	t.items.Record(ctx, snapshot.items, t.signalAttrs)
	t.inflightUncompressed.Record(ctx, snapshot.inflightUncompressed, t.signalAttrs)
}

func (t *centralQueueTelemetry) recordRejected(ctx context.Context, compressedBytes int64) {
	if t == nil || compressedBytes <= 0 {
		return
	}
	t.rejectedBytes.Add(ctx, compressedBytes, t.signalAttrs)
}

func (t *centralQueueTelemetry) recordRetry(ctx context.Context) {
	if t == nil {
		return
	}
	t.retries.Add(ctx, 1, t.signalAttrs)
}

func (t *centralQueueTelemetry) observeOldestItemAge(oldestItemAgeMillis func() int64) {
	if t == nil {
		return
	}
	t.oldestItemAgeMu.Lock()
	defer t.oldestItemAgeMu.Unlock()
	t.oldestItemAgeMillis = oldestItemAgeMillis
}
