// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/exporter"
	"go.uber.org/zap"
)

type bulkIndexer interface {
	// StartSession starts a new bulk indexing session.
	StartSession(context.Context) (bulkIndexerSession, error)

	// Close closes the bulk indexer, ending any in-progress
	// sessions and stopping any background processing.
	Close(ctx context.Context) error
}

type bulkIndexerSession interface {
	// Add adds a document to the bulk indexing session.
	Add(ctx context.Context, index string, docID string, pipeline string, document io.WriterTo, dynamicTemplates map[string]string, action string) error

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

func newBulkIndexer(logger *zap.Logger, client esapi.Transport, config *Config, requireDataStream bool) (bulkIndexer, error) {
	if config.Batcher.enabledSet {
		return newSyncBulkIndexer(logger, client, config, requireDataStream), nil
	}
	return newAsyncBulkIndexer(logger, client, config, requireDataStream)
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
		Client:                client,
		MaxDocumentRetries:    maxDocRetries,
		Pipeline:              config.Pipeline,
		RetryOnDocumentStatus: config.Retry.RetryOnStatus,
		RequireDataStream:     requireDataStream,
		CompressionLevel:      compressionLevel,
	}
}

func newSyncBulkIndexer(logger *zap.Logger, client esapi.Transport, config *Config, requireDataStream bool) *syncBulkIndexer {
	return &syncBulkIndexer{
		config:       bulkIndexerConfig(client, config, requireDataStream),
		flushTimeout: config.Timeout,
		flushBytes:   config.Flush.Bytes,
		retryConfig:  config.Retry,
		logger:       logger,
	}
}

type syncBulkIndexer struct {
	config       docappender.BulkIndexerConfig
	flushTimeout time.Duration
	flushBytes   int
	retryConfig  RetrySettings
	logger       *zap.Logger
}

// StartSession creates a new docappender.BulkIndexer, and wraps
// it with a syncBulkIndexerSession.
func (s *syncBulkIndexer) StartSession(context.Context) (bulkIndexerSession, error) {
	bi, err := docappender.NewBulkIndexer(s.config)
	if err != nil {
		return nil, err
	}
	return &syncBulkIndexerSession{
		s:  s,
		bi: bi,
	}, nil
}

// Close is a no-op.
func (s *syncBulkIndexer) Close(context.Context) error {
	return nil
}

type syncBulkIndexerSession struct {
	s  *syncBulkIndexer
	bi *docappender.BulkIndexer
}

// Add adds an item to the sync bulk indexer session.
func (s *syncBulkIndexerSession) Add(ctx context.Context, index string, docID string, pipeline string, document io.WriterTo, dynamicTemplates map[string]string, action string) error {
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
	// flush bytes should operate on uncompressed length
	// as Elasticsearch http.max_content_length measures uncompressed length.
	if s.bi.UncompressedLen() >= s.s.flushBytes {
		return s.Flush(ctx)
	}
	return nil
}

// End is a no-op.
func (s *syncBulkIndexerSession) End() {
	// TODO acquire docappender.BulkIndexer from pool in StartSession, release here
}

