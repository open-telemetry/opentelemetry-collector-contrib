// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package elasticsearchexporter contains an opentelemetry-collector exporter
// for Elasticsearch.
package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/elastic/go-docappender/v2"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

type esClientCurrent = elasticsearch7.Client
type esConfigCurrent = elasticsearch7.Config

type esBulkIndexerCurrent = bulkIndexerPool

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
		RetryOnStatus:        config.Retry.RetryOnStatus,
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

	pool := &bulkIndexerPool{
		closeCh:   make(chan struct{}),
		stats:     bulkIndexerStats{},
		available: make(chan *worker, numWorkers),
	}

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
			closeCh:       pool.closeCh,
			flushInterval: flushInterval,
			flushTimeout:  config.Timeout,
			retryBackoff:  createElasticsearchBackoffFunc(&config.Retry),
			logger:        logger,
			stats:         &pool.stats,
		}
		pool.available <- &w
	}
	return pool, nil
}

type bulkIndexerStats struct {
	docsIndexed atomic.Int64
}

type bulkIndexerPool struct {
	closeCh   chan struct{}
	stats     bulkIndexerStats
	available chan *worker
}

func (p *bulkIndexerPool) AddBatchAndFlush(ctx context.Context, batch []esBulkIndexerItem) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.closeCh:
		return fmt.Errorf("bulk indexer is closed")
	case worker := <-p.available:
		defer func() {
			p.available <- worker
		}()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.closeCh:
			return fmt.Errorf("bulk indexer is closed")
		default:
		}
		return worker.addBatchAndFlush(ctx, batch)
	}
}

// Close closes the closeCh channel and wait for workers to finish.
func (p *bulkIndexerPool) Close(ctx context.Context) error {
	close(p.closeCh)
	doneCh := make(chan struct{})
	go func() {
		for i := 0; i < cap(p.available); i++ {
			<-p.available
		}
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
	closeCh       <-chan struct{}
	flushInterval time.Duration
	flushTimeout  time.Duration
	//flushBytes    int
	mu sync.Mutex

	retryBackoff func(int) time.Duration

	stats *bulkIndexerStats

	logger *zap.Logger
}

func (w *worker) addBatchAndFlush(ctx context.Context, batch []esBulkIndexerItem) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, item := range batch {
		if err := w.indexer.Add(item); err != nil {
			w.logger.Error("error adding item to bulk indexer", zap.Error(err))
		}
	}
	for attempts := 0; ; attempts++ {
		if err := w.flush(); err != nil {
			return err
		} else if w.indexer.Items() == 0 {
			return nil
		}
		backoff := w.retryBackoff(attempts + 1)
		timer := time.NewTimer(backoff)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.closeCh:
			return fmt.Errorf("bulk indexer is closed")
		case <-timer.C:
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
	for _, resp := range stat.FailedDocs {
		w.logger.Error(fmt.Sprintf("Drop docs: failed to index: %#v", resp.Error),
			zap.Int("status", resp.Status))
	}
	return err
}
