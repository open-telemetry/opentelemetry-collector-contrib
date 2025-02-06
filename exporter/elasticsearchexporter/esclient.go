// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

// clientLogger implements the estransport.Logger interface
// that is required by the Elasticsearch client for logging.
type clientLogger struct {
	*zap.Logger
	logRequestBody  bool
	logResponseBody bool
}

// LogRoundTrip should not modify the request or response, except for consuming and closing the body.
// Implementations have to check for nil values in request and response.
func (cl *clientLogger) LogRoundTrip(requ *http.Request, resp *http.Response, clientErr error, _ time.Time, dur time.Duration) error {
	zl := cl.Logger

	var fields []zap.Field
	if cl.logRequestBody && requ != nil && requ.Body != nil {
		body := requ.Body
		if requ.Header.Get("Content-Encoding") == "gzip" {
			if r, err := gzip.NewReader(body); err == nil {
				defer r.Close()
				body = r
			}
		}
		if b, err := io.ReadAll(body); err == nil {
			fields = append(fields, zap.ByteString("request_body", b))
		}
	}
	if cl.logResponseBody && resp != nil && resp.Body != nil {
		if b, err := io.ReadAll(resp.Body); err == nil {
			fields = append(fields, zap.ByteString("response_body", b))
		}
	}

	switch {
	case clientErr == nil && resp != nil:
		fields = append(
			fields,
			zap.String("path", sanitize.String(requ.URL.Path)),
			zap.String("method", requ.Method),
			zap.Duration("duration", dur),
			zap.String("status", resp.Status),
		)
		zl.Debug("Request roundtrip completed.", fields...)

	case clientErr != nil:
		fields = append(
			fields,
			zap.NamedError("reason", clientErr),
		)
		zl.Debug("Request failed.", fields...)
	}

	return nil
}

// RequestBodyEnabled makes the client pass a copy of request body to the logger.
func (cl *clientLogger) RequestBodyEnabled() bool {
	return cl.logRequestBody
}

// ResponseBodyEnabled makes the client pass a copy of response body to the logger.
func (cl *clientLogger) ResponseBodyEnabled() bool {
	return cl.logResponseBody
}

// newElasticsearchClient returns a new elasticsearch.Client
func newElasticsearchClient(
	ctx context.Context,
	config *Config,
	host component.Host,
	telemetry component.TelemetrySettings,
	userAgent string,
) (*elasticsearch.Client, error) {
	httpClient, err := config.ClientConfig.ToClient(ctx, host, telemetry)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header)
	headers.Set("User-Agent", userAgent)

	// endpoints converts Config.Endpoints, Config.CloudID,
	// and Config.ClientConfig.Endpoint to a list of addresses.
	endpoints, err := config.endpoints()
	if err != nil {
		return nil, err
	}

	esLogger := clientLogger{
		Logger:          telemetry.Logger,
		logRequestBody:  config.LogRequestBody,
		logResponseBody: config.LogResponseBody,
	}

	maxRetries := defaultMaxRetries
	if config.Retry.MaxRetries != 0 {
		maxRetries = config.Retry.MaxRetries
	}

	return elasticsearch.NewClient(elasticsearch.Config{
		Transport: httpClient.Transport,

		// configure connection setup
		Addresses: endpoints,
		Username:  config.Authentication.User,
		Password:  string(config.Authentication.Password),
		APIKey:    string(config.Authentication.APIKey),
		Header:    headers,

		// configure retry behavior
		RetryOnStatus:        config.Retry.RetryOnStatus,
		DisableRetry:         !config.Retry.Enabled,
		EnableRetryOnTimeout: config.Retry.Enabled,
		// RetryOnError:  retryOnError, // should be used from esclient version 8 onwards
		MaxRetries:   maxRetries,
		RetryBackoff: createElasticsearchBackoffFunc(&config.Retry),

		// configure sniffing
		DiscoverNodesOnStart:  config.Discovery.OnStart,
		DiscoverNodesInterval: config.Discovery.Interval,

		// configure internal metrics reporting and logging
		EnableMetrics:     false, // TODO
		EnableDebugLogger: false, // TODO
		Logger:            &esLogger,
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
