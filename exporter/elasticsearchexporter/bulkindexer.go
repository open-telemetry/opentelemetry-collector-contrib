// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/logging"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

type bulkIndexer interface {
	// StartSession starts a new bulk indexing session.
	StartSession(context.Context) bulkIndexerSession

	// Close closes the bulk indexer, ending any in-progress
	// sessions and stopping any background processing.
	Close(ctx context.Context) error
}

type bulkIndexerSession interface {
	// Add adds a document to the bulk indexing session.
	Add(ctx context.Context, index, docID, pipeline string, document io.WriterTo, dynamicTemplates map[string]string, action string) error

	// End must be called on the session object once it is no longer
	// needed, in order to release any associated resources.
	//
	// Note that ending the session does _not_ implicitly flush
	// documents. Call Flush before calling End as needed.
	//
	// Calling other methods (including End) after End may panic.
	End()

	// Flush flushes any documents added to the bulk indexing session.
	//
	// The behavior of Flush depends on whether the bulk indexer is
	// synchronous or asynchronous. Calling Flush on an asynchronous bulk
	// indexer session is effectively a no-op; flushing will be done in
	// the background. Calling Flush on a synchronous bulk indexer session
	// will wait for bulk indexing of added documents to complete,
	// successfully or not.
	Flush(context.Context) error
}

const defaultMaxRetries = 2

func newBulkIndexer(
	client esapi.Transport,
	config *Config,
	requireDataStream bool,
	tb *metadata.TelemetryBuilder,
	logger *zap.Logger,
) (bulkIndexer, error) {
	if config.Batcher.enabledSet || (config.QueueBatchConfig.Enabled && config.QueueBatchConfig.Batch.HasValue()) {
		return newSyncBulkIndexer(client, config, requireDataStream, tb, logger), nil
	}
	return newAsyncBulkIndexer(client, config, requireDataStream, tb, logger)
}

func bulkIndexerConfig(client esapi.Transport, config *Config, requireDataStream bool) docappender.BulkIndexerConfig {
	var maxDocRetries int
	if config.Retry.Enabled {
		maxDocRetries = defaultMaxRetries
		if config.Retry.MaxRetries != 0 {
			maxDocRetries = config.Retry.MaxRetries
		}
	}
	var compressionLevel int
	if config.Compression == configcompression.TypeGzip {
		compressionLevel = int(config.CompressionParams.Level)
	}
	return docappender.BulkIndexerConfig{
		Client:                  client,
		MaxDocumentRetries:      maxDocRetries,
		Pipeline:                config.Pipeline,
		RetryOnDocumentStatus:   config.Retry.RetryOnStatus,
		RequireDataStream:       requireDataStream,
		CompressionLevel:        compressionLevel,
		PopulateFailedDocsInput: config.LogFailedDocsInput,
		IncludeSourceOnError:    bulkIndexerIncludeSourceOnError(config.IncludeSourceOnError),
	}
}

func bulkIndexerIncludeSourceOnError(includeSourceOnError *bool) docappender.Value {
	if includeSourceOnError == nil {
		return docappender.Unset
	}
	if *includeSourceOnError {
		return docappender.True
	}
	return docappender.False
}

func newSyncBulkIndexer(
	client esapi.Transport,
	config *Config,
	requireDataStream bool,
	tb *metadata.TelemetryBuilder,
	logger *zap.Logger,
) *syncBulkIndexer {
	return &syncBulkIndexer{
		config:                bulkIndexerConfig(client, config, requireDataStream),
		flushTimeout:          config.Timeout,
		flushBytes:            config.Flush.Bytes,
		retryConfig:           config.Retry,
		metadataKeys:          config.MetadataKeys,
		telemetryBuilder:      tb,
		logger:                logger,
		failedDocsInputLogger: newFailedDocsInputLogger(logger, config),
	}
}

type syncBulkIndexer struct {
	config                docappender.BulkIndexerConfig
	flushTimeout          time.Duration
	flushBytes            int
	retryConfig           RetrySettings
	metadataKeys          []string
	telemetryBuilder      *metadata.TelemetryBuilder
	logger                *zap.Logger
	failedDocsInputLogger *zap.Logger
}

