// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"math/bits"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

const (
	logFlushReasonDirect         = "direct"
	logFlushReasonSize           = "size"
	logFlushReasonTimeout        = "timeout"
	logFlushReasonShutdown       = "shutdown"
	logFlushReasonResolverChange = "resolver_change"

	defaultLogBatchMaxRecords   = 512
	defaultLogBatchMaxBytes     = 1 << 20
	defaultLogBatchFlushTimeout = 100 * time.Millisecond
)

var errLogBatcherExporterStopping = errors.New("log batcher exporter is stopping")

type logBatcherSettings struct {
	maxRecords         int
	maxBytes           int
	flushInterval      time.Duration
	payloadCompression QueuePayloadCompression
	zstd               ZstdPayloadCodecConfig
}

type (
	logBatcherSendFunc         func(context.Context, *wrappedExporter, plog.Logs, string) error
	logBatcherDrainFailureFunc func(context.Context, plog.Logs, string) error
)

type logBatcher struct {
	logger   *zap.Logger
	settings logBatcherSettings
	send     logBatcherSendFunc
	drainErr logBatcherDrainFailureFunc

	telemetry *logBatcherTelemetry

	mu       sync.RWMutex
	backends map[string]*backendLogBatcher
	stopped  atomic.Bool
}

type logBatcherRequest struct {
	kind               logBatcherRequestKind
	logs               plog.Logs
	compressedChunk    compressedLogBatcherChunk
	ctx                context.Context
	reason             string
	done               chan error
	enqueuedAtUnixNano int64
}

type logBatcherRequestKind int

const (
	logBatcherRequestEnqueue logBatcherRequestKind = iota
	logBatcherRequestFlushAndStop
)

type backendLogBatcher struct {
	endpoint string
	logger   *zap.Logger
	settings logBatcherSettings
	send     logBatcherSendFunc
	drainErr logBatcherDrainFailureFunc

	telemetry *logBatcherTelemetry

	exporterMu sync.RWMutex
	exp        *wrappedExporter

	requests chan logBatcherRequest
	done     chan struct{}
	inflight sync.WaitGroup

	pendingRecords         atomic.Int64
	pendingBytes           atomic.Int64
	pendingCompressedBytes atomic.Int64
	oldestEnqueue          atomic.Int64

	payloadCodecMu sync.Mutex
	payloadCodec   *queuePayloadCodec
}

type logBatcherTelemetry struct {
	logger                 *zap.Logger
	meter                  metric.Meter
	batchSize              metric.Int64Histogram
	batchBytes             metric.Int64Histogram
	flushTotal             metric.Int64Counter
	flushErrors            metric.Int64Counter
	flushOldestRecordAge   metric.Int64Histogram
	droppedRecords         metric.Int64Counter
	overflowTotal          metric.Int64Counter
	pendingRecords         metric.Int64ObservableGauge
	pendingBytes           metric.Int64ObservableGauge
	pendingCompressedBytes metric.Int64ObservableGauge
	pendingOldestAge       metric.Int64ObservableGauge
	pendingOldestAgeMax    metric.Int64ObservableGauge

	mu            sync.Mutex
	registrations []metric.Registration
}

func newLogBatcher(
	logger *zap.Logger,
	settings component.TelemetrySettings,
	cfg logBatcherSettings,
	send logBatcherSendFunc,
	drainErr ...logBatcherDrainFailureFunc,
) (*logBatcher, error) {
	telemetry, err := newLogBatcherTelemetry(settings)
	if err != nil {
		return nil, err
	}
	var drainFailure logBatcherDrainFailureFunc
	if len(drainErr) > 0 {
		drainFailure = drainErr[0]
	}

	lb := &logBatcher{
		logger:    logger,
		settings:  cfg,
		send:      send,
		drainErr:  drainFailure,
		telemetry: telemetry,
		backends:  make(map[string]*backendLogBatcher),
	}

	if err := telemetry.start(lb.snapshotPending); err != nil {
		return nil, err
	}

	return lb, nil
}

func (b *logBatcher) Enqueue(ctx context.Context, endpoint string, exp *wrappedExporter, logs plog.Logs) error {
	backend, err := b.acquireBackend(endpoint, exp)
	if err != nil {
		return err
	}
	defer backend.inflight.Done()
	enqueuedAtUnixNano := time.Now().UnixNano()
	req := logBatcherRequest{kind: logBatcherRequestEnqueue, logs: logs, enqueuedAtUnixNano: enqueuedAtUnixNano}
	if backend.payloadCodec != nil {
		compressedChunk, err := backend.newCompressedLogBatcherChunk(logs, enqueuedAtUnixNano)
		if err != nil {
			return err
		}
		req.logs = plog.NewLogs()
		req.compressedChunk = compressedChunk
	}
	select {
	case backend.requests <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-backend.done:
		return errors.New("log batcher backend is stopped")
	}
}

func (b *logBatcher) Remove(ctx context.Context, endpoint string, exp *wrappedExporter) error {
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
		b.scheduleBackendCleanup(backend, logFlushReasonResolverChange)
		return err
	}
	return backend.stopAndFlush(ctx, logFlushReasonResolverChange)
}

