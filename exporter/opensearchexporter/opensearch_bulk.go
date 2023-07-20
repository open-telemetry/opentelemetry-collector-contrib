// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package opensearchexporter contains an opentelemetry-collector exporter
// for OpenSearch.
package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchutil"
	"go.uber.org/zap"
)

type osClientCurrent = opensearch.Client
type osConfigCurrent = opensearch.Config
type osBulkIndexerCurrent = opensearchutil.BulkIndexer
type osBulkIndexerItem = opensearchutil.BulkIndexerItem
type osBulkIndexerResponseItem = opensearchutil.BulkIndexerResponseItem

var retryOnStatus = []int{500, 502, 503, 504, 429}

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

func newOpenSearchClient(endpoint string, httpClient *http.Client, logger *zap.Logger) (*osClientCurrent, error) {

	transport := httpClient.Transport

	return opensearch.NewClient(osConfigCurrent{
		Transport: transport,

		// configure connection setup
		Addresses:    []string{endpoint},
		DisableRetry: true,

		// configure internal metrics reporting and logging
		EnableMetrics:     false, // TODO
		EnableDebugLogger: false, // TODO
		Logger:            (*clientLogger)(logger),
	})
}

func newBulkIndexer(logger *zap.Logger, client *opensearch.Client) (osBulkIndexerCurrent, error) {
	return opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		NumWorkers: 1,
		Client:     client,
		OnError: func(_ context.Context, err error) {
			logger.Error(fmt.Sprintf("Bulk indexer error: %v", err))
		},
	})
}

func shouldRetryEvent(status int) bool {
	for _, retryable := range retryOnStatus {
		if status == retryable {
			return true
		}
	}
	return false
}

func pushDocuments(ctx context.Context, logger *zap.Logger, index string, document []byte, client *osClientCurrent, maxAttempts int) error {
	bulkIndexer, err := newBulkIndexer(logger, client)
	if err != nil {
		return err
	}

	attempts := 1
	body := bytes.NewReader(document)
	item := osBulkIndexerItem{Action: "create", Index: index, Body: body}
	// Setup error handler. The handler handles the per item response status based on the
	// selective ACKing in the bulk response.
	item.OnFailure = func(ctx context.Context, item osBulkIndexerItem, resp osBulkIndexerResponseItem, err error) {
		switch {
		case attempts < maxAttempts && shouldRetryEvent(resp.Status):
			// TODO this failure probably should be kicked out to be retried by exporterhelper.
			logger.Debug("Retrying to index",
				zap.String("name", index),
				zap.Int("attempt", attempts),
				zap.Int("status", resp.Status),
				zap.NamedError("reason", err))

			attempts++
			// _, _ = body.Seek(0, io.SeekStart)
			// _ = bulkIndexer.Add(ctx, item)

		case resp.Status == 0 && err != nil:
			// Encoding error. We didn't even attempt to send the event
			logger.Error("Drop docs: failed to add docs to the bulk request buffer.",
				zap.NamedError("reason", err))

		case err != nil:
			logger.Error("Drop docs: failed to index",
				zap.String("name", index),
				zap.Int("attempt", attempts),
				zap.Int("status", resp.Status),
				zap.NamedError("reason", err))

		default:
			errorJSON, _ := json.Marshal(resp.Error)
			logger.Error(fmt.Sprintf("Drop docs: failed to index: %s", errorJSON),
				zap.Int("attempt", attempts),
				zap.Int("status", resp.Status))
		}
	}

	err = bulkIndexer.Add(ctx, item)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to add item to bulk indexer: %s", err))
	}
	return bulkIndexer.Close(ctx)
}
