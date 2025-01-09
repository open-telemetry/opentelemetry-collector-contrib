// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-elasticsearch/v7"
	"go.opentelemetry.io/collector/config/configcompression"
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
	Add(ctx context.Context, index string, docID string, document io.WriterTo, dynamicTemplates map[string]string) error

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

func newBulkIndexer(logger *zap.Logger, client *elasticsearch.Client, config *Config) (bulkIndexer, error) {
	if config.Batcher.Enabled != nil {
		return newSyncBulkIndexer(logger, client, config), nil
	}
	return newAsyncBulkIndexer(logger, client, config)
}

func bulkIndexerConfig(client *elasticsearch.Client, config *Config) docappender.BulkIndexerConfig {
	var maxDocRetries int
	if config.Retry.Enabled {
		maxDocRetries = defaultMaxRetries
		if config.Retry.MaxRetries != 0 {
			maxDocRetries = config.Retry.MaxRetries
		}
	}
	var compressionLevel int
	if config.Compression == configcompression.TypeGzip {
		compressionLevel = gzip.BestSpeed
	}
	return docappender.BulkIndexerConfig{
		Client:                client,
		MaxDocumentRetries:    maxDocRetries,
		Pipeline:              config.Pipeline,
		RetryOnDocumentStatus: config.Retry.RetryOnStatus,
		RequireDataStream:     config.MappingMode() == MappingOTel,
		CompressionLevel:      compressionLevel,
	}
}

func newSyncBulkIndexer(logger *zap.Logger, client *elasticsearch.Client, config *Config) *syncBulkIndexer {
	return &syncBulkIndexer{
		config:       bulkIndexerConfig(client, config),
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
func (s *syncBulkIndexerSession) Add(ctx context.Context, index string, docID string, document io.WriterTo, dynamicTemplates map[string]string) error {
	doc := docappender.BulkIndexerItem{Index: index, Body: document, DynamicTemplates: dynamicTemplates}
	if docID != "" {
		doc.DocumentID = docID
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

func newAsyncBulkIndexer(logger *zap.Logger, client *elasticsearch.Client, config *Config) (*asyncBulkIndexer, error) {
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
		bi, err := docappender.NewBulkIndexer(bulkIndexerConfig(client, config))
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
func (s asyncBulkIndexerSession) Add(ctx context.Context, index string, docID string, document io.WriterTo, dynamicTemplates map[string]string) error {
	item := docappender.BulkIndexerItem{
		Index:            index,
		Body:             document,
		DynamicTemplates: dynamicTemplates,
	}
	if docID != "" {
		item.DocumentID = docID
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
