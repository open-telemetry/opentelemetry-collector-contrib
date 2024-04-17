// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package elasticsearchexporter contains an opentelemetry-collector exporter
// for Elasticsearch.
package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/elastic/go-docappender"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

type esClientCurrent = elasticsearch7.Client
type esConfigCurrent = elasticsearch7.Config

type esBulkIndexerCurrent = BulkIndexerPool

type esBulkIndexerItem = docappender.BulkIndexerItem

// clientLogger implements the estransport.Logger interface
// that is required by the Elasticsearch client for logging.
type clientLogger zap.Logger

// LogRoundTrip should not modify the request or response, except for consuming and closing the body.
// Implementations have to check for nil values in request and response.
func (cl *clientLogger) LogRoundTrip(requ *http.Request, resp *http.Response, err error, _ time.Time, dur time.Duration) error {
	zl := (*zap.Logger)(cl)
	switch {
	case err == nil && resp != nil:
		zl.Debug("Request roundtrip completed.",
			zap.String("path", sanitize.String(requ.URL.Path)),
			zap.String("method", requ.Method),
			zap.Duration("duration", dur),
			zap.String("status", resp.Status))

	case err != nil:
		zl.Error("Request failed.", zap.NamedError("reason", err))
	}

	return nil
}

// RequestBodyEnabled makes the client pass a copy of request body to the logger.
func (*clientLogger) RequestBodyEnabled() bool {
	// TODO: introduce setting log the bodies for more detailed debug logs
	return false
}

// ResponseBodyEnabled makes the client pass a copy of response body to the logger.
func (*clientLogger) ResponseBodyEnabled() bool {
	// TODO: introduce setting log the bodies for more detailed debug logs
	return false
}

func newElasticsearchClient(logger *zap.Logger, config *Config) (*esClientCurrent, error) {
	tlsCfg, err := config.ClientConfig.LoadTLSConfig(context.Background())
	if err != nil {
		return nil, err
	}

	transport := newTransport(config, tlsCfg)

	headers := make(http.Header)
	for k, v := range config.Headers {
		headers.Add(k, v)
	}

	// TODO: validate settings:
	//  - try to parse address and validate scheme (address must be a valid URL)
	//  - check if cloud ID is valid

	// maxRetries configures the maximum number of event publishing attempts,
	// including the first send and additional retries.

	maxRetries := config.Retry.MaxRequests - 1
	retryDisabled := !config.Retry.Enabled || maxRetries <= 0

	if retryDisabled {
		maxRetries = 0
	}

	return elasticsearch7.NewClient(esConfigCurrent{
		Transport: transport,

		// configure connection setup
		Addresses: config.Endpoints,
		CloudID:   config.CloudID,
		Username:  config.Authentication.User,
		Password:  string(config.Authentication.Password),
		APIKey:    string(config.Authentication.APIKey),
		Header:    headers,

		// configure retry behavior
		RetryOnStatus:        retryOnStatus,
		DisableRetry:         retryDisabled,
		EnableRetryOnTimeout: config.Retry.Enabled,
		//RetryOnError:  retryOnError, // should be used from esclient version 8 onwards
		MaxRetries:   maxRetries,
		RetryBackoff: createElasticsearchBackoffFunc(&config.Retry),

		// configure sniffing
		DiscoverNodesOnStart:  config.Discovery.OnStart,
		DiscoverNodesInterval: config.Discovery.Interval,

		// configure internal metrics reporting and logging
		EnableMetrics:     false, // TODO
		EnableDebugLogger: false, // TODO
		Logger:            (*clientLogger)(logger),
	})
}

func newTransport(config *Config, tlsCfg *tls.Config) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if tlsCfg != nil {
		transport.TLSClientConfig = tlsCfg
	}
	if config.ReadBufferSize > 0 {
		transport.ReadBufferSize = config.ReadBufferSize
	}
	if config.WriteBufferSize > 0 {
		transport.WriteBufferSize = config.WriteBufferSize
	}

	return transport
}

func createElasticsearchBackoffFunc(config *RetrySettings) func(int) time.Duration {
	if !config.Enabled {
		return nil
	}

	expBackoff := backoff.NewExponentialBackOff()
	if config.InitialInterval > 0 {
		expBackoff.InitialInterval = config.InitialInterval
	}
	if config.MaxInterval > 0 {
		expBackoff.MaxInterval = config.MaxInterval
	}
	expBackoff.Reset()

	return func(attempts int) time.Duration {
		if attempts == 1 {
			expBackoff.Reset()
		}

		return expBackoff.NextBackOff()
	}
}

func pushDocuments(ctx context.Context, index string, document []byte, bulkIndexer *esBulkIndexerCurrent) error {
	return bulkIndexer.Add(ctx, index, bytes.NewReader(document))
}

func newBulkIndexer(logger *zap.Logger, client *elasticsearch7.Client, config *Config) (*esBulkIndexerCurrent, error) {
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
	group := &errgroup.Group{}
	items := make(chan esBulkIndexerItem, config.NumWorkers)
	stats := bulkIndexerStats{}

	for i := 0; i < numWorkers; i++ {
		w := worker{
			indexer:       docappender.NewBulkIndexer(client, 0, maxDocRetry),
			items:         items,
			flushInterval: flushInterval,
			flushTimeout:  config.Timeout,
			flushBytes:    flushBytes,
			logger:        logger,
			stats:         &stats,
		}
		group.Go(w.run)
	}
	return &BulkIndexerPool{
		items:    items,
		errgroup: group,
		stats:    &stats,
	}, nil
}

type bulkIndexerStats struct {
	docsIndexed atomic.Int64
}

type BulkIndexerPool struct {
	items    chan esBulkIndexerItem
	errgroup *errgroup.Group
	stats    *bulkIndexerStats
}

func (p *BulkIndexerPool) Add(ctx context.Context, index string, document io.WriterTo) error {
	item := esBulkIndexerItem{
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

func (p *BulkIndexerPool) Close(ctx context.Context) error {
	close(p.items)
	doneCh := make(chan struct{})
	go func() {
		p.errgroup.Wait()
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
	items         chan esBulkIndexerItem
	flushInterval time.Duration
	flushTimeout  time.Duration
	flushBytes    int

	stats *bulkIndexerStats

	logger *zap.Logger
}

func (w *worker) run() error {
	flushTick := time.NewTicker(w.flushInterval)
	for {
		select {
		case item := <-w.items:
			// check if BulkIndexer is closing
			zero := esBulkIndexerItem{}
			if item == zero {
				w.flush()
				return nil
			}

			w.indexer.Add(item)
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

func (w *worker) flush() error {
	ctx, cancel := context.WithTimeout(context.Background(), w.flushTimeout)
	defer cancel()
	stat, err := w.indexer.Flush(ctx)
	w.stats.docsIndexed.Add(stat.Indexed)
	if err != nil {
		w.logger.Error("bulk indexer flush error", zap.Error(err))
	}
	return err
}