// StartSession creates a new docappender.BulkIndexer, and wraps
// it with a syncBulkIndexerSession.
func (s *syncBulkIndexer) StartSession(context.Context) bulkIndexerSession {
	bi, err := docappender.NewBulkIndexer(s.config)
	if err != nil {
		// This should never happen in practice:
		// NewBulkIndexer should only fail if the
		// config is invalid, and we expect it to
		// always be valid at this point.
		return errBulkIndexerSession{err: err}
	}
	return &syncBulkIndexerSession{s: s, bi: bi}
}

// Close is a no-op.
func (*syncBulkIndexer) Close(context.Context) error {
	return nil
}

type syncBulkIndexerSession struct {
	s  *syncBulkIndexer
	bi *docappender.BulkIndexer
}

// Add adds an item to the sync bulk indexer session.
func (s *syncBulkIndexerSession) Add(ctx context.Context, index, docID, pipeline string, document io.WriterTo, dynamicTemplates map[string]string, action string) error {
	doc := docappender.BulkIndexerItem{
		Index:            index,
		Body:             document,
		DocumentID:       docID,
		DynamicTemplates: dynamicTemplates,
		Action:           action,
		Pipeline:         pipeline,
	}
	err := s.bi.Add(doc)
	if err != nil {
		return err
	}
	s.s.telemetryBuilder.ElasticsearchDocsReceived.Add(
		ctx, 1,
		metric.WithAttributeSet(attribute.NewSet(
			getAttributesFromMetadataKeys(ctx, s.s.metadataKeys)...),
		),
	)
	// flush bytes should operate on uncompressed length
	// as Elasticsearch http.max_content_length measures uncompressed length.
	if s.bi.UncompressedLen() >= s.s.flushBytes {
		return s.Flush(ctx)
	}
	return nil
}

// End is a no-op.
func (*syncBulkIndexerSession) End() {
	// TODO acquire docappender.BulkIndexer from pool in StartSession, release here
}

// Flush flushes documents added to the bulk indexer session.
func (s *syncBulkIndexerSession) Flush(ctx context.Context) error {
	var retryBackoff func(int) time.Duration
	for attempts := 0; ; attempts++ {
		if err := flushBulkIndexer(
			ctx,
			s.bi,
			s.s.flushTimeout,
			s.s.metadataKeys,
			s.s.telemetryBuilder,
			s.s.logger,
			s.s.failedDocsInputLogger,
		); err != nil {
			return err
		}
		if s.bi.Items() == 0 {
			// No documents in buffer waiting for per-document retry, exit retry loop.
			return nil
		}
		if retryBackoff == nil {
			retryBackoff = createElasticsearchBackoffFunc(&s.s.retryConfig)
			if retryBackoff == nil {
				// BUG: This should never happen in practice.
				// When retry is disabled / document level retry limit is reached,
				// documents should go into FailedDocs instead of indexer buffer.
				return errors.New("bulk indexer contains documents pending retry but retry is disabled")
			}
		}
		backoff := retryBackoff(attempts + 1) // TODO: use exporterhelper retry_sender
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func newAsyncBulkIndexer(
	client esapi.Transport,
	config *Config,
	requireDataStream bool,
	tb *metadata.TelemetryBuilder,
	logger *zap.Logger,
) (*asyncBulkIndexer, error) {
	numWorkers := config.NumWorkers
	if numWorkers == 0 {
		numWorkers = runtime.NumCPU()
	}

	pool := &asyncBulkIndexer{
		wg:               sync.WaitGroup{},
		items:            make(chan docappender.BulkIndexerItem, config.NumWorkers),
		telemetryBuilder: tb,
	}
	pool.wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		bi, err := docappender.NewBulkIndexer(bulkIndexerConfig(client, config, requireDataStream))
		if err != nil {
			return nil, err
		}
		w := asyncBulkIndexerWorker{
			indexer:               bi,
			items:                 pool.items,
			flushInterval:         config.Flush.Interval,
			flushTimeout:          config.Timeout,
			flushBytes:            config.Flush.Bytes,
			telemetryBuilder:      tb,
			logger:                logger,
			failedDocsInputLogger: newFailedDocsInputLogger(logger, config),
		}
		go func() {
			defer pool.wg.Done()
			w.run()
		}()
	}
	return pool, nil
}

type asyncBulkIndexer struct {
	items            chan docappender.BulkIndexerItem
	wg               sync.WaitGroup
	telemetryBuilder *metadata.TelemetryBuilder
}

type asyncBulkIndexerSession struct {
	*asyncBulkIndexer
}

// StartSession returns a new asyncBulkIndexerSession.
func (a *asyncBulkIndexer) StartSession(context.Context) bulkIndexerSession {
	return asyncBulkIndexerSession{a}
}

// Close closes the asyncBulkIndexer and any active sessions.
func (a *asyncBulkIndexer) Close(ctx context.Context) error {
	close(a.items)
	doneCh := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(doneCh)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
		return nil
	}
}

