// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

const (
	metricFlushReasonSize           = "size"
	metricFlushReasonTimeout        = "timeout"
	metricFlushReasonShutdown       = "shutdown"
	metricFlushReasonResolverChange = "resolver_change"

	defaultMetricBatchMaxDataPoints         = 8192
	defaultMetricBatchMaxBytes              = 2 << 20
	defaultMetricBatchFlushTimeout          = 200 * time.Millisecond
	defaultMetricBatchRetryBufferMultiplier = 10 // allow retries to grow up to 10x configured batch limits before forced drop
	maxMetricBatchRetryBackoff              = 5 * time.Second
)

var metricBatcherMaxRetryAge = 2 * time.Minute

var errMetricBatcherExporterStopping = errors.New("metric batcher exporter is stopping")

type metricBatcherSettings struct {
	maxDataPoints            int
	maxBytes                 int
	flushInterval            time.Duration
	maxRetryBufferMultiplier int
	payloadCompression       QueuePayloadCompression
}

type (
	metricBatcherSendFunc         func(context.Context, *wrappedExporter, pmetric.Metrics, string) error
	metricBatcherDrainFailureFunc func(context.Context, pmetric.Metrics, string) error
)

type metricBatcher struct {
	logger   *zap.Logger
	settings metricBatcherSettings
	send     metricBatcherSendFunc
	drainErr metricBatcherDrainFailureFunc

	telemetry *metricBatcherTelemetry

	mu       sync.RWMutex
	backends map[string]*backendMetricBatcher
	stopped  atomic.Bool
}

type metricBatcherRequest struct {
	kind               metricBatcherRequestKind
	md                 pmetric.Metrics
	compressedChunk    compressedMetricBatcherChunk
	ctx                context.Context
	reason             string
	done               chan error
	enqueuedAtUnixNano int64
}

type metricBatcherRequestKind int

const (
	metricBatcherRequestEnqueue metricBatcherRequestKind = iota
	metricBatcherRequestFlushAndStop
)

type backendMetricBatcher struct {
	endpoint string
	logger   *zap.Logger
	settings metricBatcherSettings
	send     metricBatcherSendFunc
	drainErr metricBatcherDrainFailureFunc

	telemetry *metricBatcherTelemetry

	exporterMu sync.RWMutex
	exp        *wrappedExporter

	requests chan metricBatcherRequest
	done     chan struct{}
	inflight sync.WaitGroup

	pendingDataPoints      atomic.Int64
	pendingBytes           atomic.Int64
	pendingCompressedBytes atomic.Int64
	oldestEnqueue          atomic.Int64

	flushFailures int

	payloadCodecMu sync.Mutex
	payloadCodec   *queuePayloadCodec
}

type metricBatcherTelemetry struct {
	logger                 *zap.Logger
	meter                  metric.Meter
	batchDataPoints        metric.Int64Histogram
	batchBytes             metric.Int64Histogram
	flushTotal             metric.Int64Counter
	flushErrors            metric.Int64Counter
	flushOldestPointAge    metric.Int64Histogram
	droppedDataPoints      metric.Int64Counter
	overflowTotal          metric.Int64Counter
	pendingDataPoints      metric.Int64ObservableGauge
	pendingBytes           metric.Int64ObservableGauge
	pendingCompressedBytes metric.Int64ObservableGauge
	pendingOldestPointAge  metric.Int64ObservableGauge
	pendingOldestPointMax  metric.Int64ObservableGauge

	mu            sync.Mutex
	registrations []metric.Registration
}

func newMetricBatcher(
	logger *zap.Logger,
	settings component.TelemetrySettings,
	cfg metricBatcherSettings,
	send metricBatcherSendFunc,
	drainErr metricBatcherDrainFailureFunc,
) (*metricBatcher, error) {
	if cfg.maxRetryBufferMultiplier <= 0 {
		cfg.maxRetryBufferMultiplier = defaultMetricBatchRetryBufferMultiplier
	}

	telemetry, err := newMetricBatcherTelemetry(settings)
	if err != nil {
		return nil, err
	}

	b := &metricBatcher{
		logger:    logger,
		settings:  cfg,
		send:      send,
		drainErr:  drainErr,
		telemetry: telemetry,
		backends:  make(map[string]*backendMetricBatcher),
	}

	if err := telemetry.start(b.snapshotPending); err != nil {
		return nil, err
	}

	return b, nil
}

// TryEnqueue transfers ownership of md to the backend batcher when it returns (true, nil).
// Callers must treat md as immutable/invalid after successful enqueue.
func (b *metricBatcher) TryEnqueue(endpoint string, exp *wrappedExporter, md pmetric.Metrics) (bool, error) {
	backend, err := b.acquireBackend(endpoint, exp)
	if err != nil {
		return false, err
	}
	defer backend.inflight.Done()

	select {
	case <-backend.done:
		return false, errMetricBatcherExporterStopping
	default:
	}
	if cap(backend.requests) > 0 && len(backend.requests) >= cap(backend.requests) {
		return false, nil
	}

	enqueuedAtUnixNano := time.Now().UnixNano()
	req := metricBatcherRequest{kind: metricBatcherRequestEnqueue, md: md, enqueuedAtUnixNano: enqueuedAtUnixNano}
	if backend.payloadCodec != nil {
		compressedChunk, err := backend.newCompressedMetricBatcherChunk(md, enqueuedAtUnixNano)
		if err != nil {
			return false, err
		}
		req.md = pmetric.NewMetrics()
		req.compressedChunk = compressedChunk
	}
	select {
	case backend.requests <- req:
		return true, nil
	case <-backend.done:
		return false, errMetricBatcherExporterStopping
	default:
		return false, nil
	}
}

func (b *metricBatcher) Remove(ctx context.Context, endpoint string, exp *wrappedExporter) error {
	b.mu.Lock()
	backend, ok := b.backends[endpoint]
	if ok {
		delete(b.backends, endpoint)
	}
	b.mu.Unlock()
	if !ok {
		return nil
	}
	if exp != nil {
		backend.setExporter(exp)
	}
	if err := waitForInflight(ctx, &backend.inflight); err != nil {
		b.scheduleBackendCleanup(backend, metricFlushReasonResolverChange)
		return err
	}
	return backend.stopAndFlush(ctx, metricFlushReasonResolverChange)
}

