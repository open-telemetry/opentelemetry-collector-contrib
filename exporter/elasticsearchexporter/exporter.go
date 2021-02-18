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
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/cenkalti/backoff"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	esutil7 "github.com/elastic/go-elasticsearch/v7/esutil"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type esClientCurrent = elasticsearch7.Client
type esConfigCurrent = elasticsearch7.Config
type esBulkIndexerCurrent = esutil7.BulkIndexer

type elasticsearchExporter struct {
	logger *zap.Logger

	client      *esClientCurrent
	bulkIndexer esBulkIndexerCurrent
}

var retryOnStatus = []int{502, 503, 504, 429}

func newExporter(logger *zap.Logger, cfg *Config) (*elasticsearchExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newElasticsearchClient(logger, cfg)
	if err != nil {
		return nil, err
	}

	bulkIndexer, err := newBulkIndexer(client, cfg)
	if err != nil {
		return nil, err
	}

	return &elasticsearchExporter{
		logger:      logger,
		client:      client,
		bulkIndexer: bulkIndexer,
	}, nil
}

func (e *elasticsearchExporter) Shutdown(ctx context.Context) error {
	return e.bulkIndexer.Close(ctx)
}

func (e *elasticsearchExporter) pushLogsData(ctx context.Context, ld pdata.Logs) (dropped int, err error) {
	panic("TODO")
}

// clientLogger implements the estransport.Logger interface
// that is required by the Elasticsearch client for logging.
type clientLogger zap.Logger

// LogRoundTrip should not modify the request or response, except for consuming and closing the body.
// Implementations have to check for nil values in request and response.
func (cl *clientLogger) LogRoundTrip(_ *http.Request, resp *http.Response, err error, _ time.Time, dur time.Duration) error {
	zl := (*zap.Logger)(cl)
	switch {
	case err == nil && resp != nil:
		zl.Debug("Request roundtrip completed.", zap.Duration("duration", dur), zap.String("status", resp.Status))
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
		DisableRetry:         false,
		EnableRetryOnTimeout: true,
		MaxRetries:           config.Retry.MaxRequests,
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

func newBulkIndexer(client *elasticsearch7.Client, config *Config) (esBulkIndexerCurrent, error) {
	// TODO: add debug logger
	return esutil7.NewBulkIndexer(esutil7.BulkIndexerConfig{
		NumWorkers:    config.NumWorkers,
		FlushBytes:    config.Flush.Bytes,
		FlushInterval: config.Flush.Interval,
		Client:        client,
		Index:         config.Index,
		Pipeline:      config.Pipeline,
		Timeout:       config.Timeout,
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