// Add adds an item to the async bulk indexer session.
//
// Adding an item after a call to Close() will panic.
func (s asyncBulkIndexerSession) Add(ctx context.Context, index, docID, pipeline string, document io.WriterTo, dynamicTemplates map[string]string, action string) error {
	item := docappender.BulkIndexerItem{
		Index:            index,
		Body:             document,
		DocumentID:       docID,
		DynamicTemplates: dynamicTemplates,
		Action:           action,
		Pipeline:         pipeline,
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.items <- item:
	}
	s.telemetryBuilder.ElasticsearchDocsReceived.Add(ctx, 1)
	return nil
}

// End is a no-op.
func (asyncBulkIndexerSession) End() {
}

// Flush is a no-op.
func (asyncBulkIndexerSession) Flush(context.Context) error {
	return nil
}

type asyncBulkIndexerWorker struct {
	indexer       *docappender.BulkIndexer
	items         <-chan docappender.BulkIndexerItem
	flushInterval time.Duration
	flushTimeout  time.Duration
	flushBytes    int

	logger                *zap.Logger
	failedDocsInputLogger *zap.Logger
	telemetryBuilder      *metadata.TelemetryBuilder
}

func (w *asyncBulkIndexerWorker) run() {
	flushTick := time.NewTicker(w.flushInterval)
	defer flushTick.Stop()
	for {
		select {
		case item, ok := <-w.items:
			// if channel is closed, flush and return
			if !ok {
				w.flush()
				return
			}

			if err := w.indexer.Add(item); err != nil {
				w.logger.Error("error adding item to bulk indexer", zap.Error(err))
			}

			// flush bytes should operate on uncompressed length
			// as Elasticsearch http.max_content_length measures uncompressed length.
			if w.indexer.UncompressedLen() >= w.flushBytes {
				w.flush()
				flushTick.Reset(w.flushInterval)
			}
		case <-flushTick.C:
			// bulk indexer needs to be flushed every flush interval because
			// there may be pending bytes in bulk indexer buffer due to e.g. document level 429
			w.flush()
		}
	}
}

func (w *asyncBulkIndexerWorker) flush() {
	// TODO (lahsivjar): Should use proper context else client metadata will not be accessible
	ctx := context.Background()
	// ignore error as we they should be already logged and for async we don't propagate errors
	_ = flushBulkIndexer(
		ctx,
		w.indexer,
		w.flushTimeout,
		nil, // async bulk indexer cannot propagate client context/metadata
		w.telemetryBuilder,
		w.logger,
		w.failedDocsInputLogger,
	)
}