func (b *metricBatcher) Shutdown(ctx context.Context) error {
	b.stopped.Store(true)

	b.mu.Lock()
	backends := make([]*backendMetricBatcher, 0, len(b.backends))
	for endpoint, backend := range b.backends {
		backends = append(backends, backend)
		delete(b.backends, endpoint)
	}
	b.mu.Unlock()

	var errs error
	for _, backend := range backends {
		if err := waitForInflight(ctx, &backend.inflight); err != nil {
			b.scheduleBackendCleanup(backend, metricFlushReasonShutdown)
			errs = errors.Join(errs, err)
			continue
		}
		errs = errors.Join(errs, backend.stopAndFlush(ctx, metricFlushReasonShutdown))
	}
	b.telemetry.shutdown()
	return errs
}

func (b *metricBatcher) scheduleBackendCleanup(backend *backendMetricBatcher, reason string) {
	go func() {
		if err := waitForInflight(context.Background(), &backend.inflight); err != nil {
			b.logger.Warn("failed waiting for inflight metric batcher requests during background cleanup", zap.String("endpoint", backend.endpoint), zap.Error(err))
			return
		}
		if err := backend.stopAndFlush(context.Background(), reason); err != nil {
			b.logger.Error("failed to stop metric batcher backend during background cleanup", zap.String("endpoint", backend.endpoint), zap.String("reason", reason), zap.Error(err))
		}
	}()
}

type metricBatcherPending struct {
	endpoint        string
	datapoints      int64
	bytes           int64
	compressedBytes int64
	oldestAgeMillis int64
}

type metricBatcherSnapshot struct {
	pending            []metricBatcherPending
	maxOldestAgeMillis int64
}

func (b *metricBatcher) snapshotPending() metricBatcherSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	now := time.Now()
	pending := make([]metricBatcherPending, 0, len(b.backends))
	var maxOldestAgeMillis int64
	for endpoint, backend := range b.backends {
		datapoints := backend.pendingDataPoints.Load()
		bytes := backend.pendingBytes.Load()
		compressedBytes := backend.pendingCompressedBytes.Load()
		var oldestAgeMillis int64
		if datapoints > 0 {
			oldestAgeMillis = ageMillisFromUnixNano(now, backend.oldestEnqueue.Load())
		}
		if oldestAgeMillis > maxOldestAgeMillis {
			maxOldestAgeMillis = oldestAgeMillis
		}
		pending = append(pending, metricBatcherPending{
			endpoint:        endpoint,
			datapoints:      datapoints,
			bytes:           bytes,
			compressedBytes: compressedBytes,
			oldestAgeMillis: oldestAgeMillis,
		})
	}
	return metricBatcherSnapshot{pending: pending, maxOldestAgeMillis: maxOldestAgeMillis}
}

func (b *metricBatcher) acquireBackend(endpoint string, exp *wrappedExporter) (*backendMetricBatcher, error) {
	if b.stopped.Load() {
		return nil, errMetricBatcherExporterStopping
	}

	b.mu.RLock()
	backend, ok := b.backends[endpoint]
	if ok {
		if exp != nil && exp.isStopping() {
			b.mu.RUnlock()
			return nil, errMetricBatcherExporterStopping
		}
		backend.setExporter(exp)
		backend.inflight.Add(1)
		b.mu.RUnlock()
		return backend, nil
	}
	b.mu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped.Load() {
		return nil, errMetricBatcherExporterStopping
	}
	backend, ok = b.backends[endpoint]
	if ok {
		if exp != nil && exp.isStopping() {
			return nil, errMetricBatcherExporterStopping
		}
		backend.setExporter(exp)
		backend.inflight.Add(1)
		return backend, nil
	}
	if exp != nil && exp.isStopping() {
		return nil, errMetricBatcherExporterStopping
	}

	backend = newBackendMetricBatcher(endpoint, exp, b.logger, b.settings, b.telemetry, b.send, b.drainErr)
	backend.inflight.Add(1)
	b.backends[endpoint] = backend
	return backend, nil
}

func newBackendMetricBatcher(
	endpoint string,
	exp *wrappedExporter,
	logger *zap.Logger,
	settings metricBatcherSettings,
	telemetry *metricBatcherTelemetry,
	send metricBatcherSendFunc,
	drainErr metricBatcherDrainFailureFunc,
) *backendMetricBatcher {
	backend := &backendMetricBatcher{
		endpoint:     endpoint,
		exp:          exp,
		logger:       logger.With(zap.String("endpoint", endpoint)),
		settings:     settings,
		telemetry:    telemetry,
		send:         send,
		drainErr:     drainErr,
		requests:     make(chan metricBatcherRequest, 16),
		done:         make(chan struct{}),
		payloadCodec: newMetricBatcherPayloadCodec(settings.payloadCompression),
	}

	go backend.run()
	return backend
}

func newMetricBatcherPayloadCodec(compression QueuePayloadCompression) *queuePayloadCodec {
	if compression == "" || compression == QueuePayloadCompressionNone {
		return nil
	}
	return newQueuePayloadCodec(compression)
}

