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
	signal               signalKind
	signalAttrs          metric.MeasurementOption
	compressedBytes      metric.Int64Gauge
	compressedCapacity   metric.Int64Gauge
	saturation           metric.Float64Gauge
	items                metric.Int64Gauge
	rejectedBytes        metric.Int64Counter
	retries              metric.Int64Counter
	decodeFailures       metric.Int64Counter
	inflightUncompressed metric.Int64Gauge
	windowCompressed     metric.Int64Histogram
	windowFlush          metric.Int64Counter
	windowUncompressed   metric.Int64Histogram
	windowItems          metric.Int64Histogram
	windowPayloads       metric.Int64Histogram
	windowUnderfilled    metric.Int64Counter
	oldestItemAge        metric.Int64ObservableGauge
	oldestItemAgeReg     metric.Registration
	oldestItemAgeMu      sync.RWMutex
	oldestItemAgeMillis  func() int64
	flushReasonAttrs     map[centralQueueFlushReason]metric.MeasurementOption
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
	signalAttr := attribute.String("signal", string(signal))
	t := &centralQueueTelemetry{
		signal:      signal,
		signalAttrs: metric.WithAttributeSet(attribute.NewSet(signalAttr)),
		flushReasonAttrs: map[centralQueueFlushReason]metric.MeasurementOption{
			centralQueueFlushReasonTargetReached:      metric.WithAttributeSet(attribute.NewSet(signalAttr, attribute.String("reason", string(centralQueueFlushReasonTargetReached)))),
			centralQueueFlushReasonHardCap:            metric.WithAttributeSet(attribute.NewSet(signalAttr, attribute.String("reason", string(centralQueueFlushReasonHardCap)))),
			centralQueueFlushReasonMaxDelayLowTraffic: metric.WithAttributeSet(attribute.NewSet(signalAttr, attribute.String("reason", string(centralQueueFlushReasonMaxDelayLowTraffic)))),
			centralQueueFlushReasonShutdown:           metric.WithAttributeSet(attribute.NewSet(signalAttr, attribute.String("reason", string(centralQueueFlushReasonShutdown)))),
		},
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
	t.decodeFailures, err = meter.Int64Counter(
		"otelcol_loadbalancer_central_queue_decode_failures",
		metric.WithDescription("Log records or metric datapoints dropped after central queue payload decode failures."),
		metric.WithUnit("{items}"),
	)
	errs = errors.Join(errs, err)
	t.inflightUncompressed, err = meter.Int64Gauge(
		"otelcol_loadbalancer_central_queue_inflight_uncompressed_bytes",
		metric.WithDescription("Uncompressed bytes currently leased from the central load-balancing queue."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.windowCompressed, err = meter.Int64Histogram(
		"otelcol_loadbalancer_central_queue_window_compressed_bytes",
		metric.WithDescription("Compressed bytes in each central load-balancing queue window before decode and send."),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(1024, 4096, 16384, 65536, 262144, 1048576, 4194304),
	)
	errs = errors.Join(errs, err)
	t.windowFlush, err = meter.Int64Counter(
		"otelcol_loadbalancer_central_queue_window_flush_total",
		metric.WithDescription("Central load-balancing queue windows flushed by bounded reason."),
		metric.WithUnit("{windows}"),
	)
	errs = errors.Join(errs, err)
	t.windowUncompressed, err = meter.Int64Histogram(
		"otelcol_loadbalancer_central_queue_window_uncompressed_bytes",
		metric.WithDescription("Estimated uncompressed OTLP bytes in each central load-balancing queue window before decode and send."),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216),
	)
	errs = errors.Join(errs, err)
	t.windowItems, err = meter.Int64Histogram(
		"otelcol_loadbalancer_central_queue_window_items",
		metric.WithDescription("Log records or metric datapoints in each central load-balancing queue window."),
		metric.WithUnit("{items}"),
		metric.WithExplicitBucketBoundaries(1, 10, 50, 100, 500, 1000, 5000, 10000, 50000),
	)
	errs = errors.Join(errs, err)
	t.windowPayloads, err = meter.Int64Histogram(
		"otelcol_loadbalancer_central_queue_window_payloads",
		metric.WithDescription("Compressed queue payloads merged into each central load-balancing queue window."),
		metric.WithUnit("{payloads}"),
		metric.WithExplicitBucketBoundaries(1, 2, 4, 8, 16, 32, 64, 128),
	)
	errs = errors.Join(errs, err)
	t.windowUnderfilled, err = meter.Int64Counter(
		"otelcol_loadbalancer_central_queue_window_underfilled_total",
		metric.WithDescription("Central load-balancing queue windows sent below target compressed bytes by bounded reason."),
		metric.WithUnit("{windows}"),
	)
	errs = errors.Join(errs, err)
	t.oldestItemAge, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_central_queue_oldest_item_age",
		metric.WithDescription("Age in ms of the oldest queued central load-balancing item."),
		metric.WithUnit("ms"),
	)
	errs = errors.Join(errs, err)
	if err == nil {
		t.oldestItemAgeReg, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
			t.oldestItemAgeMu.RLock()
			oldestItemAgeMillis := t.oldestItemAgeMillis
			t.oldestItemAgeMu.RUnlock()
			if oldestItemAgeMillis != nil {
				observer.ObserveInt64(t.oldestItemAge, oldestItemAgeMillis(), t.signalAttrs)
			}
			return nil
		}, t.oldestItemAge)
		errs = errors.Join(errs, err)
	}
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

func (t *centralQueueTelemetry) recordDecodeFailure(ctx context.Context, droppedItems int64) {
	if t == nil || droppedItems <= 0 {
		return
	}
	t.decodeFailures.Add(ctx, droppedItems, t.signalAttrs)
}

func (t *centralQueueTelemetry) recordWindow(ctx context.Context, window centralQueueWindow, targetCompressedBytes int64) {
	if t == nil {
		return
	}
	t.windowCompressed.Record(ctx, int64(window.compressedBytes), t.signalAttrs)
	t.windowUncompressed.Record(ctx, int64(window.uncompressedBytes), t.signalAttrs)
	t.windowItems.Record(ctx, int64(window.count), t.signalAttrs)
	t.windowPayloads.Record(ctx, int64(len(window.items)), t.signalAttrs)
	reasonAttrs := t.flushAttrs(window.flushReason)
	t.windowFlush.Add(ctx, 1, reasonAttrs)
	if int64(window.compressedBytes) < targetCompressedBytes {
		t.windowUnderfilled.Add(ctx, 1, reasonAttrs)
	}
}

func (t *centralQueueTelemetry) flushAttrs(reason centralQueueFlushReason) metric.MeasurementOption {
	if attrs, ok := t.flushReasonAttrs[reason]; ok {
		return attrs
	}
	return metric.WithAttributeSet(attribute.NewSet(attribute.String("signal", string(t.signal)), attribute.String("reason", string(reason))))
}

func (t *centralQueueTelemetry) observeOldestItemAge(oldestItemAgeMillis func() int64) {
	if t == nil {
		return
	}
	t.oldestItemAgeMu.Lock()
	defer t.oldestItemAgeMu.Unlock()
	t.oldestItemAgeMillis = oldestItemAgeMillis
}

func (t *centralQueueTelemetry) stopObservingOldestItemAge() {
	if t == nil {
		return
	}
	t.oldestItemAgeMu.Lock()
	registration := t.oldestItemAgeReg
	t.oldestItemAgeReg = nil
	t.oldestItemAgeMillis = nil
	t.oldestItemAgeMu.Unlock()
	if registration != nil {
		_ = registration.Unregister()
	}
}
