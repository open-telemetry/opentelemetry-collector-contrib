// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"context"
	"errors"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchutil"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type logBulkIndexer struct {
	index       string
	bulkAction  string
	model       mappingModel
	errs        []error
	bulkIndexer opensearchutil.BulkIndexer
}

func newLogBulkIndexer(index, bulkAction string, model mappingModel) *logBulkIndexer {
	return &logBulkIndexer{index, bulkAction, model, nil, nil}
}

func (lbi *logBulkIndexer) start(client *opensearch.Client) error {
	var startErr error
	lbi.bulkIndexer, startErr = newLogOpenSearchBulkIndexer(client, lbi.onIndexerError)
	return startErr
}

func (lbi *logBulkIndexer) joinedError() error {
	return errors.Join(lbi.errs...)
}

func (lbi *logBulkIndexer) close(ctx context.Context) {
	closeErr := lbi.bulkIndexer.Close(ctx)
	if closeErr != nil {
		lbi.errs = append(lbi.errs, closeErr)
	}
}

func (lbi *logBulkIndexer) onIndexerError(_ context.Context, indexerErr error) {
	if indexerErr != nil {
		lbi.appendPermanentError(consumererror.NewPermanent(indexerErr))
	}
}

func (lbi *logBulkIndexer) appendPermanentError(e error) {
	lbi.errs = append(lbi.errs, consumererror.NewPermanent(e))
}

func (lbi *logBulkIndexer) appendRetryLogError(err error, log plog.Logs) {
	lbi.errs = append(lbi.errs, consumererror.NewLogs(err, log))
}

func (lbi *logBulkIndexer) submit(ctx context.Context, ld plog.Logs) {
	forEachLog(ld, func(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, log plog.LogRecord) {
		payload, err := lbi.model.encodeLog(resource, scope, scopeSchemaURL, log)
		if err != nil {
			lbi.appendPermanentError(err)
		} else {
			ItemFailureHandler := func(ctx context.Context, item opensearchutil.BulkIndexerItem, resp opensearchutil.BulkIndexerResponseItem, itemErr error) {
				// Setup error handler. The handler handles the per item response status based on the
				// selective ACKing in the bulk response.
				lbi.processItemFailure(resp, itemErr, makeLog(resource, resourceSchemaURL, scope, scopeSchemaURL, log))
			}
			bi := lbi.newBulkIndexerItem(payload)
			bi.OnFailure = ItemFailureHandler
			err = lbi.bulkIndexer.Add(ctx, bi)
			if err != nil {
				lbi.appendRetryLogError(err, makeLog(resource, resourceSchemaURL, scope, scopeSchemaURL, log))
			}
		}
	})
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

func (lbi *logBulkIndexer) processItemFailure(resp opensearchutil.BulkIndexerResponseItem, itemErr error, logs plog.Logs) {
	switch {
	case shouldRetryEvent(resp.Status):
		// Recoverable OpenSearch error
		lbi.appendRetryLogError(responseAsError(resp), logs)
	case resp.Status != 0 && itemErr == nil:
		// Non-recoverable OpenSearch error while indexing document
		lbi.appendPermanentError(responseAsError(resp))
	default:
		// Encoding error. We didn't even attempt to send the event
		lbi.appendPermanentError(itemErr)
	}
}

func (lbi *logBulkIndexer) newBulkIndexerItem(document []byte) opensearchutil.BulkIndexerItem {
	body := bytes.NewReader(document)
	item := opensearchutil.BulkIndexerItem{Action: lbi.bulkAction, Index: lbi.index, Body: body}
	return item
}

func newLogOpenSearchBulkIndexer(client *opensearch.Client, onIndexerError func(context.Context, error)) (opensearchutil.BulkIndexer, error) {
	return opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		NumWorkers: 1,
		Client:     client,
		OnError:    onIndexerError,
	})
}

func forEachLog(ld plog.Logs, visitor func(pcommon.Resource, string, pcommon.InstrumentationScope, string, plog.LogRecord)) {
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		il := resourceLogs.At(i)
		resource := il.Resource()
		scopeLogs := il.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			scopeSpan := scopeLogs.At(j)
			logs := scopeLogs.At(j).LogRecords()

			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				visitor(resource, il.SchemaUrl(), scopeSpan.Scope(), scopeSpan.SchemaUrl(), log)
			}
		}
	}
}
