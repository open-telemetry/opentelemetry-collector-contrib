// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/elastic/go-docappender/v2"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

type esClientCurrent = elasticsearch7.Client
type esConfigCurrent = elasticsearch7.Config

type esBulkIndexerCurrent = bulkIndexerManager

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

func newElasticsearchClient(
	ctx context.Context,
	config *Config,
	host component.Host,
	telemetry component.TelemetrySettings,
	userAgent string,
) (*esClientCurrent, error) {
	httpClient, err := config.ClientConfig.ToClient(ctx, host, telemetry)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header)
	headers.Set("User-Agent", userAgent)

	// maxRetries configures the maximum number of event publishing attempts,
	// including the first send and additional retries.

	maxRetries := config.Retry.MaxRequests - 1
	retryDisabled := !config.Retry.Enabled || maxRetries <= 0

	if retryDisabled {
		maxRetries = 0
	}

	// endpoints converts Config.Endpoints, Config.CloudID,
	// and Config.ClientConfig.Endpoint to a list of addresses.
	endpoints, err := config.endpoints()
	if err != nil {
		return nil, err
	}

	return elasticsearch7.NewClient(esConfigCurrent{
		Transport: httpClient.Transport,

		// configure connection setup
		Addresses: endpoints,
		Username:  config.Authentication.User,
		Password:  string(config.Authentication.Password),
		APIKey:    string(config.Authentication.APIKey),
		Header:    headers,

		// configure retry behavior
		RetryOnStatus:        config.Retry.RetryOnStatus,
		DisableRetry:         retryDisabled,
		EnableRetryOnTimeout: config.Retry.Enabled, // for timeouts in underlying transport layers
		//RetryOnError:  retryOnError, // should be used from esclient version 8 onwards
		MaxRetries:   maxRetries,
		RetryBackoff: createElasticsearchBackoffFunc(&config.Retry),

		// configure sniffing
		DiscoverNodesOnStart:  config.Discovery.OnStart,
		DiscoverNodesInterval: config.Discovery.Interval,

		// configure internal metrics reporting and logging
		EnableMetrics:     false, // TODO
		EnableDebugLogger: false, // TODO
		Logger:            (*clientLogger)(telemetry.Logger),
	})
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

func newBulkIndexer(logger *zap.Logger, client *esClientCurrent, config *Config) (*esBulkIndexerCurrent, error) {
	return &bulkIndexerManager{
		closeCh: make(chan struct{}),
		stats:   bulkIndexerStats{},
		logger:  logger,
		config:  config,
		wg:      &sync.WaitGroup{},
		sem:     semaphore.NewWeighted(int64(config.NumWorkers)),
		client:  client,
	}, nil
}

type bulkIndexerStats struct {
	docsIndexed atomic.Int64
}

type bulkIndexerManager struct {
	closeCh chan struct{}
	stats   bulkIndexerStats
	logger  *zap.Logger
	config  *Config
	wg      *sync.WaitGroup
	sem     *semaphore.Weighted
	pool    *sync.Pool
	client  *esClientCurrent
}

func (p *bulkIndexerManager) AddBatchAndFlush(ctx context.Context, batch []esBulkIndexerItem) error {
	p.wg.Add(1)
	defer p.wg.Done()

	if err := p.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	defer p.sem.Release(1)

	var maxDocRetry int
	if p.config.Retry.Enabled {
		// max_requests includes initial attempt
		// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32344
		maxDocRetry = p.config.Retry.MaxRequests - 1
	}

	bi, err := docappender.NewBulkIndexer(docappender.BulkIndexerConfig{
		Client:                p.client,
		MaxDocumentRetries:    maxDocRetry,
		Pipeline:              p.config.Pipeline,
		RetryOnDocumentStatus: p.config.Retry.RetryOnStatus,
	})
	if err != nil {
		return fmt.Errorf("error creating docappender bulk indexer: %w", err)
	}

	w := worker{
		indexer:      bi,
		closeCh:      p.closeCh,
		flushTimeout: p.config.Timeout,
		retryBackoff: createElasticsearchBackoffFunc(&p.config.Retry),
		logger:       p.logger,
		stats:        &p.stats,
	}
	return w.addBatchAndFlush(ctx, batch)
}

// Close closes the closeCh channel and wait for all p.AddBatchAndFlush to finish.
func (p *bulkIndexerManager) Close(ctx context.Context) error {
	close(p.closeCh)
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
	indexer *docappender.BulkIndexer
	closeCh <-chan struct{}

	// timeout on a single bulk request, not to be confused with `batcher.flush_timeout` option
	flushTimeout time.Duration

	retryBackoff func(int) time.Duration

	stats *bulkIndexerStats

	logger *zap.Logger
}

func (w *worker) addBatchAndFlush(ctx context.Context, batch []esBulkIndexerItem) error {
	for _, item := range batch {
		if err := w.indexer.Add(item); err != nil {
			return fmt.Errorf("failed to add item to bulk indexer: %w", err)
		}
	}
	for attempts := 0; ; attempts++ {
		if err := w.flush(ctx); err != nil {
			return err
		}
		if w.indexer.Items() == 0 {
			// No documents in buffer waiting for per-document retry, exit retry loop.
			return nil
		}
		if w.retryBackoff == nil {
			// BUG: This should never happen in practice.
			// When retry is disabled / document level retry limit is reached,
			// documents should go into FailedDocs instead of indexer buffer.
			return errors.New("bulk indexer contains documents pending retry but retry is disabled")
		}
		backoff := w.retryBackoff(attempts + 1) // TODO: use exporterhelper retry_sender
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-w.closeCh:
			timer.Stop()
			return errors.New("bulk indexer is closed")
		case <-timer.C:
		}
	}
}

func (w *worker) flush(ctx context.Context) error {
	if w.flushTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.flushTimeout)
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
	return err
}
