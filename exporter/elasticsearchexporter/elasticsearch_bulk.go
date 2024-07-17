// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
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

func pushDocuments(ctx context.Context, index string, document []byte, bulkIndexer *bulkIndexerPool) error {
	return bulkIndexer.Add(ctx, index, bytes.NewReader(document))
}

func newBulkIndexer(logger *zap.Logger, client *elasticsearch.Client, config *Config) (*bulkIndexerPool, error) {
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

	pool := &bulkIndexerPool{
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
		w := worker{
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

type bulkIndexerPool struct {
	items chan docappender.BulkIndexerItem
	wg    sync.WaitGroup
	stats bulkIndexerStats
}

// Add adds an item to the bulk indexer pool.
//
// Adding an item after a call to Close() will panic.
func (p *bulkIndexerPool) Add(ctx context.Context, index string, document io.WriterTo) error {
	item := docappender.BulkIndexerItem{
		Index: index,
		Body:  document,
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.items <- item:
		return nil
	}
}

// Close closes the items channel and waits for the workers to drain it.
func (p *bulkIndexerPool) Close(ctx context.Context) error {
	close(p.items)
	doneCh := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(doneCh)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
		return nil
	}
}

type worker struct {
	indexer       *docappender.BulkIndexer
	items         <-chan docappender.BulkIndexerItem
	flushInterval time.Duration
	flushTimeout  time.Duration
	flushBytes    int

	stats *bulkIndexerStats

	logger *zap.Logger
}

func (w *worker) run() {
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

func (w *worker) flush() {
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