func (b *logBatcher) Shutdown(ctx context.Context) error {
	b.stopped.Store(true)

	b.mu.Lock()
	backends := make([]*backendLogBatcher, 0, len(b.backends))
	for endpoint, backend := range b.backends {
		backends = append(backends, backend)
		delete(b.backends, endpoint)
	}
	b.mu.Unlock()

	var errs error
	for _, backend := range backends {
		if err := waitForInflight(ctx, &backend.inflight); err != nil {
			b.scheduleBackendCleanup(backend, logFlushReasonShutdown)
			errs = errors.Join(errs, err)
			continue
		}
		errs = errors.Join(errs, backend.stopAndFlush(ctx, logFlushReasonShutdown))
	}
	b.telemetry.shutdown()
	return errs
}

func (b *logBatcher) scheduleBackendCleanup(backend *backendLogBatcher, reason string) {
	go func() {
		if err := waitForInflight(context.Background(), &backend.inflight); err != nil {
			b.logger.Warn("failed waiting for inflight log batcher requests during background cleanup", zap.String("endpoint", backend.endpoint), zap.Error(err))
			return
		}
		if err := backend.stopAndFlush(context.Background(), reason); err != nil {
			b.logger.Warn("failed to stop log batcher backend during background cleanup", zap.String("endpoint", backend.endpoint), zap.String("reason", reason), zap.Error(err))
		}
	}()
}

type logBatcherSnapshot struct {
	pending            []logBatcherPending
	maxOldestAgeMillis int64
}

func (b *logBatcher) snapshotPending() logBatcherSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	now := time.Now()
	pending := make([]logBatcherPending, 0, len(b.backends))
	var maxOldestAgeMillis int64
	for endpoint, backend := range b.backends {
		records := backend.pendingRecords.Load()
		bytes := backend.pendingBytes.Load()
		compressedBytes := backend.pendingCompressedBytes.Load()
		var oldestAgeMillis int64
		if records > 0 {
			oldestAgeMillis = ageMillisFromUnixNano(now, backend.oldestEnqueue.Load())
		}
		if oldestAgeMillis > maxOldestAgeMillis {
			maxOldestAgeMillis = oldestAgeMillis
		}
		pending = append(pending, logBatcherPending{
			endpoint:        endpoint,
			records:         records,
			bytes:           bytes,
			compressedBytes: compressedBytes,
			oldestAgeMillis: oldestAgeMillis,
		})
	}
	return logBatcherSnapshot{pending: pending, maxOldestAgeMillis: maxOldestAgeMillis}
}

func (b *logBatcher) acquireBackend(endpoint string, exp *wrappedExporter) (*backendLogBatcher, error) {
	if b.stopped.Load() {
		return nil, errLogBatcherExporterStopping
	}

	b.mu.RLock()
	backend, ok := b.backends[endpoint]
	if ok {
		if exp != nil && exp.isStopping() {
			b.mu.RUnlock()
			return nil, errLogBatcherExporterStopping
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
		return nil, errLogBatcherExporterStopping
	}
	backend, ok = b.backends[endpoint]
	if ok {
		if exp != nil && exp.isStopping() {
			return nil, errLogBatcherExporterStopping
		}
		backend.setExporter(exp)
		backend.inflight.Add(1)
		return backend, nil
	}
	if exp != nil && exp.isStopping() {
		return nil, errLogBatcherExporterStopping
	}

	backend = newBackendLogBatcher(endpoint, exp, b.logger, b.settings, b.telemetry, b.send, b.drainErr)
	backend.inflight.Add(1)
	b.backends[endpoint] = backend
	return backend, nil
}

func newBackendLogBatcher(
	endpoint string,
	exp *wrappedExporter,
	logger *zap.Logger,
	settings logBatcherSettings,
	telemetry *logBatcherTelemetry,
	send logBatcherSendFunc,
	drainErr logBatcherDrainFailureFunc,
) *backendLogBatcher {
	backend := &backendLogBatcher{
		endpoint:     endpoint,
		exp:          exp,
		logger:       logger.With(zap.String("endpoint", endpoint)),
		settings:     settings,
		send:         send,
		drainErr:     drainErr,
		telemetry:    telemetry,
		requests:     make(chan logBatcherRequest, 16),
		done:         make(chan struct{}),
		payloadCodec: newLogBatcherPayloadCodec(settings.payloadCompression, settings.zstd),
	}

	go backend.run()
	return backend
}

func newLogBatcherPayloadCodec(compression QueuePayloadCompression, zstdConfig ...ZstdPayloadCodecConfig) *queuePayloadCodec {
	if compression == "" || compression == QueuePayloadCompressionNone {
		return nil
	}
	return newQueuePayloadCodec(compression, zstdConfig...)
}

func (b *backendLogBatcher) stopAndFlush(ctx context.Context, reason string) error {
	done := make(chan error, 1)
	select {
	case b.requests <- logBatcherRequest{
		kind:   logBatcherRequestFlushAndStop,
		ctx:    ctx,
		reason: reason,
		done:   done,
	}:
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

func (b *backendLogBatcher) setExporter(exp *wrappedExporter) {
	b.exporterMu.Lock()
	b.exp = exp
	b.exporterMu.Unlock()
}

func (b *backendLogBatcher) exporter() *wrappedExporter {
	b.exporterMu.RLock()
	defer b.exporterMu.RUnlock()
	return b.exp
}

func waitForInflight(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	default:
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		select {
		case <-done:
			return nil
		default:
			return ctx.Err()
		}
	}
}

func (b *backendLogBatcher) run() {
	defer close(b.done)
	if b.payloadCodec != nil {
		defer func() {
			if err := b.payloadCodec.Close(); err != nil {
				b.logger.Warn("failed to close log batcher payload codec", zap.Error(err))
			}
		}()
		b.runCompressed()
		return
	}

	b.runUncompressed()
}

func (b *backendLogBatcher) runUncompressed() {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	var timerC <-chan time.Time

	pending := plog.NewLogs()
	pendingBytes := 0
	pendingRecords := 0
	sizer := &plog.ProtoMarshaler{}
	var nextReq *logBatcherRequest

	for {
		if nextReq != nil {
			req := *nextReq
			nextReq = nil
			if b.handleRequest(req, sizer, &pending, &pendingRecords, &pendingBytes, &nextReq, timer, &timerC) {
				return
			}
			continue
		}

		select {
		case req := <-b.requests:
			if b.handleRequest(req, sizer, &pending, &pendingRecords, &pendingBytes, &nextReq, timer, &timerC) {
				return
			}
		case <-timerC:
			if err := b.flush(context.Background(), &pending, &pendingRecords, &pendingBytes, logFlushReasonTimeout, timer, &timerC); err != nil {
				b.logger.Warn("failed to flush log batch", zap.String("reason", logFlushReasonTimeout), zap.Error(err))
			}
		}
	}
}

func (b *backendLogBatcher) handleRequest(
	req logBatcherRequest,
	sizer *plog.ProtoMarshaler,
	pending *plog.Logs,
	pendingRecords *int,
	pendingBytes *int,
	nextReq **logBatcherRequest,
	timer *time.Timer,
	timerC *<-chan time.Time,
) bool {
	switch req.kind {
	case logBatcherRequestEnqueue:
		recordsAdded, oldestEnqueue := b.mergeQueuedRequests(pending, req, nextReq)
		*pendingRecords += recordsAdded
		if *pendingRecords > 0 && b.oldestEnqueue.Load() == 0 {
			b.oldestEnqueue.Store(oldestEnqueue)
		}
		// Track max_bytes using the actual serialized merged OTLP payload.
		// Resource/scope dedup during merge means chunk sizes are not additive.
		*pendingBytes = sizer.LogsSize(*pending)
		b.pendingRecords.Store(int64(*pendingRecords))
		b.pendingBytes.Store(int64(*pendingBytes))
		if *pendingRecords > 0 && *timerC == nil {
			timer.Reset(b.settings.flushInterval)
			*timerC = timer.C
		}
		if *pendingRecords >= b.settings.maxRecords || *pendingBytes >= b.settings.maxBytes {
			b.telemetry.overflowTotal.Add(context.Background(), 1, metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint))))
			if err := b.flush(context.Background(), pending, pendingRecords, pendingBytes, logFlushReasonSize, timer, timerC); err != nil {
				b.logger.Warn("failed to flush log batch", zap.String("reason", logFlushReasonSize), zap.Error(err))
			}
		}
	case logBatcherRequestFlushAndStop:
		err := b.flush(req.ctx, pending, pendingRecords, pendingBytes, req.reason, timer, timerC)
		req.done <- err
		return true
	}
	return false
}