func (b *backendMetricBatcher) stopAndFlush(ctx context.Context, reason string) error {
	done := make(chan error, 1)
	select {
	case b.requests <- metricBatcherRequest{kind: metricBatcherRequestFlushAndStop, ctx: ctx, reason: reason, done: done}:
	case <-b.done:
		return nil
	}

	select {
	case err := <-done:
		<-b.done
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *backendMetricBatcher) setExporter(exp *wrappedExporter) {
	b.exporterMu.Lock()
	b.exp = exp
	b.exporterMu.Unlock()
}

func (b *backendMetricBatcher) exporter() *wrappedExporter {
	b.exporterMu.RLock()
	defer b.exporterMu.RUnlock()
	return b.exp
}

func (b *backendMetricBatcher) run() {
	defer close(b.done)
	if b.payloadCodec != nil {
		defer func() {
			if err := b.payloadCodec.Close(); err != nil {
				b.logger.Warn("failed to close metric batcher payload codec", zap.Error(err))
			}
		}()
		b.runCompressed()
		return
	}

	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	var timerC <-chan time.Time
	retryingSince := time.Time{}

	pending := make([]pmetric.Metrics, 0, cap(b.requests))
	pendingBytes := 0
	pendingDataPoints := 0
	sizer := &pmetric.ProtoMarshaler{}
	var nextReq *metricBatcherRequest

	for {
		if nextReq != nil {
			req := *nextReq
			nextReq = nil
			if b.handleRequest(req, sizer, &pending, &pendingDataPoints, &pendingBytes, &nextReq, timer, &timerC, &retryingSince) {
				return
			}
			continue
		}

		select {
		case req := <-b.requests:
			if b.handleRequest(req, sizer, &pending, &pendingDataPoints, &pendingBytes, &nextReq, timer, &timerC, &retryingSince) {
				return
			}
		case <-timerC:
			if err := b.flush(context.Background(), sizer, &pending, &pendingDataPoints, &pendingBytes, metricFlushReasonTimeout, timer, &timerC, &retryingSince); err != nil {
				b.logger.Warn("failed to flush metric batch", zap.String("reason", metricFlushReasonTimeout), zap.Error(err))
			}
		}
	}
}

func (b *backendMetricBatcher) handleRequest(
	req metricBatcherRequest,
	sizer *pmetric.ProtoMarshaler,
	pending *[]pmetric.Metrics,
	pendingDataPoints *int,
	pendingBytes *int,
	nextReq **metricBatcherRequest,
	timer *time.Timer,
	timerC *<-chan time.Time,
	retryingSince *time.Time,
) bool {
	switch req.kind {
	case metricBatcherRequestEnqueue:
		dps, bytes, oldestEnqueue := b.drainEnqueueRequestsIntoPending(sizer, pending, req, nextReq)
		*pendingDataPoints += dps
		*pendingBytes += bytes
		if *pendingDataPoints > 0 && b.oldestEnqueue.Load() == 0 {
			b.oldestEnqueue.Store(oldestEnqueue)
		}
		b.pendingDataPoints.Store(int64(*pendingDataPoints))
		b.pendingBytes.Store(int64(*pendingBytes))
		if *pendingDataPoints > 0 && *timerC == nil {
			timer.Reset(b.settings.flushInterval)
			*timerC = timer.C
		}
		if *pendingDataPoints >= b.settings.maxDataPoints || *pendingBytes >= b.settings.maxBytes {
			attrs := metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint)))
			b.telemetry.overflowTotal.Add(context.Background(), 1, attrs)
			if err := b.flush(context.Background(), sizer, pending, pendingDataPoints, pendingBytes, metricFlushReasonSize, timer, timerC, retryingSince); err != nil {
				b.logger.Warn("failed to flush metric batch", zap.String("reason", metricFlushReasonSize), zap.Error(err))
			}
		}
	case metricBatcherRequestFlushAndStop:
		err := b.flush(req.ctx, sizer, pending, pendingDataPoints, pendingBytes, req.reason, timer, timerC, retryingSince)
		req.done <- err
		return true
	}
	return false
}

// drainEnqueueRequestsIntoPending drains immediately available enqueue requests
// into pending without merging payload trees.
// Returns (dataPointsAdded, bytesAdded, oldestEnqueueUnixNano).
func (b *backendMetricBatcher) drainEnqueueRequestsIntoPending(
	sizer *pmetric.ProtoMarshaler,
	pending *[]pmetric.Metrics,
	first metricBatcherRequest,
	nextReq **metricBatcherRequest,
) (int, int, int64) {
	dataPointCount := 0
	bytes := 0
	var oldestEnqueue int64

	appendRequest := func(req metricBatcherRequest) {
		count := req.md.DataPointCount()
		if count == 0 {
			return
		}
		dataPointCount += count
		bytes += sizer.MetricsSize(req.md)
		if oldestEnqueue == 0 || req.enqueuedAtUnixNano < oldestEnqueue {
			oldestEnqueue = req.enqueuedAtUnixNano
		}
		*pending = append(*pending, req.md)
	}

	appendRequest(first)

	for i := 0; i < cap(b.requests); i++ {
		select {
		case req := <-b.requests:
			if req.kind != metricBatcherRequestEnqueue {
				// Channels don't support unread/put-back. Stash this control request
				// so the run loop processes it next, in-order.
				*nextReq = &req
				return dataPointCount, bytes, oldestEnqueue
			}
			appendRequest(req)
		default:
			return dataPointCount, bytes, oldestEnqueue
		}
	}

	return dataPointCount, bytes, oldestEnqueue
}

type compressedMetricBatcherChunk struct {
	payload               []byte
	dataPoints            int
	uncompressedBytes     int
	compressedBytes       int
	oldestEnqueueUnixNano int64
}

// newCompressedMetricBatcherChunk requires callers to serialize concurrent use of codec.
// Production callers use backendMetricBatcher.payloadCodecMu; tests use private codecs.
func newCompressedMetricBatcherChunk(marshaler *pmetric.ProtoMarshaler, codec *queuePayloadCodec, req metricBatcherRequest) (compressedMetricBatcherChunk, error) {
	payload, err := marshaler.MarshalMetrics(req.md)
	if err != nil {
		return compressedMetricBatcherChunk{}, err
	}
	encoded, err := codec.Encode(payload)
	if err != nil {
		return compressedMetricBatcherChunk{}, err
	}
	encoded = append([]byte(nil), encoded...)
	encoded = encoded[:len(encoded):len(encoded)]
	return compressedMetricBatcherChunk{
		payload:               encoded,
		dataPoints:            req.md.DataPointCount(),
		uncompressedBytes:     len(payload),
		compressedBytes:       len(encoded),
		oldestEnqueueUnixNano: req.enqueuedAtUnixNano,
	}, nil
}