// Flush flushes documents added to the bulk indexer session.
func (s *syncBulkIndexerSession) Flush(ctx context.Context) error {
	var retryBackoff func(int) time.Duration
	for attempts := 0; ; attempts++ {
		if _, err := flushBulkIndexer(ctx, s.bi, s.s.flushTimeout, s.s.logger); err != nil {
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

func newAsyncBulkIndexer(logger *zap.Logger, client esapi.Transport, config *Config, requireDataStream bool) (*asyncBulkIndexer, error) {
	numWorkers := config.NumWorkers
	if numWorkers == 0 {
		numWorkers = runtime.NumCPU()
	}

	pool := &asyncBulkIndexer{
		wg:    sync.WaitGroup{},
		items: make(chan docappender.BulkIndexerItem, config.NumWorkers),
		stats: bulkIndexerStats{},
	}
	pool.wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		bi, err := docappender.NewBulkIndexer(bulkIndexerConfig(client, config, requireDataStream))
		if err != nil {
			return nil, err
		}
		w := asyncBulkIndexerWorker{
			indexer:       bi,
			items:         pool.items,
			flushInterval: config.Flush.Interval,
			flushTimeout:  config.Timeout,
			flushBytes:    config.Flush.Bytes,
			logger:        logger,
			stats:         &pool.stats,
		}
		go func() {
			defer pool.wg.Done()
			w.run()
		}()
	}
	return pool, nil
}

type bulkIndexerStats struct {
	docsIndexed atomic.Int64
}

type asyncBulkIndexer struct {
	items chan docappender.BulkIndexerItem
	wg    sync.WaitGroup
	stats bulkIndexerStats
}

type asyncBulkIndexerSession struct {
	*asyncBulkIndexer
}

// StartSession returns a new asyncBulkIndexerSession.
func (a *asyncBulkIndexer) StartSession(context.Context) (bulkIndexerSession, error) {
	return asyncBulkIndexerSession{a}, nil
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
func (s asyncBulkIndexerSession) Add(ctx context.Context, index string, docID string, pipeline string, document io.WriterTo, dynamicTemplates map[string]string, action string) error {
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
		return nil
	}
}

// End is a no-op.
func (s asyncBulkIndexerSession) End() {
}

// Flush is a no-op.
func (s asyncBulkIndexerSession) Flush(context.Context) error {
	return nil
}

type asyncBulkIndexerWorker struct {
	indexer       *docappender.BulkIndexer
	items         <-chan docappender.BulkIndexerItem
	flushInterval time.Duration
	flushTimeout  time.Duration
	flushBytes    int

	stats *bulkIndexerStats

	logger *zap.Logger
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
	ctx := context.Background()
	stat, _ := flushBulkIndexer(ctx, w.indexer, w.flushTimeout, w.logger)
	w.stats.docsIndexed.Add(stat.Indexed)
}

func flushBulkIndexer(
	ctx context.Context,
	bi *docappender.BulkIndexer,
	timeout time.Duration,
	logger *zap.Logger,
) (docappender.BulkIndexerResponseStat, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	stat, err := bi.Flush(ctx)
	if err != nil {
		logger.Error("bulk indexer flush error", zap.Error(err))
	}
	for _, resp := range stat.FailedDocs {
		fields := []zap.Field{
			zap.String("index", resp.Index),
			zap.String("error.type", resp.Error.Type),
			zap.String("error.reason", resp.Error.Reason),
		}
		if hint := getErrorHint(resp.Index, resp.Error.Type); hint != "" {
			fields = append(fields, zap.String("hint", hint))
		}
		logger.Error("failed to index document", fields...)
	}
	return stat, err
}

func getErrorHint(index, errorType string) string {
	if strings.HasPrefix(index, ".ds-metrics-") && errorType == "version_conflict_engine_exception" {
		return "check the \"Known issues\" section of Elasticsearch Exporter docs"
	}
	return ""
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
		bi, err = newBulkIndexer(set.TelemetrySettings.Logger, esClient, cfg, mode == MappingOTel)
		if err != nil {
			return err
		}
		b.modes[mode] = &wgTrackingBulkIndexer{bulkIndexer: bi, wg: &b.wg}
	}

	profilingEvents, err := newBulkIndexer(set.Logger, esClient, cfg, true)
	if err != nil {
		return err
	}
	b.profilingEvents = &wgTrackingBulkIndexer{bulkIndexer: profilingEvents, wg: &b.wg}

	profilingStackTraces, err := newBulkIndexer(set.Logger, esClient, cfg, false)
	if err != nil {
		return err
	}
	b.profilingStackTraces = &wgTrackingBulkIndexer{bulkIndexer: profilingStackTraces, wg: &b.wg}

	profilingStackFrames, err := newBulkIndexer(set.Logger, esClient, cfg, false)
	if err != nil {
		return err
	}
	b.profilingStackFrames = &wgTrackingBulkIndexer{bulkIndexer: profilingStackFrames, wg: &b.wg}

	profilingExecutables, err := newBulkIndexer(set.Logger, esClient, cfg, false)
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

func (w *wgTrackingBulkIndexer) StartSession(ctx context.Context) (bulkIndexerSession, error) {
	w.wg.Add(1)
	session, err := w.bulkIndexer.StartSession(ctx)
	if err != nil {
		w.wg.Done()
		return nil, err
	}
	return &wgTrackingBulkIndexerSession{bulkIndexerSession: session, wg: w.wg}, nil
}

type wgTrackingBulkIndexerSession struct {
	bulkIndexerSession
	wg *sync.WaitGroup
}

func (w *wgTrackingBulkIndexerSession) End() {
	defer w.wg.Done()
	w.bulkIndexerSession.End()
}