func (b *backendLogBatcher) mergeQueuedRequests(pending *plog.Logs, first logBatcherRequest, nextReq **logBatcherRequest) (int, int64) {
	recordCount := 0
	var oldestEnqueue int64

	mergeRequest := func(req logBatcherRequest) {
		count := req.logs.LogRecordCount()
		if count == 0 {
			return
		}
		recordCount += count
		if oldestEnqueue == 0 || req.enqueuedAtUnixNano < oldestEnqueue {
			oldestEnqueue = req.enqueuedAtUnixNano
		}
		*pending = mergeLogs(*pending, req.logs)
	}

	mergeRequest(first)

	for i := 0; i < cap(b.requests); i++ {
		select {
		case req := <-b.requests:
			if req.kind != logBatcherRequestEnqueue {
				*nextReq = &req
				return recordCount, oldestEnqueue
			}
			mergeRequest(req)
		default:
			return recordCount, oldestEnqueue
		}
	}

	return recordCount, oldestEnqueue
}

type compressedLogBatcherChunk struct {
	payload               []byte
	records               int
	uncompressedBytes     int
	compressedBytes       int
	oldestEnqueueUnixNano int64
	sizingResources       []compressedLogBatcherResourceSizing
	sizingMetadata        plog.Logs
}

type compressedLogBatcherResourceSizing struct {
	resource plog.ResourceLogs
	scopes   []compressedLogBatcherScopeSizing
}

type compressedLogBatcherScopeSizing struct {
	scope             plog.ScopeLogs
	recordsDeltaBytes int
}

type compressedLogBatcherSizeState struct {
	metadata  plog.Logs
	resources []compressedLogBatcherResourceSizeState
	bytes     int
}

type compressedLogBatcherResourceSizeState struct {
	resource plog.ResourceLogs
	size     int
	scopes   []compressedLogBatcherScopeSizeState
}

type compressedLogBatcherScopeSizeState struct {
	scope plog.ScopeLogs
	size  int
}

