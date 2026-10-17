// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v4/opensearchutil"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type logBulkIndexer struct {
	bulkAction  string
	pipeline    string
	model       mappingModel
	errs        []error
	bulkIndexer opensearchutil.BulkIndexer

	// mu guards the fields below and errs. Item callbacks run on the bulk
	// indexer's worker goroutine while items are still being added, and can
	// outlive close when the context expires during a flush.
	mu sync.Mutex
	// requestErr is set when a whole bulk request was rejected with a retryable status.
	requestErr error
	// retryErrs and retryLogs hold the records OpenSearch rejected one by one with a retryable status.
	retryErrs []error
	retryLogs plog.Logs
}

func newLogBulkIndexer(bulkAction string, model mappingModel, pipeline string) *logBulkIndexer {
	return &logBulkIndexer{bulkAction: bulkAction, pipeline: pipeline, model: model, errs: nil, bulkIndexer: nil}
}

func (lbi *logBulkIndexer) start(client *opensearchapi.Client) error {
	var startErr error
	lbi.bulkIndexer, startErr = newLogOpenSearchBulkIndexer(client, lbi.onIndexerError, lbi.pipeline)
	return startErr
}

func (lbi *logBulkIndexer) joinedError() error {
	lbi.mu.Lock()
	defer lbi.mu.Unlock()
	return joinBulkErrors(lbi.errs, lbi.requestErr, lbi.retryErrs, func(err error) error {
		return consumererror.NewLogs(err, lbi.retryLogs)
	})
}

func (lbi *logBulkIndexer) close(ctx context.Context) {
	closeErr := lbi.bulkIndexer.Close(ctx)
	if closeErr != nil {
		lbi.mu.Lock()
		lbi.errs = append(lbi.errs, closeErr)
		lbi.mu.Unlock()
	}
}

func (lbi *logBulkIndexer) onIndexerError(_ context.Context, indexerErr error) {
	if indexerErr != nil {
		lbi.appendPermanentError(consumererror.NewPermanent(indexerErr))
	}
}

func (lbi *logBulkIndexer) appendPermanentError(e error) {
	lbi.mu.Lock()
	defer lbi.mu.Unlock()
	lbi.errs = append(lbi.errs, consumererror.NewPermanent(e))
}

func (lbi *logBulkIndexer) appendRetryLogError(err error, log plog.Logs) {
	lbi.mu.Lock()
	defer lbi.mu.Unlock()
	if len(lbi.retryErrs) == 0 {
		lbi.retryLogs = plog.NewLogs()
	}
	lbi.retryErrs = append(lbi.retryErrs, err)
	log.ResourceLogs().MoveAndAppendTo(lbi.retryLogs.ResourceLogs())
}

func (lbi *logBulkIndexer) submit(ctx context.Context, ld plog.Logs, ir *indexResolver, cfg *Config, timestamp time.Time) {
	keys := ir.extractPlaceholderKeys(cfg.LogsIndex)
	timeSuffix := ir.calculateTimeSuffix(cfg.LogsIndexTimeFormat, timestamp)
	resourceLogs := ld.ResourceLogs()

	for i := 0; i < resourceLogs.Len(); i++ {
		il := resourceLogs.At(i)
		resource := il.Resource()
		resourceAttrs := ir.collectResourceAttributes(resource, keys)
		scopeLogs := il.ScopeLogs()

		for j := 0; j < scopeLogs.Len(); j++ {
			scopeSpan := scopeLogs.At(j)
			scopeAttrs := ir.collectScopeAttributes(scopeSpan.Scope(), keys)
			logs := scopeLogs.At(j).LogRecords()

			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				indexName := ir.resolveIndexName(cfg.LogsIndex, cfg.LogsIndexFallback, log.Attributes(), keys, scopeAttrs, resourceAttrs, timeSuffix)
				lbi.processItem(ctx, indexName, resource, il.SchemaUrl(), scopeSpan.Scope(), scopeSpan.SchemaUrl(), log)
			}
		}
	}
}

func (lbi *logBulkIndexer) processItem(ctx context.Context, indexName string, resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, logRecord plog.LogRecord) {
	payload, err := lbi.model.encodeLog(resource, scope, scopeSchemaURL, logRecord)
	if err != nil {
		lbi.appendPermanentError(err)
	} else {
		ItemFailureHandler := func(_ context.Context, _ opensearchutil.BulkIndexerItem, resp opensearchapi.BulkRespItem, itemErr error) {
			// Setup error handler. The handler handles the per item response status based on the
			// selective ACKing in the bulk response.
			lbi.processItemFailure(resp, itemErr, makeLog(resource, resourceSchemaURL, scope, scopeSchemaURL, logRecord))
		}
		bi := lbi.newBulkIndexerItem(payload, indexName)
		bi.OnFailure = ItemFailureHandler
		err = lbi.bulkIndexer.Add(ctx, bi)
		if err != nil {
			lbi.appendRetryLogError(err, makeLog(resource, resourceSchemaURL, scope, scopeSchemaURL, logRecord))
		}
	}
}

func makeLog(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, log plog.LogRecord) plog.Logs {
	logs := plog.NewLogs()
	rs := logs.ResourceLogs().AppendEmpty()
	resource.CopyTo(rs.Resource())
	rs.SetSchemaUrl(resourceSchemaURL)
	ss := rs.ScopeLogs().AppendEmpty()

	ss.SetSchemaUrl(scopeSchemaURL)
	scope.CopyTo(ss.Scope())
	s := ss.LogRecords().AppendEmpty()

	log.CopyTo(s)

	return logs
}

func (lbi *logBulkIndexer) processItemFailure(resp opensearchapi.BulkRespItem, itemErr error, logs plog.Logs) {
	switch {
	case shouldRetryEvent(resp.Status):
		// Recoverable OpenSearch error
		lbi.appendRetryLogError(responseAsError(resp), logs)
	case resp.Status == 0 && isRetryableResponse(itemErr):
		// The whole bulk request was rejected, and this is called for each of its items.
		lbi.mu.Lock()
		if lbi.requestErr == nil {
			lbi.requestErr = itemErr
		}
		lbi.mu.Unlock()
	case resp.Status != 0 && itemErr == nil:
		// Non-recoverable OpenSearch error while indexing document
		lbi.appendPermanentError(responseAsError(resp))
	default:
		// Encoding error. We didn't even attempt to send the event
		lbi.appendPermanentError(itemErr)
	}
}

func (lbi *logBulkIndexer) newBulkIndexerItem(document []byte, indexName string) opensearchutil.BulkIndexerItem {
	body := bytes.NewReader(document)
	item := opensearchutil.BulkIndexerItem{Action: lbi.bulkAction, Index: indexName, Body: body}
	return item
}

func newLogOpenSearchBulkIndexer(client *opensearchapi.Client, onIndexerError func(context.Context, error), pipeline string) (opensearchutil.BulkIndexer, error) {
	return opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		NumWorkers: 1,
		Client:     client,
		OnError:    onIndexerError,
		Pipeline:   pipeline,
	})
}