func (b *backendMetricBatcher) newCompressedMetricBatcherChunk(md pmetric.Metrics, enqueuedAtUnixNano int64) (compressedMetricBatcherChunk, error) {
	b.payloadCodecMu.Lock()
	defer b.payloadCodecMu.Unlock()
	return newCompressedMetricBatcherChunk(&pmetric.ProtoMarshaler{}, b.payloadCodec, metricBatcherRequest{
		md:                 md,
		enqueuedAtUnixNano: enqueuedAtUnixNano,
	})
}

// decodeCompressedMetricBatcherChunk requires callers to serialize concurrent use of codec.
// Production callers hold backendMetricBatcher.payloadCodecMu around merge/decode paths.
func decodeCompressedMetricBatcherChunk(unmarshaler *pmetric.ProtoUnmarshaler, codec *queuePayloadCodec, chunk compressedMetricBatcherChunk) (pmetric.Metrics, error) {
	payload, err := codec.Decode(chunk.payload)
	if err != nil {
		return pmetric.Metrics{}, err
	}
	return unmarshaler.UnmarshalMetrics(payload)
}

func (b *backendMetricBatcher) runCompressed() {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	var timerC <-chan time.Time

	pending := make([]compressedMetricBatcherChunk, 0, cap(b.requests))
	pendingBytes := 0
	pendingCompressedBytes := 0
	pendingDataPoints := 0
	unmarshaler := &pmetric.ProtoUnmarshaler{}
	var nextReq *metricBatcherRequest
	retryingSince := time.Time{}

	for {
		if nextReq != nil {
			req := *nextReq
			nextReq = nil
			if b.handleCompressedRequest(req, unmarshaler, &pending, &pendingDataPoints, &pendingBytes, &pendingCompressedBytes, &nextReq, timer, &timerC, &retryingSince) {
				return
			}
			continue
		}

		select {
		case req := <-b.requests:
			if b.handleCompressedRequest(req, unmarshaler, &pending, &pendingDataPoints, &pendingBytes, &pendingCompressedBytes, &nextReq, timer, &timerC, &retryingSince) {
				return
			}
		case <-timerC:
			if err := b.flushCompressed(context.Background(), unmarshaler, &pending, &pendingDataPoints, &pendingBytes, &pendingCompressedBytes, metricFlushReasonTimeout, timer, &timerC, &retryingSince); err != nil {
				b.logger.Warn("failed to flush compressed metric batch", zap.String("reason", metricFlushReasonTimeout), zap.Error(err))
			}
		}
	}
}

func (b *backendMetricBatcher) handleCompressedRequest(
	req metricBatcherRequest,
	unmarshaler *pmetric.ProtoUnmarshaler,
	pending *[]compressedMetricBatcherChunk,
	pendingDataPoints *int,
	pendingBytes *int,
	pendingCompressedBytes *int,
	nextReq **metricBatcherRequest,
	timer *time.Timer,
	timerC *<-chan time.Time,
	retryingSince *time.Time,
) bool {
	switch req.kind {
	case metricBatcherRequestEnqueue:
		dps, bytes, compressedBytes, oldestEnqueue := b.drainEnqueueRequestsIntoCompressedChunks(pending, req, nextReq)
		*pendingDataPoints += dps
		*pendingBytes += bytes
		*pendingCompressedBytes += compressedBytes
		if *pendingDataPoints > 0 && b.oldestEnqueue.Load() == 0 {
			b.oldestEnqueue.Store(oldestEnqueue)
		}
		b.pendingDataPoints.Store(int64(*pendingDataPoints))
		b.pendingBytes.Store(int64(*pendingBytes))
		b.pendingCompressedBytes.Store(int64(*pendingCompressedBytes))
		if *pendingDataPoints > 0 && *timerC == nil {
			timer.Reset(b.settings.flushInterval)
			*timerC = timer.C
		}
		if *pendingDataPoints >= b.settings.maxDataPoints || *pendingBytes >= b.settings.maxBytes {
			attrs := metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint)))
			b.telemetry.overflowTotal.Add(context.Background(), 1, attrs)
			if err := b.flushCompressed(context.Background(), unmarshaler, pending, pendingDataPoints, pendingBytes, pendingCompressedBytes, metricFlushReasonSize, timer, timerC, retryingSince); err != nil {
				b.logger.Warn("failed to flush compressed metric batch", zap.String("reason", metricFlushReasonSize), zap.Error(err))
			}
		}
	case metricBatcherRequestFlushAndStop:
		err := b.flushCompressed(req.ctx, unmarshaler, pending, pendingDataPoints, pendingBytes, pendingCompressedBytes, req.reason, timer, timerC, retryingSince)
		req.done <- err
		return true
	}
	return false
}

func (b *backendMetricBatcher) drainEnqueueRequestsIntoCompressedChunks(
	pending *[]compressedMetricBatcherChunk,
	first metricBatcherRequest,
	nextReq **metricBatcherRequest,
) (int, int, int, int64) {
	dataPointCount := 0
	bytes := 0
	compressedBytes := 0
	var oldestEnqueue int64

	appendRequest := func(req metricBatcherRequest) {
		count := req.compressedChunk.dataPoints
		if count == 0 {
			return
		}
		dataPointCount += count
		bytes += req.compressedChunk.uncompressedBytes
		compressedBytes += req.compressedChunk.compressedBytes
		if oldestEnqueue == 0 || req.enqueuedAtUnixNano < oldestEnqueue {
			oldestEnqueue = req.enqueuedAtUnixNano
		}
		*pending = append(*pending, req.compressedChunk)
	}

	appendRequest(first)

	for i := 0; i < cap(b.requests); i++ {
		select {
		case req := <-b.requests:
			if req.kind != metricBatcherRequestEnqueue {
				*nextReq = &req
				return dataPointCount, bytes, compressedBytes, oldestEnqueue
			}
			appendRequest(req)
		default:
			return dataPointCount, bytes, compressedBytes, oldestEnqueue
		}
	}

	return dataPointCount, bytes, compressedBytes, oldestEnqueue
}