func (b *backendLogBatcher) runCompressed() {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	var timerC <-chan time.Time

	pending := make([]compressedLogBatcherChunk, 0, cap(b.requests))
	pendingBytes := 0
	pendingCompressedBytes := 0
	pendingRecords := 0
	marshaler := &plog.ProtoMarshaler{}
	unmarshaler := &plog.ProtoUnmarshaler{}
	sizeState := newCompressedLogBatcherSizeState()
	var nextReq *logBatcherRequest

	for {
		if nextReq != nil {
			req := *nextReq
			nextReq = nil
			if b.handleCompressedRequest(req, marshaler, unmarshaler, sizeState, &pending, &pendingRecords, &pendingBytes, &pendingCompressedBytes, &nextReq, timer, &timerC) {
				return
			}
			continue
		}

		select {
		case req := <-b.requests:
			if b.handleCompressedRequest(req, marshaler, unmarshaler, sizeState, &pending, &pendingRecords, &pendingBytes, &pendingCompressedBytes, &nextReq, timer, &timerC) {
				return
			}
		case <-timerC:
			if err := b.flushCompressed(context.Background(), marshaler, unmarshaler, sizeState, &pending, &pendingRecords, &pendingBytes, &pendingCompressedBytes, logFlushReasonTimeout, timer, &timerC); err != nil {
				b.logger.Warn("failed to flush compressed log batch", zap.String("reason", logFlushReasonTimeout), zap.Error(err))
			}
		}
	}
}

func (b *backendLogBatcher) handleCompressedRequest(
	req logBatcherRequest,
	marshaler *plog.ProtoMarshaler,
	unmarshaler *plog.ProtoUnmarshaler,
	sizeState *compressedLogBatcherSizeState,
	pending *[]compressedLogBatcherChunk,
	pendingRecords *int,
	pendingBytes *int,
	pendingCompressedBytes *int,
	nextReq **logBatcherRequest,
	timer *time.Timer,
	timerC *<-chan time.Time,
) bool {
	switch req.kind {
	case logBatcherRequestEnqueue:
		recordsAdded, bytesAdded, compressedBytesAdded, oldestEnqueue := b.drainEnqueueRequestsIntoCompressedChunks(marshaler, sizeState, pending, req, nextReq)
		*pendingRecords += recordsAdded
		*pendingBytes += bytesAdded
		*pendingCompressedBytes += compressedBytesAdded
		if *pendingRecords > 0 && b.oldestEnqueue.Load() == 0 {
			b.oldestEnqueue.Store(oldestEnqueue)
		}
		b.pendingRecords.Store(int64(*pendingRecords))
		b.pendingBytes.Store(int64(*pendingBytes))
		b.pendingCompressedBytes.Store(int64(*pendingCompressedBytes))
		if *pendingRecords > 0 && *timerC == nil {
			timer.Reset(b.settings.flushInterval)
			*timerC = timer.C
		}
		if *pendingRecords >= b.settings.maxRecords || *pendingBytes >= b.settings.maxBytes {
			b.telemetry.overflowTotal.Add(context.Background(), 1, metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint))))
			if err := b.flushCompressed(context.Background(), marshaler, unmarshaler, sizeState, pending, pendingRecords, pendingBytes, pendingCompressedBytes, logFlushReasonSize, timer, timerC); err != nil {
				b.logger.Warn("failed to flush compressed log batch", zap.String("reason", logFlushReasonSize), zap.Error(err))
			}
		}
	case logBatcherRequestFlushAndStop:
		err := b.flushCompressed(req.ctx, marshaler, unmarshaler, sizeState, pending, pendingRecords, pendingBytes, pendingCompressedBytes, req.reason, timer, timerC)
		req.done <- err
		return true
	}
	return false
}

func (b *backendLogBatcher) drainEnqueueRequestsIntoCompressedChunks(
	marshaler *plog.ProtoMarshaler,
	sizeState *compressedLogBatcherSizeState,
	pending *[]compressedLogBatcherChunk,
	first logBatcherRequest,
	nextReq **logBatcherRequest,
) (int, int, int, int64) {
	recordCount := 0
	bytes := 0
	compressedBytes := 0
	var oldestEnqueue int64

	appendRequest := func(req logBatcherRequest) {
		count := req.compressedChunk.records
		if count == 0 {
			return
		}
		recordCount += count
		bytes += sizeState.addChunk(req.compressedChunk, marshaler)
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
			if req.kind != logBatcherRequestEnqueue {
				*nextReq = &req
				return recordCount, bytes, compressedBytes, oldestEnqueue
			}
			appendRequest(req)
		default:
			return recordCount, bytes, compressedBytes, oldestEnqueue
		}
	}

	return recordCount, bytes, compressedBytes, oldestEnqueue
}

func newCompressedLogBatcherChunk(marshaler *plog.ProtoMarshaler, codec *queuePayloadCodec, req logBatcherRequest) (compressedLogBatcherChunk, error) {
	payload, err := marshaler.MarshalLogs(req.logs)
	if err != nil {
		return compressedLogBatcherChunk{}, err
	}
	encoded, err := codec.Encode(payload)
	if err != nil {
		return compressedLogBatcherChunk{}, err
	}
	encoded = append([]byte(nil), encoded...)
	encoded = encoded[:len(encoded):len(encoded)]
	sizingMetadata, sizingResources := newCompressedLogBatcherSizing(req.logs, marshaler)
	return compressedLogBatcherChunk{
		payload:               encoded,
		records:               req.logs.LogRecordCount(),
		uncompressedBytes:     len(payload),
		compressedBytes:       len(encoded),
		oldestEnqueueUnixNano: req.enqueuedAtUnixNano,
		sizingResources:       sizingResources,
		sizingMetadata:        sizingMetadata,
	}, nil
}