func flushBulkIndexer(
	ctx context.Context,
	bi *docappender.BulkIndexer,
	timeout time.Duration,
	tMetaKeys []string,
	tb *metadata.TelemetryBuilder,
	logger *zap.Logger,
	failedDocsInputLogger *zap.Logger,
) error {
	itemsCount := bi.Items()
	if itemsCount == 0 {
		return nil
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	startTime := time.Now()
	stat, err := bi.Flush(ctx)
	latency := time.Since(startTime).Seconds()
	defaultMetaAttrs := getAttributesFromMetadataKeys(ctx, tMetaKeys)
	defaultAttrsSet := attribute.NewSet(defaultMetaAttrs...)
	if flushed := bi.BytesFlushed(); flushed > 0 {
		tb.ElasticsearchFlushedBytes.Add(ctx, int64(flushed), metric.WithAttributeSet(defaultAttrsSet))
	}
	if flushed := bi.BytesUncompressedFlushed(); flushed > 0 {
		tb.ElasticsearchFlushedUncompressedBytes.Add(
			ctx, int64(flushed), metric.WithAttributeSet(defaultAttrsSet),
		)
	}

	var fields []zap.Field
	// append metadata attributes to error log fields
	for _, kv := range defaultMetaAttrs {
		switch kv.Value.Type() {
		case attribute.STRINGSLICE:
			fields = append(fields, zap.Strings(string(kv.Key), kv.Value.AsStringSlice()))
		default:
			// For other types, convert to string
			fields = append(fields, zap.String(string(kv.Key), kv.Value.AsString()))
		}
	}
	if err != nil {
		logger.Error("bulk indexer flush error", append(fields, zap.Error(err))...)
		var bulkFailedErr docappender.ErrorFlushFailed
		switch {
		case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
			attrSet := metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{attribute.String("outcome", "timeout")}, defaultMetaAttrs...)...,
			))
			tb.ElasticsearchDocsProcessed.Add(ctx, int64(itemsCount), attrSet)
			tb.ElasticsearchBulkRequestsCount.Add(ctx, int64(1), attrSet)
			tb.ElasticsearchBulkRequestsLatency.Record(ctx, latency, attrSet)
		case errors.As(err, &bulkFailedErr):
			var outcome string
			code := bulkFailedErr.StatusCode()
			switch {
			case code == http.StatusTooManyRequests:
				outcome = "too_many"
			case code >= 500:
				outcome = "failed_server"
			case code >= 400:
				outcome = "failed_client"
			}
			attrSet := metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					semconv.HTTPResponseStatusCode(code),
					attribute.String("outcome", outcome),
				}, defaultMetaAttrs...)...,
			))
			tb.ElasticsearchDocsProcessed.Add(ctx, int64(itemsCount), attrSet)
			tb.ElasticsearchBulkRequestsCount.Add(ctx, int64(1), attrSet)
			tb.ElasticsearchBulkRequestsLatency.Record(ctx, latency, attrSet)
		default:
			attrSet := metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", "internal_server_error"),
					semconv.HTTPResponseStatusCode(http.StatusInternalServerError),
				}, defaultMetaAttrs...)...,
			))
			tb.ElasticsearchDocsProcessed.Add(ctx, int64(itemsCount), attrSet)
			tb.ElasticsearchBulkRequestsCount.Add(ctx, int64(1), attrSet)
			tb.ElasticsearchBulkRequestsLatency.Record(ctx, latency, attrSet)
		}
	} else {
		// Record a successful completed bulk request
		successAttrSet := metric.WithAttributeSet(attribute.NewSet(
			append([]attribute.KeyValue{
				attribute.String("outcome", "success"),
				semconv.HTTPResponseStatusCode(http.StatusOK),
			}, defaultMetaAttrs...)...,
		))

		tb.ElasticsearchBulkRequestsCount.Add(ctx, int64(1), successAttrSet)
		tb.ElasticsearchBulkRequestsLatency.Record(ctx, latency, successAttrSet)
	}

	for _, resp := range stat.FailedDocs {
		// Collect telemetry
		var outcome string
		switch {
		case resp.Status == http.StatusTooManyRequests:
			outcome = "too_many"
		case resp.Status >= 500:
			outcome = "failed_server"
		case resp.Status >= 400:
			outcome = "failed_client"
		}

		tb.ElasticsearchDocsProcessed.Add(
			ctx,
			int64(1),
			metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", outcome),
					attribute.String("error.type", resp.Error.Type),
				}, defaultMetaAttrs...)...,
			)),
		)

		if resp.Error.Type == "version_conflict_engine_exception" &&
			(strings.HasPrefix(resp.Index, ".profiling-stackframes-") ||
				strings.HasPrefix(resp.Index, ".profiling-stacktraces-")) {
			// For the Profiling indices .profiling-[stacktraces|stackframes]- the
			// rejection of duplicates are expected from Elasticsearch. So we do not want
			// to log these here.
			continue
		}

		// Log failed docs
		fields = append(fields,
			zap.String("index", resp.Index),
			zap.String("error.type", resp.Error.Type),
			zap.String("error.reason", resp.Error.Reason),
		)

		if hint := getErrorHint(resp.Index, resp.Error.Type); hint != "" {
			fields = append(fields, zap.String("hint", hint))
		}
		logger.Error("failed to index document", fields...)

		if resp.Input != "" {
			fields = append(fields, zap.String("input", resp.Input))
		}
		failedDocsInputLogger.Debug("failed to index document; input may contain sensitive data", fields...)
	}
	if stat.Indexed > 0 {
		tb.ElasticsearchDocsProcessed.Add(
			ctx,
			stat.Indexed,
			metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", "success"),
				}, defaultMetaAttrs...)...,
			)),
		)
	}
	if stat.FailureStoreDocs.Used > 0 {
		tb.ElasticsearchDocsProcessed.Add(
			ctx,
			stat.FailureStoreDocs.Used,
			metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", "failure_store"),
					attribute.String("failure_store", string(docappender.FailureStoreStatusUsed)),
				}, defaultMetaAttrs...)...,
			)),
		)
	}
	if stat.FailureStoreDocs.Failed > 0 {
		tb.ElasticsearchDocsProcessed.Add(
			ctx,
			stat.FailureStoreDocs.Failed,
			metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", "failure_store"),
					attribute.String("failure_store", string(docappender.FailureStoreStatusFailed)),
				}, defaultMetaAttrs...)...,
			)),
		)
	}
	if stat.FailureStoreDocs.NotEnabled > 0 {
		tb.ElasticsearchDocsProcessed.Add(
			ctx,
			stat.FailureStoreDocs.NotEnabled,
			metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", "failure_store"),
					attribute.String("failure_store", string(docappender.FailureStoreStatusNotEnabled)),
				}, defaultMetaAttrs...)...,
			)),
		)
	}
	if stat.RetriedDocs > 0 {
		tb.ElasticsearchDocsRetried.Add(ctx, stat.RetriedDocs, metric.WithAttributeSet(defaultAttrsSet))
	}
	return err
}