func (b *backendMetricBatcher) flushCompressed(
	ctx context.Context,
	unmarshaler *pmetric.ProtoUnmarshaler,
	pending *[]compressedMetricBatcherChunk,
	pendingDataPoints *int,
	pendingBytes *int,
	pendingCompressedBytes *int,
	reason string,
	timer *time.Timer,
	timerC *<-chan time.Time,
	retryingSince *time.Time,
) error {
	if !timer.Stop() && *timerC != nil {
		select {
		case <-timer.C:
		default:
		}
	}
	*timerC = nil

	if *pendingDataPoints == 0 {
		return nil
	}

	drainedChunks := *pending
	datapoints := *pendingDataPoints
	bytes := *pendingBytes
	oldestUnixNano := b.oldestEnqueue.Load()
	oldestAgeMillis := ageMillisFromUnixNano(time.Now(), oldestUnixNano)
	*pending = (*pending)[:0]
	*pendingDataPoints = 0
	*pendingBytes = 0
	*pendingCompressedBytes = 0
	b.pendingDataPoints.Store(0)
	b.pendingBytes.Store(0)
	b.pendingCompressedBytes.Store(0)
	b.oldestEnqueue.Store(0)

	b.payloadCodecMu.Lock()
	drained, err := mergeCompressedMetricBatcherChunks(unmarshaler, b.payloadCodec, drainedChunks)
	b.payloadCodecMu.Unlock()
	attrs := metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint)))
	flushAttrs := metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint), attribute.String("reason", reason)))
	b.telemetry.batchDataPoints.Record(ctx, int64(datapoints), attrs)
	b.telemetry.batchBytes.Record(ctx, int64(bytes), attrs)
	b.telemetry.flushTotal.Add(ctx, 1, flushAttrs)
	if err != nil {
		b.telemetry.flushErrors.Add(ctx, 1, attrs)
		b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
		b.telemetry.droppedDataPoints.Add(ctx, int64(datapoints), attrs)
		clearCompressedMetricBatcherChunks(drainedChunks)
		return err
	}

	var rerouteMetrics pmetric.Metrics
	if b.drainErr != nil && reason != metricFlushReasonShutdown {
		rerouteMetrics = pmetric.NewMetrics()
		drained.CopyTo(rerouteMetrics)
	}
	exp := b.exporter()
	if exp != nil && exp.isStopping() && b.drainErr != nil && reason != metricFlushReasonShutdown {
		err = metricBatcherRerouteableError{err: errMetricBatcherExporterStopping, data: rerouteMetrics}
	} else {
		err = b.send(ctx, exp, drained, reason)
	}
	if err != nil {
		b.telemetry.flushErrors.Add(ctx, 1, attrs)
		retryChunks := drainedChunks
		retryUsesDrainedChunks := true
		retryDataPoints := datapoints
		retryBytes := bytes
		retryCompressedBytes := compressedMetricBatcherChunksCompressedBytes(drainedChunks)
		rerouteAttempted := false
		if b.drainErr != nil && reason != metricFlushReasonShutdown {
			var rerouteable metricBatcherRerouteableError
			if errors.As(err, &rerouteable) {
				rerouteAttempted = true
				rerouteErr := b.drainErr(ctx, rerouteable.Data(), reason)
				rerouteable.RecordReroute(ctx, rerouteErr)
				if rerouteErr == nil {
					b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
					*retryingSince = time.Time{}
					b.flushFailures = 0
					clearCompressedMetricBatcherChunks(drainedChunks)
					return nil
				}
				retryMetrics := rerouteMetrics
				var rerouteMetricsErr consumererror.Metrics
				if errors.As(rerouteErr, &rerouteMetricsErr) {
					retryMetrics = rerouteMetricsErr.Data()
				}
				var retryErr error
				retryChunks, retryDataPoints, retryBytes, retryCompressedBytes, retryErr = b.compressedMetricBatcherRetryChunks(retryMetrics, oldestUnixNano)
				if retryErr != nil {
					b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
					b.telemetry.droppedDataPoints.Add(ctx, int64(retryDataPoints), attrs)
					clearCompressedMetricBatcherChunks(drainedChunks)
					return errors.Join(err, rerouteErr, retryErr)
				}
				retryUsesDrainedChunks = false
				datapoints = retryDataPoints
				err = errors.Join(err, rerouteErr)
			}
		}
		if reason == metricFlushReasonSize || reason == metricFlushReasonTimeout {
			now := time.Now()
			if retryingSince.IsZero() {
				*retryingSince = now
			}

			b.flushFailures++
			overCap := isBeyondRetryCap(retryDataPoints, b.settings.maxDataPoints, b.settings.maxRetryBufferMultiplier) || isBeyondRetryCap(retryBytes, b.settings.maxBytes, b.settings.maxRetryBufferMultiplier)
			overAge := now.Sub(*retryingSince) >= metricBatcherMaxRetryAge
			if overCap || overAge {
				b.logger.Error("dropping compressed metric batch after retry limits exceeded", zap.String("endpoint", b.endpoint), zap.String("reason", reason), zap.Int("failures", b.flushFailures), zap.Int("datapoints", retryDataPoints), zap.Int("bytes", retryBytes), zap.Int("compressed_bytes", retryCompressedBytes), zap.Bool("over_cap", overCap), zap.Bool("over_age", overAge), zap.Duration("retry_age", now.Sub(*retryingSince)), zap.Error(err))
				b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
				b.telemetry.droppedDataPoints.Add(ctx, int64(retryDataPoints), attrs)
				*retryingSince = time.Time{}
				b.flushFailures = 0
				clearCompressedMetricBatcherChunks(drainedChunks)
				if !retryUsesDrainedChunks {
					clearCompressedMetricBatcherChunks(retryChunks)
				}
				return err
			}
			if !retryUsesDrainedChunks {
				clearCompressedMetricBatcherChunks(drainedChunks)
				*pending = append((*pending)[:0], retryChunks...)
			} else {
				*pending = retryChunks
			}
			*pendingDataPoints = retryDataPoints
			*pendingBytes = retryBytes
			*pendingCompressedBytes = retryCompressedBytes
			b.pendingDataPoints.Store(int64(retryDataPoints))
			b.pendingBytes.Store(int64(retryBytes))
			b.pendingCompressedBytes.Store(int64(retryCompressedBytes))
			b.oldestEnqueue.Store(oldestUnixNano)
			if *timerC == nil {
				delay := b.settings.flushInterval * time.Duration(1<<min(b.flushFailures-1, 4))
				delay = min(delay, maxMetricBatchRetryBackoff)
				timer.Reset(delay)
				*timerC = timer.C
			}
			return err
		}
		if !rerouteAttempted && b.drainErr != nil && (reason == metricFlushReasonResolverChange || reason == metricFlushReasonShutdown) {
			rerouteErr := b.drainErr(ctx, drained, reason)
			if rerouteErr == nil {
				b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
				*retryingSince = time.Time{}
				b.flushFailures = 0
				clearCompressedMetricBatcherChunks(drainedChunks)
				return nil
			}
			var rerouteMetricsErr consumererror.Metrics
			if errors.As(rerouteErr, &rerouteMetricsErr) {
				datapoints = rerouteMetricsErr.Data().DataPointCount()
			}
			err = errors.Join(err, rerouteErr)
		}
		b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
		b.telemetry.droppedDataPoints.Add(ctx, int64(datapoints), attrs)
		*retryingSince = time.Time{}
		b.flushFailures = 0
		clearCompressedMetricBatcherChunks(drainedChunks)
	} else {
		b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
		b.flushFailures = 0
		*retryingSince = time.Time{}
		clearCompressedMetricBatcherChunks(drainedChunks)
	}
	return err
}