func newCompressedLogBatcherSizing(logs plog.Logs, marshaler *plog.ProtoMarshaler) (plog.Logs, []compressedLogBatcherResourceSizing) {
	metadata := plog.NewLogs()
	resources := make([]compressedLogBatcherResourceSizing, 0, logs.ResourceLogs().Len())
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		srcRL := logs.ResourceLogs().At(i)
		dstRL := metadata.ResourceLogs().AppendEmpty()
		srcRL.Resource().CopyTo(dstRL.Resource())
		dstRL.SetSchemaUrl(srcRL.SchemaUrl())

		resource := compressedLogBatcherResourceSizing{resource: dstRL}
		for j := 0; j < srcRL.ScopeLogs().Len(); j++ {
			srcSL := srcRL.ScopeLogs().At(j)
			recordsDeltaBytes := 0
			for k := 0; k < srcSL.LogRecords().Len(); k++ {
				recordsDeltaBytes += logProtoDeltaSize(marshaler.LogRecordSize(srcSL.LogRecords().At(k)))
			}
			if recordsDeltaBytes == 0 {
				continue
			}

			dstSL := dstRL.ScopeLogs().AppendEmpty()
			srcSL.Scope().CopyTo(dstSL.Scope())
			dstSL.SetSchemaUrl(srcSL.SchemaUrl())
			resource.scopes = append(resource.scopes, compressedLogBatcherScopeSizing{
				scope:             dstSL,
				recordsDeltaBytes: recordsDeltaBytes,
			})
		}
		resources = append(resources, resource)
	}
	return metadata, resources
}

func newCompressedLogBatcherSizeState() *compressedLogBatcherSizeState {
	return &compressedLogBatcherSizeState{metadata: plog.NewLogs()}
}

func (s *compressedLogBatcherSizeState) reset() {
	s.metadata = plog.NewLogs()
	s.resources = nil
	s.bytes = 0
}

func (s *compressedLogBatcherSizeState) addChunk(chunk compressedLogBatcherChunk, marshaler *plog.ProtoMarshaler) int {
	before := s.bytes
	for _, resource := range chunk.sizingResources {
		s.addResource(resource, marshaler)
	}
	return s.bytes - before
}

func (s *compressedLogBatcherSizeState) addResource(resource compressedLogBatcherResourceSizing, marshaler *plog.ProtoMarshaler) {
	if len(resource.scopes) == 0 {
		return
	}

	resourceIndex := s.findResource(resource.resource)
	resourceIsNew := resourceIndex == -1
	if resourceIsNew {
		dstRL := s.metadata.ResourceLogs().AppendEmpty()
		resource.resource.Resource().CopyTo(dstRL.Resource())
		dstRL.SetSchemaUrl(resource.resource.SchemaUrl())
		s.resources = append(s.resources, compressedLogBatcherResourceSizeState{
			resource: dstRL,
			size:     marshaler.ResourceLogsSize(dstRL),
		})
		resourceIndex = len(s.resources) - 1
	}

	oldResourceSize := s.resources[resourceIndex].size
	for _, scope := range resource.scopes {
		s.addScope(resourceIndex, scope, marshaler)
	}
	if resourceIsNew {
		s.bytes += logProtoDeltaSize(s.resources[resourceIndex].size)
		return
	}
	s.bytes += logProtoDeltaSize(s.resources[resourceIndex].size) - logProtoDeltaSize(oldResourceSize)
}

func (s *compressedLogBatcherSizeState) addScope(resourceIndex int, scope compressedLogBatcherScopeSizing, marshaler *plog.ProtoMarshaler) {
	scopeIndex := s.findScope(resourceIndex, scope.scope)
	if scopeIndex == -1 {
		dstSL := s.resources[resourceIndex].resource.ScopeLogs().AppendEmpty()
		scope.scope.Scope().CopyTo(dstSL.Scope())
		dstSL.SetSchemaUrl(scope.scope.SchemaUrl())
		scopeSize := marshaler.ScopeLogsSize(dstSL) + scope.recordsDeltaBytes
		s.resources[resourceIndex].scopes = append(s.resources[resourceIndex].scopes, compressedLogBatcherScopeSizeState{
			scope: dstSL,
			size:  scopeSize,
		})
		s.resources[resourceIndex].size += logProtoDeltaSize(scopeSize)
		return
	}

	oldScopeSize := s.resources[resourceIndex].scopes[scopeIndex].size
	s.resources[resourceIndex].scopes[scopeIndex].size += scope.recordsDeltaBytes
	newScopeSize := s.resources[resourceIndex].scopes[scopeIndex].size
	s.resources[resourceIndex].size += logProtoDeltaSize(newScopeSize) - logProtoDeltaSize(oldScopeSize)
}

func (s *compressedLogBatcherSizeState) findResource(resource plog.ResourceLogs) int {
	for i, existing := range s.resources {
		if resourceLogsMatches(existing.resource, resource) {
			return i
		}
	}
	return -1
}

func (s *compressedLogBatcherSizeState) findScope(resourceIndex int, scope plog.ScopeLogs) int {
	for i, existing := range s.resources[resourceIndex].scopes {
		if scopeLogsMatches(existing.scope, scope) {
			return i
		}
	}
	return -1
}

func logProtoDeltaSize(newItemSize int) int {
	return 1 + newItemSize + protoSov(uint64(newItemSize))
}

func protoSov(x uint64) int {
	return (bits.Len64(x|1) + 6) / 7
}

