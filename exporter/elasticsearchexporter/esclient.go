// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	elastictransportversion "github.com/elastic/elastic-transport-go/v8/elastictransport/version"
	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

// clientLogger implements the estransport.Logger interface
// that is required by the Elasticsearch client for logging.
type clientLogger struct {
	*zap.Logger
	logRequestBody  bool
	logResponseBody bool
	componentHost   component.Host
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
		if resp.StatusCode == http.StatusOK {
			// Success
			componentstatus.ReportStatus(
				cl.componentHost, componentstatus.NewEvent(componentstatus.StatusOK))
		} else if httpRecoverableErrorStatus(resp.StatusCode) {
			err := fmt.Errorf("Elasticsearch request failed: %v", resp.Status)
			componentstatus.ReportStatus(
				cl.componentHost, componentstatus.NewRecoverableErrorEvent(err))
		}

	case clientErr != nil:
		fields = append(
			fields,
			zap.NamedError("reason", clientErr),
		)
		zl.Debug("Request failed.", fields...)
		err := fmt.Errorf("Elasticsearch request failed: %w", clientErr)
		componentstatus.ReportStatus(
			cl.componentHost, componentstatus.NewRecoverableErrorEvent(err))
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

const (
	unknownProduct = "the client noticed that the server is not Elasticsearch and we do not support this unknown product"
	defaultURL     = "http://localhost:9200"
)

// genuineCheckHeader validates the presence of the X-Elastic-Product header
func genuineCheckHeader(header http.Header) error {
	if header.Get("X-Elastic-Product") != "Elasticsearch" {
		return errors.New(unknownProduct)
	}
	return nil
}

type esClient struct {
	transport           elastictransport.Interface
	productCheckSuccess atomic.Bool
}

func (e *esClient) Perform(req *http.Request) (*http.Response, error) {
	res, err := e.transport.Perform(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		checkHeader := func() error { return genuineCheckHeader(res.Header) }
		if err := e.doProductCheck(checkHeader); err != nil {
			res.Body.Close()
			return nil, err
		}
	}
	return res, nil
}

func (e *esClient) doProductCheck(f func() error) error {
	if e.productCheckSuccess.Load() {
		return nil
	}
	if err := f(); err != nil {
		return err
	}
	e.productCheckSuccess.Store(true)
	return nil
}

// newElasticsearchClient returns a new elastictransport.Interface.
func newElasticsearchClient(
	ctx context.Context,
	config *Config,
	host component.Host,
	telemetry component.TelemetrySettings,
	userAgent string,
) (elastictransport.Interface, error) {
	httpClient, err := config.ToClient(ctx, host.GetExtensions(), telemetry)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header)

	// endpoints converts Config.Endpoints, Config.CloudID,
	// and Config.ClientConfig.Endpoint to a list of addresses.
	endpoints, err := config.endpoints()
	if err != nil {
		return nil, err
	}

	esLogger := &clientLogger{
		Logger:          telemetry.Logger,
		logRequestBody:  config.LogRequestBody,
		logResponseBody: config.LogResponseBody,
		componentHost:   host,
	}

	maxRetries := defaultMaxRetries
	if config.Retry.MaxRetries != 0 {
		maxRetries = config.Retry.MaxRetries
	}

	// Convert addresses to URLs
	urls, err := addrsToURLs(endpoints)
	if err != nil {
		return nil, fmt.Errorf("cannot create client: %w", err)
	}

	if len(urls) == 0 {
		u, _ := url.Parse(defaultURL)
		urls = append(urls, u)
	}

	username := config.Authentication.User
	password := string(config.Authentication.Password)
	if user := urls[0].User; user != nil {
		username = user.Username()
		password, _ = user.Password()
	}

	// Create transport configuration matching elasticsearch.newTransport structure
	tpConfig := elastictransport.Config{
		UserAgent: userAgent,

		URLs:     urls,
		Username: username,
		Password: password,
		APIKey:   string(config.Authentication.APIKey),

		Header: headers,

		RetryOnStatus: config.Retry.RetryOnStatus,
		DisableRetry:  !config.Retry.Enabled,
		RetryOnError: func(_ *http.Request, err error) bool {
			return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
		},
		MaxRetries:   maxRetries,
		RetryBackoff: createElasticsearchBackoffFunc(&config.Retry),

		EnableMetrics:     false, // TODO
		EnableDebugLogger: false, // TODO

		DiscoverNodesInterval: config.Discovery.Interval,

		Transport: httpClient.Transport,
		Logger:    esLogger,
		Instrumentation: elastictransport.NewOtelInstrumentation(
			telemetry.TracerProvider,
			false, /* captureSearchBody */
			elastictransportversion.Version,
		),
	}

	tp, err := elastictransport.New(tpConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating transport: %w", err)
	}

	// Handle node discovery on start, matching elasticsearch.NewClient behavior
	if config.Discovery.OnStart {
		go func() {
			_ = tp.DiscoverNodesContext(ctx)
		}()
	}

	return &esClient{transport: tp}, nil
}

// addrsToURLs creates a list of url.URL structures from url list.
func addrsToURLs(addrs []string) ([]*url.URL, error) {
	var urls []*url.URL
	for _, addr := range addrs {
		u, err := url.Parse(strings.TrimRight(addr, "/"))
		if err != nil {
			return nil, fmt.Errorf("cannot parse url: %w", err)
		}

		urls = append(urls, u)
	}
	return urls, nil
}

func createElasticsearchBackoffFunc(config *RetrySettings) func(int) time.Duration {
	if !config.Enabled {
		return nil
	}

	return func(attempts int) time.Duration {
		next := min(config.MaxInterval, config.InitialInterval*(1<<(attempts-1)))
		nextWithJitter := next/2 + time.Duration(rand.Float64()*float64(next/2))
		return nextWithJitter
	}
}

func httpRecoverableErrorStatus(statusCode int) bool {
	// Elasticsearch uses 409 conflict to report duplicates, which aren't really
	// an error state, so those return false (but if we were already in an error
	// state, we will still wait until we get an actual 200 OK before changing
	// our state back).
	return statusCode >= 300 && statusCode != http.StatusConflict
}