func (b *backendMetricBatcher) compressedMetricBatcherRetryChunks(md pmetric.Metrics, oldestUnixNano int64) ([]compressedMetricBatcherChunk, int, int, int, error) {
	dataPoints := md.DataPointCount()
	if dataPoints == 0 {
		return nil, 0, 0, 0, nil
	}
	bytes := (&pmetric.ProtoMarshaler{}).MetricsSize(md)
	chunk, err := b.newCompressedMetricBatcherChunk(md, oldestUnixNano)
	if err != nil {
		return nil, dataPoints, bytes, 0, err
	}
	return []compressedMetricBatcherChunk{chunk}, chunk.dataPoints, chunk.uncompressedBytes, chunk.compressedBytes, nil
}

func mergeCompressedMetricBatcherChunks(unmarshaler *pmetric.ProtoUnmarshaler, codec *queuePayloadCodec, chunks []compressedMetricBatcherChunk) (pmetric.Metrics, error) {
	if len(chunks) == 0 {
		return pmetric.NewMetrics(), nil
	}

	merged, err := decodeCompressedMetricBatcherChunk(unmarshaler, codec, chunks[0])
	if err != nil {
		return pmetric.Metrics{}, err
	}
	for i := 1; i < len(chunks); i++ {
		md, err := decodeCompressedMetricBatcherChunk(unmarshaler, codec, chunks[i])
		if err != nil {
			return pmetric.Metrics{}, err
		}
		mergeMetricChunksByMove(merged, md)
	}
	return merged, nil
}

func clearCompressedMetricBatcherChunks(chunks []compressedMetricBatcherChunk) {
	for i := range chunks {
		chunks[i] = compressedMetricBatcherChunk{}
	}
}

func compressedMetricBatcherChunksCompressedBytes(chunks []compressedMetricBatcherChunk) int {
	compressedBytes := 0
	for _, chunk := range chunks {
		compressedBytes += chunk.compressedBytes
	}
	return compressedBytes
}