func getAttributesFromMetadataKeys(ctx context.Context, keys []string) []attribute.KeyValue {
	clientInfo := client.FromContext(ctx)
	attrs := make([]attribute.KeyValue, 0, len(keys))
	for _, k := range keys {
		if values := clientInfo.Metadata.Get(k); len(values) != 0 {
			attrs = append(attrs, attribute.StringSlice(k, values))
		}
	}
	return attrs
}

func getErrorHint(index, errorType string) string {
	if strings.HasPrefix(index, ".ds-metrics-") && errorType == "version_conflict_engine_exception" {
		return "check the \"Known issues\" section of Elasticsearch Exporter docs"
	}
	return ""
}

func newFailedDocsInputLogger(logger *zap.Logger, config *Config) *zap.Logger {
	if !config.LogFailedDocsInput {
		return zap.NewNop()
	}
	return logger.WithOptions(logging.WithRateLimit(config.LogFailedDocsInputRateLimit))
}

type bulkIndexers struct {
	// wg tracks active sessions
	wg sync.WaitGroup

	// NOTE(axw) when we get rid of the async bulk indexer there would be
	// no reason for having one per mode or for different document types.
	// Instead, the caller can create separate sessions as needed, and we
	// can either have one for required_data_stream=true and one for false,
	// or callers can set this per document.

	modes                [NumMappingModes]bulkIndexer
	profilingEvents      bulkIndexer // For profiling-events-*
	profilingStackTraces bulkIndexer // For profiling-stacktraces
	profilingStackFrames bulkIndexer // For profiling-stackframes
	profilingExecutables bulkIndexer // For profiling-executables

	telemetryBuilder *metadata.TelemetryBuilder
}