func (b *backendLogBatcher) newCompressedLogBatcherChunk(logs plog.Logs, enqueuedAtUnixNano int64) (compressedLogBatcherChunk, error) {
	b.payloadCodecMu.Lock()
	defer b.payloadCodecMu.Unlock()
	return newCompressedLogBatcherChunk(&plog.ProtoMarshaler{}, b.payloadCodec, logBatcherRequest{
		logs:               logs,
		enqueuedAtUnixNano: enqueuedAtUnixNano,
	})
}

func decodeCompressedLogBatcherChunk(unmarshaler *plog.ProtoUnmarshaler, codec *queuePayloadCodec, chunk compressedLogBatcherChunk) (plog.Logs, error) {
	payload, err := codec.Decode(chunk.payload)
	if err != nil {
		return plog.Logs{}, err
	}
	return unmarshaler.UnmarshalLogs(payload)
}

func (b *backendLogBatcher) flush(
	ctx context.Context,
	pending *plog.Logs,
	pendingRecords *int,
	pendingBytes *int,
	reason string,
	timer *time.Timer,
	timerC *<-chan time.Time,
) error {
	if !timer.Stop() && *timerC != nil {
		select {
		case <-timer.C:
		default:
		}
	}
	*timerC = nil

	if pending.LogRecordCount() == 0 {
		return nil
	}

	drained := *pending
	records := *pendingRecords
	bytes := *pendingBytes
	oldestAgeMillis := ageMillisFromUnixNano(time.Now(), b.oldestEnqueue.Load())
	*pending = plog.NewLogs()
	*pendingRecords = 0
	*pendingBytes = 0
	b.pendingRecords.Store(0)
	b.pendingBytes.Store(0)
	b.oldestEnqueue.Store(0)

	b.telemetry.batchSize.Record(ctx, int64(records), metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint))))
	b.telemetry.batchBytes.Record(ctx, int64(bytes), metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint))))
	b.telemetry.flushOldestRecordAge.Record(ctx, oldestAgeMillis, metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint), attribute.String("reason", reason))))
	b.telemetry.flushTotal.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint), attribute.String("reason", reason))))

	var rerouteLogs plog.Logs
	if b.drainErr != nil && reason != logFlushReasonShutdown {
		rerouteLogs = plog.NewLogs()
		drained.CopyTo(rerouteLogs)
	}
	exp := b.exporter()
	if exp != nil && exp.isStopping() && b.drainErr != nil && reason != logFlushReasonShutdown {
		rerouteErr := b.drainErr(ctx, rerouteLogs, reason)
		if rerouteErr == nil {
			return nil
		}
		b.telemetry.flushErrors.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint))))
		b.telemetry.droppedRecords.Add(ctx, int64(records), metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint))))
		return rerouteErr
	}
	err := b.send(ctx, exp, drained, reason)
	if err != nil {
		b.telemetry.flushErrors.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint))))
		if b.drainErr != nil && reason != logFlushReasonShutdown {
			var rerouteable logBatcherRerouteableError
			if errors.As(err, &rerouteable) {
				rerouteErr := b.drainErr(ctx, rerouteLogs, reason)
				rerouteable.RecordReroute(ctx, rerouteErr)
				if rerouteErr == nil {
					return nil
				}
				err = errors.Join(err, rerouteErr)
			}
		}
		b.telemetry.droppedRecords.Add(ctx, int64(records), metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint))))
	}
	return err
}