func (b *backendMetricBatcher) flush(
	ctx context.Context,
	sizer *pmetric.ProtoMarshaler,
	pending *[]pmetric.Metrics,
	pendingDataPoints *int,
	pendingBytes *int,
	reason string,
	timer *time.Timer,
	timerC *<-chan time.Time,
	retryingSince *time.Time,
) error {
	if !timer.Stop() && *timerC != nil {
		select {
		case <-timer.C:
		default:
		}
	}
	*timerC = nil

	if *pendingDataPoints == 0 {
		return nil
	}

	drainedChunks := *pending
	drained := mergePendingMetricChunks(drainedChunks)
	datapoints := *pendingDataPoints
	bytes := sizer.MetricsSize(drained)
	oldestUnixNano := b.oldestEnqueue.Load()
	oldestAgeMillis := ageMillisFromUnixNano(time.Now(), oldestUnixNano)
	*pending = (*pending)[:0]
	*pendingDataPoints = 0
	*pendingBytes = 0
	b.pendingDataPoints.Store(0)
	b.pendingBytes.Store(0)
	b.oldestEnqueue.Store(0)

	attrs := metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint)))
	flushAttrs := metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint), attribute.String("reason", reason)))
	b.telemetry.batchDataPoints.Record(ctx, int64(datapoints), attrs)
	b.telemetry.batchBytes.Record(ctx, int64(bytes), attrs)
	b.telemetry.flushTotal.Add(ctx, 1, flushAttrs)

	var rerouteMetrics pmetric.Metrics
	if b.drainErr != nil && reason != metricFlushReasonShutdown {
		rerouteMetrics = pmetric.NewMetrics()
		drained.CopyTo(rerouteMetrics)
	}
	exp := b.exporter()
	var err error
	if exp != nil && exp.isStopping() && b.drainErr != nil && reason != metricFlushReasonShutdown {
		err = metricBatcherRerouteableError{err: errMetricBatcherExporterStopping, data: rerouteMetrics}
	} else {
		err = b.send(ctx, exp, drained, reason)
	}
	if err != nil {
		b.telemetry.flushErrors.Add(ctx, 1, attrs)
		rerouteAttempted := false
		var rerouteable metricBatcherRerouteableError
		if b.drainErr != nil && reason != metricFlushReasonShutdown && errors.As(err, &rerouteable) {
			rerouteAttempted = true
			rerouteErr := b.drainErr(ctx, rerouteable.Data(), reason)
			rerouteable.RecordReroute(ctx, rerouteErr)
			if rerouteErr == nil {
				b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
				*retryingSince = time.Time{}
				b.flushFailures = 0
				return nil
			}
			var rerouteMetricsErr consumererror.Metrics
			if errors.As(rerouteErr, &rerouteMetricsErr) {
				drained = rerouteMetricsErr.Data()
			} else {
				drained = rerouteMetrics
			}
			datapoints = drained.DataPointCount()
			bytes = sizer.MetricsSize(drained)
			err = errors.Join(err, rerouteErr)
		}
		if reason == metricFlushReasonSize || reason == metricFlushReasonTimeout {
			now := time.Now()
			if retryingSince.IsZero() {
				*retryingSince = now
			}

			b.flushFailures++
			overCap := isBeyondRetryCap(datapoints, b.settings.maxDataPoints, b.settings.maxRetryBufferMultiplier) || isBeyondRetryCap(bytes, b.settings.maxBytes, b.settings.maxRetryBufferMultiplier)
			overAge := now.Sub(*retryingSince) >= metricBatcherMaxRetryAge
			if overCap || overAge {
				b.logger.Error("dropping metric batch after retry limits exceeded", zap.String("endpoint", b.endpoint), zap.String("reason", reason), zap.Int("failures", b.flushFailures), zap.Int("datapoints", datapoints), zap.Int("bytes", bytes), zap.Bool("over_cap", overCap), zap.Bool("over_age", overAge), zap.Duration("retry_age", now.Sub(*retryingSince)), zap.Error(err))
				b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
				b.telemetry.droppedDataPoints.Add(ctx, int64(datapoints), attrs)
				*retryingSince = time.Time{}
				b.flushFailures = 0
				return err
			}
			*pending = append((*pending)[:0], drained)
			*pendingDataPoints = datapoints
			*pendingBytes = bytes
			b.pendingDataPoints.Store(int64(datapoints))
			b.pendingBytes.Store(int64(bytes))
			b.oldestEnqueue.Store(oldestUnixNano)
			if *timerC == nil {
				delay := b.settings.flushInterval * time.Duration(1<<min(b.flushFailures-1, 4))
				delay = min(delay, maxMetricBatchRetryBackoff)
				timer.Reset(delay)
				*timerC = timer.C
			}
		} else {
			if !rerouteAttempted && b.drainErr != nil && (reason == metricFlushReasonResolverChange || reason == metricFlushReasonShutdown) {
				rerouteErr := b.drainErr(ctx, drained, reason)
				if rerouteErr == nil {
					b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
					return nil
				}
				var mErr consumererror.Metrics
				if errors.As(rerouteErr, &mErr) {
					drained = mErr.Data()
					datapoints = drained.DataPointCount()
				}
				err = errors.Join(err, rerouteErr)
			}
			b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
			b.telemetry.droppedDataPoints.Add(ctx, int64(datapoints), attrs)
		}
	}

	if err == nil {
		b.telemetry.flushOldestPointAge.Record(ctx, oldestAgeMillis, flushAttrs)
		*retryingSince = time.Time{}
		b.flushFailures = 0
	}
	return err
}

func mergePendingMetricChunks(chunks []pmetric.Metrics) pmetric.Metrics {
	if len(chunks) == 0 {
		return pmetric.NewMetrics()
	}
	if len(chunks) == 1 {
		return chunks[0]
	}
	merged := chunks[0]
	for i := 1; i < len(chunks); i++ {
		mergeMetricChunksByMove(merged, chunks[i])
	}
	return merged
}

// mergeMetricChunksByMove assumes src is owned by the batcher and can be consumed.
// It uses move semantics where possible to avoid deep-copy merge overhead.
func mergeMetricChunksByMove(dst, src pmetric.Metrics) {
	dstResourceMetrics := dst.ResourceMetrics()
	srcResourceMetrics := src.ResourceMetrics()

	resources := make(map[identity.Resource]pmetric.ResourceMetrics, dstResourceMetrics.Len())
	for i := 0; i < dstResourceMetrics.Len(); i++ {
		rm := dstResourceMetrics.At(i)
		resources[identity.OfResource(rm.Resource())] = rm
	}

	for i := 0; i < srcResourceMetrics.Len(); i++ {
		rm := srcResourceMetrics.At(i)
		resourceID := identity.OfResource(rm.Resource())

		dstRM, ok := resources[resourceID]
		if !ok {
			dstRM = dstResourceMetrics.AppendEmpty()
			rm.MoveTo(dstRM)
			resources[resourceID] = dstRM
			continue
		}

		mergeResourceMetricsByMove(resourceID, dstRM, rm)
	}
}

func mergeResourceMetricsByMove(resourceID identity.Resource, dst, src pmetric.ResourceMetrics) {
	dstScopeMetrics := dst.ScopeMetrics()
	srcScopeMetrics := src.ScopeMetrics()

	scopes := make(map[identity.Scope]pmetric.ScopeMetrics, dstScopeMetrics.Len())
	for i := 0; i < dstScopeMetrics.Len(); i++ {
		sm := dstScopeMetrics.At(i)
		scopes[identity.OfScope(resourceID, sm.Scope())] = sm
	}

	for i := 0; i < srcScopeMetrics.Len(); i++ {
		sm := srcScopeMetrics.At(i)
		scopeID := identity.OfScope(resourceID, sm.Scope())

		dstSM, ok := scopes[scopeID]
		if !ok {
			dstSM = dstScopeMetrics.AppendEmpty()
			sm.MoveTo(dstSM)
			scopes[scopeID] = dstSM
			continue
		}

		mergeScopeMetricsByMove(scopeID, dstSM, sm)
	}
}

func mergeScopeMetricsByMove(scopeID identity.Scope, dst, src pmetric.ScopeMetrics) {
	dstMetrics := dst.Metrics()
	srcMetrics := src.Metrics()

	metricsByIdentity := make(map[identity.Metric]pmetric.Metric, dstMetrics.Len())
	for i := 0; i < dstMetrics.Len(); i++ {
		m := dstMetrics.At(i)
		metricsByIdentity[identity.OfMetric(scopeID, m)] = m
	}

	for i := 0; i < srcMetrics.Len(); i++ {
		srcMetric := srcMetrics.At(i)
		metricID := identity.OfMetric(scopeID, srcMetric)

		dstMetric, ok := metricsByIdentity[metricID]
		if !ok {
			dstMetric = dstMetrics.AppendEmpty()
			srcMetric.MoveTo(dstMetric)
			metricsByIdentity[metricID] = dstMetric
			continue
		}

		mergeMetricDataPointsByMove(dstMetric, srcMetric)
	}
}

