// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchutil"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type traceBulkIndexer struct {
	dataset     string
	namespace   string
	bulkAction  string
	model       mappingModel
	errs        []error
	bulkIndexer opensearchutil.BulkIndexer
}

func newTraceBulkIndexer(dataset string, namespace string, bulkAction string, model mappingModel) *traceBulkIndexer {
	return &traceBulkIndexer{dataset, namespace, bulkAction, model, nil, nil}
}

func (tbi *traceBulkIndexer) joinedError() error {
	return errors.Join(tbi.errs...)
}

func (tbi *traceBulkIndexer) start(client *opensearch.Client) error {
	var startErr error
	tbi.bulkIndexer, startErr = newOpenSearchBulkIndexer(client, tbi.onIndexerError)
	return startErr
}

func (tbi *traceBulkIndexer) close(ctx context.Context) {
	closeErr := tbi.bulkIndexer.Close(ctx)
	if closeErr != nil {
		tbi.errs = append(tbi.errs, closeErr)
	}
}

func (tbi *traceBulkIndexer) onIndexerError(_ context.Context, indexerErr error) {
	if indexerErr != nil {
		tbi.appendPermanentError(consumererror.NewPermanent(indexerErr))
	}
}

func (tbi *traceBulkIndexer) appendPermanentError(e error) {
	tbi.errs = append(tbi.errs, consumererror.NewPermanent(e))
}

func (tbi *traceBulkIndexer) appendRetryTraceError(err error, trace ptrace.Traces) {
	tbi.errs = append(tbi.errs, consumererror.NewTraces(err, trace))
}

func (tbi *traceBulkIndexer) submit(ctx context.Context, td ptrace.Traces) {
	forEachSpan(td, func(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span) {
		payload, err := tbi.model.encodeTrace(resource, scope, scopeSchemaURL, span)
		if err != nil {
			tbi.appendPermanentError(err)
		} else {
			ItemFailureHandler := func(_ context.Context, _ opensearchutil.BulkIndexerItem, resp opensearchutil.BulkIndexerResponseItem, itemErr error) {
				// Setup error handler. The handler handles the per item response status based on the
				// selective ACKing in the bulk response.
				tbi.processItemFailure(resp, itemErr, makeTrace(resource, resourceSchemaURL, scope, scopeSchemaURL, span))
			}
			bi := tbi.newBulkIndexerItem(payload)
			bi.OnFailure = ItemFailureHandler
			err = tbi.bulkIndexer.Add(ctx, bi)
			if err != nil {
				tbi.appendRetryTraceError(err, makeTrace(resource, resourceSchemaURL, scope, scopeSchemaURL, span))
			}
		}
	})
}

func makeTrace(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	resource.CopyTo(rs.Resource())
	rs.SetSchemaUrl(resourceSchemaURL)
	ss := rs.ScopeSpans().AppendEmpty()

	ss.SetSchemaUrl(scopeSchemaURL)
	scope.CopyTo(ss.Scope())
	s := ss.Spans().AppendEmpty()

	span.CopyTo(s)

	return traces
}

func (tbi *traceBulkIndexer) processItemFailure(resp opensearchutil.BulkIndexerResponseItem, itemErr error, traces ptrace.Traces) {
	switch {
	case shouldRetryEvent(resp.Status):
		// Recoverable OpenSearch error
		tbi.appendRetryTraceError(responseAsError(resp), traces)
	case resp.Status != 0 && itemErr == nil:
		// Non-recoverable OpenSearch error while indexing document
		tbi.appendPermanentError(responseAsError(resp))
	default:
		// Encoding error. We didn't even attempt to send the event
		tbi.appendPermanentError(itemErr)
	}
}

// responseAsError converts an opensearchutil.BulkIndexerResponseItem.Error into an error
func responseAsError(item opensearchutil.BulkIndexerResponseItem) error {
	errorJSON, _ := json.Marshal(item.Error)
	return errors.New(string(errorJSON))
}

func attributesToMapString(attributes pcommon.Map) map[string]string {
	m := make(map[string]string, attributes.Len())
	for k, v := range attributes.All() {
		m[k] = v.AsString()
	}
	return m
}

func shouldRetryEvent(status int) bool {
	retryOnStatus := []int{500, 502, 503, 504, 429}
	for _, retryable := range retryOnStatus {
		if status == retryable {
			return true
		}
	}
	return false
}

func (tbi *traceBulkIndexer) newBulkIndexerItem(document []byte) opensearchutil.BulkIndexerItem {
	body := bytes.NewReader(document)
	item := opensearchutil.BulkIndexerItem{Action: tbi.bulkAction, Index: tbi.getIndexName(), Body: body}
	return item
}

func (tbi *traceBulkIndexer) getIndexName() string {
	return strings.Join([]string{"ss4o_traces", tbi.dataset, tbi.namespace}, "-")
}

func newOpenSearchBulkIndexer(client *opensearch.Client, onIndexerError func(context.Context, error)) (opensearchutil.BulkIndexer, error) {
	return opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		NumWorkers: 1,
		Client:     client,
		OnError:    onIndexerError,
	})
}

func forEachSpan(td ptrace.Traces, visitor func(pcommon.Resource, string, pcommon.InstrumentationScope, string, ptrace.Span)) {
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		il := resourceSpans.At(i)
		resource := il.Resource()
		scopeSpans := il.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			scopeSpan := scopeSpans.At(j)
			spans := scopeSpans.At(j).Spans()

			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				visitor(resource, il.SchemaUrl(), scopeSpan.Scope(), scopeSpan.SchemaUrl(), span)
			}
		}
	}
}
