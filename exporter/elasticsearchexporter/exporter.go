// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package elasticsearchexporter contains an opentelemetry-collector exporter
// for Elasticsearch.
package elasticsearchexporter

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	esutil7 "github.com/elastic/go-elasticsearch/v7/esutil"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type esClientCurrent = elasticsearch7.Client
type esConfigCurrent = elasticsearch7.Config
type esBulkIndexerCurrent = esutil7.BulkIndexer
type esBulkIndexerItem = esutil7.BulkIndexerItem
type esBulkIndexerResponseItem = esutil7.BulkIndexerResponseItem

type elasticsearchExporter struct {
	logger *zap.Logger

	index       string
	maxAttempts int

	client      *esClientCurrent
	bulkIndexer esBulkIndexerCurrent
	model       mappingModel
}

var retryOnStatus = []int{500, 502, 503, 504, 429}

const createAction = "create"

func newExporter(logger *zap.Logger, cfg *Config) (*elasticsearchExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newElasticsearchClient(logger, cfg)
	if err != nil {
		return nil, err
	}

	bulkIndexer, err := newBulkIndexer(logger, client, cfg)
	if err != nil {
		return nil, err
	}

	maxAttempts := 1
	if cfg.Retry.Enabled {
		maxAttempts = cfg.Retry.MaxRequests
	}

	// TODO: Apply encoding and field mapping settings.
	model := &encodeModel{dedup: true, dedot: false}

	return &elasticsearchExporter{
		logger:      logger,
		client:      client,
		bulkIndexer: bulkIndexer,

		index:       cfg.Index,
		maxAttempts: maxAttempts,
		model:       model,
	}, nil
}

func (e *elasticsearchExporter) Shutdown(ctx context.Context) error {
	return e.bulkIndexer.Close(ctx)
}

func (e *elasticsearchExporter) pushLogsData(ctx context.Context, ld pdata.Logs) error {
	var errs []error

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()
		ills := rl.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(i).Logs()
			for k := 0; k < logs.Len(); k++ {
				if err := e.pushLogRecord(ctx, resource, logs.At(k)); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					errs = append(errs, err)
				}
			}
		}
	}

	return multierr.Combine(errs...)
}

func (e *elasticsearchExporter) pushLogRecord(ctx context.Context, resource pdata.Resource, record pdata.LogRecord) error {
	document, err := e.model.encodeLog(resource, record)
	if err != nil {
		return fmt.Errorf("Failed to encode log event: %w", err)
	}
	return e.pushEvent(ctx, document)
}

func (e *elasticsearchExporter) pushEvent(ctx context.Context, document []byte) error {
	attempts := 1
	body := bytes.NewReader(document)
	item := esBulkIndexerItem{Action: createAction, Index: e.index, Body: body}

	// Setup error handler. The handler handles the per item response status based on the
	// selective ACKing in the bulk response.
	item.OnFailure = func(ctx context.Context, item esBulkIndexerItem, resp esBulkIndexerResponseItem, err error) {
		switch {
		case attempts < e.maxAttempts && shouldRetryEvent(resp.Status):
			e.logger.Debug("Retrying to index event",
				zap.Int("attempt", attempts),
				zap.Int("status", resp.Status),
				zap.NamedError("reason", err))

			attempts++
			body.Seek(0, io.SeekStart)
			e.bulkIndexer.Add(ctx, item)

		case resp.Status == 0 && err != nil:
			// Encoding error. We didn't even attempt to send the event
			e.logger.Error("Drop event: failed to add event to the bulk request buffer.",
				zap.NamedError("reason", err))

		case err != nil:
			e.logger.Error("Drop event: failed to index event",
				zap.Int("attempt", attempts),
				zap.Int("status", resp.Status),
				zap.NamedError("reason", err))

		default:
			e.logger.Error(fmt.Sprintf("Drop event: failed to index event: %#v", resp.Error),
				zap.Int("attempt", attempts),
				zap.Int("status", resp.Status))
		}
	}

	return e.bulkIndexer.Add(ctx, item)
}

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
			zap.String("path", requ.URL.Path),
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
	tlsCfg, err := config.TLSClientSetting.LoadTLSConfig()
	if err != nil {
		return nil, err
	}

	transport := newTransport(config, tlsCfg)

	var headers http.Header
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
		Password:  config.Authentication.Password,
		APIKey:    config.Authentication.APIKey,
		Header:    headers,

		// configure retry behavior
		RetryOnStatus:        retryOnStatus,
		DisableRetry:         retryDisabled,
		EnableRetryOnTimeout: config.Retry.Enabled,
		MaxRetries:           maxRetries,
		RetryBackoff:         createElasticsearchBackoffFunc(&config.Retry),

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

func newBulkIndexer(logger *zap.Logger, client *elasticsearch7.Client, config *Config) (esBulkIndexerCurrent, error) {
	// TODO: add debug logger
	return esutil7.NewBulkIndexer(esutil7.BulkIndexerConfig{
		NumWorkers:    config.NumWorkers,
		FlushBytes:    config.Flush.Bytes,
		FlushInterval: config.Flush.Interval,
		Client:        client,
		Pipeline:      config.Pipeline,
		Timeout:       config.Timeout,

		OnError: func(_ context.Context, err error) {
			logger.Error(fmt.Sprintf("Bulk indexer error: %v", err))
		},
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

func shouldRetryEvent(status int) bool {
	for _, retryable := range retryOnStatus {
		if status == retryable {
			return true
		}
	}
	return false
}