func mergeMetricDataPointsByMove(dst, src pmetric.Metric) {
	switch dst.Type() {
	case pmetric.MetricTypeGauge:
		src.Gauge().DataPoints().MoveAndAppendTo(dst.Gauge().DataPoints())
	case pmetric.MetricTypeSum:
		src.Sum().DataPoints().MoveAndAppendTo(dst.Sum().DataPoints())
	case pmetric.MetricTypeHistogram:
		src.Histogram().DataPoints().MoveAndAppendTo(dst.Histogram().DataPoints())
	case pmetric.MetricTypeExponentialHistogram:
		src.ExponentialHistogram().DataPoints().MoveAndAppendTo(dst.ExponentialHistogram().DataPoints())
	case pmetric.MetricTypeSummary:
		src.Summary().DataPoints().MoveAndAppendTo(dst.Summary().DataPoints())
	default:
		newMetric := dst
		src.MoveTo(newMetric)
	}
}

func isBeyondRetryCap(current, base, multiplier int) bool {
	if current <= 0 || base <= 0 {
		return false
	}
	if multiplier <= 0 {
		return false
	}
	if base > math.MaxInt/multiplier {
		return false
	}
	return current > base*multiplier
}

func newMetricBatcherTelemetry(settings component.TelemetrySettings) (*metricBatcherTelemetry, error) {
	meter := metadata.Meter(settings)
	var err, errs error

	t := &metricBatcherTelemetry{logger: settings.Logger, meter: meter}
	t.batchDataPoints, err = meter.Int64Histogram(
		"otelcol_loadbalancer_metric_batch_size",
		metric.WithDescription("Number of metric datapoints per flushed backend batch."),
		metric.WithUnit("{datapoints}"),
	)
	errs = errors.Join(errs, err)
	t.batchBytes, err = meter.Int64Histogram(
		"otelcol_loadbalancer_metric_batch_bytes",
		metric.WithDescription("Serialized OTLP bytes per flushed metric backend batch before compression."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.flushTotal, err = meter.Int64Counter(
		"otelcol_loadbalancer_metric_batch_flush_total",
		metric.WithDescription("Number of metric batch flushes by endpoint and reason."),
		metric.WithUnit("{flushes}"),
	)
	errs = errors.Join(errs, err)
	t.flushErrors, err = meter.Int64Counter(
		"otelcol_loadbalancer_metric_batch_flush_errors",
		metric.WithDescription("Number of metric batch flush errors."),
		metric.WithUnit("{errors}"),
	)
	errs = errors.Join(errs, err)
	t.flushOldestPointAge, err = meter.Int64Histogram(
		"otelcol_loadbalancer_metric_batch_flush_oldest_datapoint_age",
		metric.WithDescription("Age in ms of the oldest metric datapoint in a flushed backend batch."),
		metric.WithUnit("ms"),
	)
	errs = errors.Join(errs, err)
	t.droppedDataPoints, err = meter.Int64Counter(
		"otelcol_loadbalancer_metric_batch_dropped_datapoints",
		metric.WithDescription("Number of dropped metric datapoints in the internal metric batcher."),
		metric.WithUnit("{datapoints}"),
	)
	errs = errors.Join(errs, err)
	t.overflowTotal, err = meter.Int64Counter(
		"otelcol_loadbalancer_metric_batch_overflow_total",
		metric.WithDescription("Number of times an internal metric batch hit a size bound and was force-flushed."),
		metric.WithUnit("{overflows}"),
	)
	errs = errors.Join(errs, err)
	t.pendingDataPoints, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_metric_batch_pending_datapoints",
		metric.WithDescription("Current number of pending metric datapoints per backend batch."),
		metric.WithUnit("{datapoints}"),
	)
	errs = errors.Join(errs, err)
	t.pendingBytes, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_metric_batch_pending_bytes",
		metric.WithDescription("Current serialized OTLP bytes per pending metric backend batch before compression."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.pendingCompressedBytes, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_metric_batch_pending_compressed_bytes",
		metric.WithDescription("Current compressed OTLP bytes per pending metric backend batch."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.pendingOldestPointAge, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_metric_batch_pending_oldest_datapoint_age",
		metric.WithDescription("Age in ms of the oldest pending metric datapoint per backend batch."),
		metric.WithUnit("ms"),
	)
	errs = errors.Join(errs, err)
	t.pendingOldestPointMax, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_metric_batch_pending_oldest_datapoint_age_max",
		metric.WithDescription("Maximum age in ms of the oldest pending metric datapoint across backend batches."),
		metric.WithUnit("ms"),
	)
	errs = errors.Join(errs, err)

	return t, errs
}

func (t *metricBatcherTelemetry) start(snapshot func() metricBatcherSnapshot) error {
	reg, err := t.meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		state := snapshot()
		for _, pending := range state.pending {
			attrs := metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", pending.endpoint)))
			observer.ObserveInt64(t.pendingDataPoints, pending.datapoints, attrs)
			observer.ObserveInt64(t.pendingBytes, pending.bytes, attrs)
			observer.ObserveInt64(t.pendingCompressedBytes, pending.compressedBytes, attrs)
			observer.ObserveInt64(t.pendingOldestPointAge, pending.oldestAgeMillis, attrs)
		}
		observer.ObserveInt64(t.pendingOldestPointMax, state.maxOldestAgeMillis)
		return nil
	}, t.pendingDataPoints, t.pendingBytes, t.pendingCompressedBytes, t.pendingOldestPointAge, t.pendingOldestPointMax)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.registrations = append(t.registrations, reg)
	t.mu.Unlock()
	return nil
}

func (t *metricBatcherTelemetry) shutdown() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, reg := range t.registrations {
		if err := reg.Unregister(); err != nil {
			t.logger.Warn("failed to unregister metric batcher metric callback", zap.Error(err))
		}
	}
	t.registrations = nil
}