func (b *backendLogBatcher) flushCompressed(
	ctx context.Context,
	sizer *plog.ProtoMarshaler,
	unmarshaler *plog.ProtoUnmarshaler,
	sizeState *compressedLogBatcherSizeState,
	pending *[]compressedLogBatcherChunk,
	pendingRecords *int,
	pendingBytes *int,
	pendingCompressedBytes *int,
	reason string,
	timer *time.Timer,
	timerC *<-chan time.Time,
) error {
	if !timer.Stop() && *timerC != nil {
		select {
		case <-timer.C:
		default:
		}
	}
	*timerC = nil

	if *pendingRecords == 0 {
		return nil
	}

	drainedChunks := *pending
	*pending = make([]compressedLogBatcherChunk, 0, cap(b.requests))
	*pendingRecords = 0
	*pendingBytes = 0
	*pendingCompressedBytes = 0
	sizeState.reset()
	b.pendingRecords.Store(0)
	b.pendingBytes.Store(0)
	b.pendingCompressedBytes.Store(0)
	b.oldestEnqueue.Store(0)
	defer clearCompressedLogBatcherChunks(drainedChunks)

	var errs error
	for len(drainedChunks) > 0 {
		var window []compressedLogBatcherChunk
		var records int
		var oldestUnixNano int64
		window, drainedChunks, records, oldestUnixNano = nextCompressedLogBatcherWindow(drainedChunks, b.settings.maxRecords, b.settings.maxBytes, sizer)

		b.payloadCodecMu.Lock()
		drained, err := mergeCompressedLogBatcherChunks(unmarshaler, b.payloadCodec, window)
		b.payloadCodecMu.Unlock()
		if err != nil {
			b.telemetry.flushErrors.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint))))
			b.telemetry.droppedRecords.Add(ctx, int64(records), metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint))))
			errs = errors.Join(errs, err)
			continue
		}

		bytes := sizer.LogsSize(drained)
		oldestAgeMillis := ageMillisFromUnixNano(time.Now(), oldestUnixNano)
		attrs := metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint)))
		flushAttrs := metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", b.endpoint), attribute.String("reason", reason)))
		b.telemetry.batchSize.Record(ctx, int64(records), attrs)
		b.telemetry.batchBytes.Record(ctx, int64(bytes), attrs)
		b.telemetry.flushOldestRecordAge.Record(ctx, oldestAgeMillis, flushAttrs)
		b.telemetry.flushTotal.Add(ctx, 1, flushAttrs)

		var rerouteLogs plog.Logs
		if b.drainErr != nil && reason != logFlushReasonShutdown {
			rerouteLogs = plog.NewLogs()
			drained.CopyTo(rerouteLogs)
		}
		exp := b.exporter()
		if exp != nil && exp.isStopping() && b.drainErr != nil && reason != logFlushReasonShutdown {
			rerouteErr := b.drainErr(ctx, rerouteLogs, reason)
			if rerouteErr == nil {
				continue
			}
			b.telemetry.flushErrors.Add(ctx, 1, attrs)
			b.telemetry.droppedRecords.Add(ctx, int64(records), attrs)
			errs = errors.Join(errs, rerouteErr)
			continue
		}
		err = b.send(ctx, exp, drained, reason)
		if err != nil {
			b.telemetry.flushErrors.Add(ctx, 1, attrs)
			if b.drainErr != nil && reason != logFlushReasonShutdown {
				var rerouteable logBatcherRerouteableError
				if errors.As(err, &rerouteable) {
					rerouteErr := b.drainErr(ctx, rerouteLogs, reason)
					rerouteable.RecordReroute(ctx, rerouteErr)
					if rerouteErr == nil {
						continue
					}
					err = errors.Join(err, rerouteErr)
				}
			}
			b.telemetry.droppedRecords.Add(ctx, int64(records), attrs)
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func nextCompressedLogBatcherWindow(chunks []compressedLogBatcherChunk, maxRecords, maxBytes int, sizer *plog.ProtoMarshaler) ([]compressedLogBatcherChunk, []compressedLogBatcherChunk, int, int64) {
	records := 0
	var oldestUnixNano int64
	sizeState := newCompressedLogBatcherSizeState()
	end := 0
	for end < len(chunks) {
		chunk := chunks[end]
		sizeState.addChunk(chunk, sizer)
		if end > 0 && (records+chunk.records > maxRecords || sizeState.bytes > maxBytes) {
			break
		}
		records += chunk.records
		if oldestUnixNano == 0 || chunk.oldestEnqueueUnixNano < oldestUnixNano {
			oldestUnixNano = chunk.oldestEnqueueUnixNano
		}
		end++
		if records >= maxRecords || sizeState.bytes >= maxBytes {
			break
		}
	}
	return chunks[:end], chunks[end:], records, oldestUnixNano
}

func mergeCompressedLogBatcherChunks(unmarshaler *plog.ProtoUnmarshaler, codec *queuePayloadCodec, chunks []compressedLogBatcherChunk) (plog.Logs, error) {
	if len(chunks) == 0 {
		return plog.NewLogs(), nil
	}

	merged, err := decodeCompressedLogBatcherChunk(unmarshaler, codec, chunks[0])
	if err != nil {
		return plog.Logs{}, err
	}
	for i := 1; i < len(chunks); i++ {
		logs, err := decodeCompressedLogBatcherChunk(unmarshaler, codec, chunks[i])
		if err != nil {
			return plog.Logs{}, err
		}
		mergeLogChunksByMove(merged, logs)
	}
	return merged, nil
}

func mergeLogChunksByMove(dst, src plog.Logs) {
	dstResourceLogs := dst.ResourceLogs()
	srcResourceLogs := src.ResourceLogs()
	for i := 0; i < srcResourceLogs.Len(); i++ {
		srcRL := srcResourceLogs.At(i)
		dstRL, ok := findMatchingResourceLogs(dst, srcRL)
		if !ok {
			dstRL = dstResourceLogs.AppendEmpty()
			srcRL.MoveTo(dstRL)
			continue
		}

		mergeResourceLogsByMove(dstRL, srcRL)
	}
}

func mergeResourceLogsByMove(dst, src plog.ResourceLogs) {
	dstScopeLogs := dst.ScopeLogs()
	srcScopeLogs := src.ScopeLogs()
	for i := 0; i < srcScopeLogs.Len(); i++ {
		srcSL := srcScopeLogs.At(i)
		dstSL, ok := findMatchingScopeLogs(dst, srcSL)
		if !ok {
			dstSL = dstScopeLogs.AppendEmpty()
			srcSL.MoveTo(dstSL)
			continue
		}

		srcSL.LogRecords().MoveAndAppendTo(dstSL.LogRecords())
	}
}

func findMatchingResourceLogs(dest plog.Logs, src plog.ResourceLogs) (plog.ResourceLogs, bool) {
	for i := 0; i < dest.ResourceLogs().Len(); i++ {
		rl := dest.ResourceLogs().At(i)
		if resourceLogsMatches(rl, src) {
			return rl, true
		}
	}
	return plog.ResourceLogs{}, false
}

func findMatchingScopeLogs(dest plog.ResourceLogs, src plog.ScopeLogs) (plog.ScopeLogs, bool) {
	for i := 0; i < dest.ScopeLogs().Len(); i++ {
		sl := dest.ScopeLogs().At(i)
		if scopeLogsMatches(sl, src) {
			return sl, true
		}
	}
	return plog.ScopeLogs{}, false
}

func clearCompressedLogBatcherChunks(chunks []compressedLogBatcherChunk) {
	for i := range chunks {
		chunks[i] = compressedLogBatcherChunk{}
	}
}

type logBatcherPending struct {
	endpoint        string
	records         int64
	bytes           int64
	compressedBytes int64
	oldestAgeMillis int64
}

func newLogBatcherTelemetry(settings component.TelemetrySettings) (*logBatcherTelemetry, error) {
	meter := metadata.Meter(settings)
	var err, errs error

	t := &logBatcherTelemetry{logger: settings.Logger, meter: meter}
	t.batchSize, err = meter.Int64Histogram(
		"otelcol_loadbalancer_log_batch_size",
		metric.WithDescription("Number of log records per flushed backend batch."),
		metric.WithUnit("{records}"),
	)
	errs = errors.Join(errs, err)
	t.batchBytes, err = meter.Int64Histogram(
		"otelcol_loadbalancer_log_batch_bytes",
		metric.WithDescription("Serialized OTLP bytes per flushed backend batch before compression."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.flushTotal, err = meter.Int64Counter(
		"otelcol_loadbalancer_log_batch_flush_total",
		metric.WithDescription("Number of log batch flushes by endpoint and reason."),
		metric.WithUnit("{flushes}"),
	)
	errs = errors.Join(errs, err)
	t.flushErrors, err = meter.Int64Counter(
		"otelcol_loadbalancer_log_batch_flush_errors",
		metric.WithDescription("Number of log batch flush errors."),
		metric.WithUnit("{errors}"),
	)
	errs = errors.Join(errs, err)
	t.flushOldestRecordAge, err = meter.Int64Histogram(
		"otelcol_loadbalancer_log_batch_flush_oldest_record_age",
		metric.WithDescription("Age in ms of the oldest log record in a flushed backend batch."),
		metric.WithUnit("ms"),
	)
	errs = errors.Join(errs, err)
	t.droppedRecords, err = meter.Int64Counter(
		"otelcol_loadbalancer_log_batch_dropped_records",
		metric.WithDescription("Number of dropped log records in the internal log batcher."),
		metric.WithUnit("{records}"),
	)
	errs = errors.Join(errs, err)
	t.overflowTotal, err = meter.Int64Counter(
		"otelcol_loadbalancer_log_batch_overflow_total",
		metric.WithDescription("Number of times an internal log batch hit a size bound and was force-flushed."),
		metric.WithUnit("{overflows}"),
	)
	errs = errors.Join(errs, err)
	t.pendingRecords, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_log_batch_pending_records",
		metric.WithDescription("Current number of pending log records per backend batch."),
		metric.WithUnit("{records}"),
	)
	errs = errors.Join(errs, err)
	t.pendingBytes, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_log_batch_pending_bytes",
		metric.WithDescription("Current serialized OTLP bytes per pending backend batch before compression."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.pendingCompressedBytes, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_log_batch_pending_compressed_bytes",
		metric.WithDescription("Current compressed serialized OTLP bytes per pending backend batch."),
		metric.WithUnit("By"),
	)
	errs = errors.Join(errs, err)
	t.pendingOldestAge, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_log_batch_pending_oldest_record_age",
		metric.WithDescription("Age in ms of the oldest pending log record per backend batch."),
		metric.WithUnit("ms"),
	)
	errs = errors.Join(errs, err)
	t.pendingOldestAgeMax, err = meter.Int64ObservableGauge(
		"otelcol_loadbalancer_log_batch_pending_oldest_record_age_max",
		metric.WithDescription("Maximum age in ms of the oldest pending log record across backend batches."),
		metric.WithUnit("ms"),
	)
	errs = errors.Join(errs, err)

	return t, errs
}

func (t *logBatcherTelemetry) start(snapshot func() logBatcherSnapshot) error {
	reg, err := t.meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		state := snapshot()
		for _, pending := range state.pending {
			attrs := metric.WithAttributeSet(attribute.NewSet(attribute.String("endpoint", pending.endpoint)))
			observer.ObserveInt64(t.pendingRecords, pending.records, attrs)
			observer.ObserveInt64(t.pendingBytes, pending.bytes, attrs)
			observer.ObserveInt64(t.pendingCompressedBytes, pending.compressedBytes, attrs)
			observer.ObserveInt64(t.pendingOldestAge, pending.oldestAgeMillis, attrs)
		}
		observer.ObserveInt64(t.pendingOldestAgeMax, state.maxOldestAgeMillis)
		return nil
	}, t.pendingRecords, t.pendingBytes, t.pendingCompressedBytes, t.pendingOldestAge, t.pendingOldestAgeMax)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.registrations = append(t.registrations, reg)
	t.mu.Unlock()
	return nil
}

func (t *logBatcherTelemetry) shutdown() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, reg := range t.registrations {
		if err := reg.Unregister(); err != nil {
			t.logger.Warn("failed to unregister log batcher metric callback", zap.Error(err))
		}
	}
	t.registrations = nil
}

func ageMillisFromUnixNano(now time.Time, unixNano int64) int64 {
	if unixNano <= 0 {
		return 0
	}

	age := now.Sub(time.Unix(0, unixNano)).Milliseconds()
	if age < 0 {
		return 0
	}
	return age
}
