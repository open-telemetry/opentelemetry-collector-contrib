// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-elasticsearch/v7"
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
	Add(ctx context.Context, index string, document io.WriterTo) error

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

func newBulkIndexer(logger *zap.Logger, client *elasticsearch.Client, config *Config) (bulkIndexer, error) {
	// TODO: add support for synchronous bulk indexing, to integrate with the exporterhelper batch sender.
	return newAsyncBulkIndexer(logger, client, config)
}

func newAsyncBulkIndexer(logger *zap.Logger, client *elasticsearch.Client, config *Config) (*asyncBulkIndexer, error) {
	numWorkers := config.NumWorkers
	if numWorkers == 0 {
		numWorkers = runtime.NumCPU()
	}

	flushInterval := config.Flush.Interval
	if flushInterval == 0 {
		flushInterval = 30 * time.Second
	}

	flushBytes := config.Flush.Bytes
	if flushBytes == 0 {
		flushBytes = 5e+6
	}

	var maxDocRetry int
	if config.Retry.Enabled {
		// max_requests includes initial attempt
		// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32344
		maxDocRetry = config.Retry.MaxRequests - 1
	}

	pool := &asyncBulkIndexer{
		wg:    sync.WaitGroup{},
		items: make(chan docappender.BulkIndexerItem, config.NumWorkers),
		stats: bulkIndexerStats{},
	}
	pool.wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		bi, err := docappender.NewBulkIndexer(docappender.BulkIndexerConfig{
			Client:                client,
			MaxDocumentRetries:    maxDocRetry,
			Pipeline:              config.Pipeline,
			RetryOnDocumentStatus: config.Retry.RetryOnStatus,
		})
		if err != nil {
			return nil, err
		}
		w := asyncBulkIndexerWorker{
			indexer:       bi,
			items:         pool.items,
			flushInterval: flushInterval,
			flushTimeout:  config.Timeout,
			flushBytes:    flushBytes,
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
func (s asyncBulkIndexerSession) Add(ctx context.Context, index string, document io.WriterTo) error {
	item := docappender.BulkIndexerItem{
		Index: index,
		Body:  document,
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

			// w.indexer.Len() can be either compressed or uncompressed bytes
			if w.indexer.Len() >= w.flushBytes {
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
	if w.flushTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), w.flushTimeout)
		defer cancel()
	}
	stat, err := w.indexer.Flush(ctx)
	w.stats.docsIndexed.Add(stat.Indexed)
	if err != nil {
		w.logger.Error("bulk indexer flush error", zap.Error(err))
	}
	for _, resp := range stat.FailedDocs {
		w.logger.Error(fmt.Sprintf("Drop docs: failed to index: %#v", resp.Error),
			zap.Int("status", resp.Status))
	}
}