func (b *bulkIndexers) start(
	ctx context.Context,
	cfg *Config,
	set exporter.Settings,
	host component.Host,
	allowedMappingModes map[string]MappingMode,
) error {
	userAgent := fmt.Sprintf(
		"%s/%s (%s/%s)",
		set.BuildInfo.Description,
		set.BuildInfo.Version,
		runtime.GOOS,
		runtime.GOARCH,
	)

	esClient, err := newElasticsearchClient(ctx, cfg, host, set.TelemetrySettings, userAgent)
	if err != nil {
		return err
	}

	for _, mode := range allowedMappingModes {
		var bi bulkIndexer
		bi, err = newBulkIndexer(esClient, cfg, mode == MappingOTel, b.telemetryBuilder, set.Logger)
		if err != nil {
			return err
		}
		b.modes[mode] = &wgTrackingBulkIndexer{bulkIndexer: bi, wg: &b.wg}
	}

	profilingEvents, err := newBulkIndexer(esClient, cfg, true, b.telemetryBuilder, set.Logger)
	if err != nil {
		return err
	}
	b.profilingEvents = &wgTrackingBulkIndexer{bulkIndexer: profilingEvents, wg: &b.wg}

	profilingStackTraces, err := newBulkIndexer(esClient, cfg, false, b.telemetryBuilder, set.Logger)
	if err != nil {
		return err
	}
	b.profilingStackTraces = &wgTrackingBulkIndexer{bulkIndexer: profilingStackTraces, wg: &b.wg}

	profilingStackFrames, err := newBulkIndexer(esClient, cfg, false, b.telemetryBuilder, set.Logger)
	if err != nil {
		return err
	}
	b.profilingStackFrames = &wgTrackingBulkIndexer{bulkIndexer: profilingStackFrames, wg: &b.wg}

	profilingExecutables, err := newBulkIndexer(esClient, cfg, false, b.telemetryBuilder, set.Logger)
	if err != nil {
		return err
	}
	b.profilingExecutables = &wgTrackingBulkIndexer{bulkIndexer: profilingExecutables, wg: &b.wg}
	return nil
}

func (b *bulkIndexers) shutdown(ctx context.Context) error {
	for _, bi := range b.modes {
		if bi == nil {
			continue
		}
		if err := bi.Close(ctx); err != nil {
			return err
		}
	}
	if b.profilingEvents != nil {
		if err := b.profilingEvents.Close(ctx); err != nil {
			return err
		}
	}
	if b.profilingStackTraces != nil {
		if err := b.profilingStackTraces.Close(ctx); err != nil {
			return err
		}
	}
	if b.profilingStackFrames != nil {
		if err := b.profilingStackFrames.Close(ctx); err != nil {
			return err
		}
	}
	if b.profilingExecutables != nil {
		if err := b.profilingExecutables.Close(ctx); err != nil {
			return err
		}
	}

	doneCh := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(doneCh)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
	}
	return nil
}

type wgTrackingBulkIndexer struct {
	bulkIndexer
	wg *sync.WaitGroup
}

func (w *wgTrackingBulkIndexer) StartSession(ctx context.Context) bulkIndexerSession {
	w.wg.Add(1)
	session := w.bulkIndexer.StartSession(ctx)
	return &wgTrackingBulkIndexerSession{bulkIndexerSession: session, wg: w.wg}
}

type wgTrackingBulkIndexerSession struct {
	bulkIndexerSession
	wg *sync.WaitGroup
}

func (w *wgTrackingBulkIndexerSession) End() {
	defer w.wg.Done()
	w.bulkIndexerSession.End()
}

type errBulkIndexerSession struct {
	err error
}

func (s errBulkIndexerSession) Add(context.Context, string, string, string, io.WriterTo, map[string]string, string) error {
	return fmt.Errorf("creating bulk indexer session failed, cannot add item: %w", s.err)
}

func (errBulkIndexerSession) End() {}

func (s errBulkIndexerSession) Flush(context.Context) error {
	return fmt.Errorf("creating bulk indexer session failed, cannot flush: %w", s.err)
}
