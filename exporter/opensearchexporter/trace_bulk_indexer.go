// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"slices"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v4/opensearchutil"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type traceBulkIndexer struct {
	bulkAction  string
	pipeline    string
	model       mappingModel
	errs        []error
	bulkIndexer opensearchutil.BulkIndexer
}

func newTraceBulkIndexer(bulkAction string, model mappingModel, pipeline string) *traceBulkIndexer {
	return &traceBulkIndexer{bulkAction: bulkAction, pipeline: pipeline, model: model, errs: nil, bulkIndexer: nil}
}

func (tbi *traceBulkIndexer) joinedError() error {
	return errors.Join(tbi.errs...)
}

func (tbi *traceBulkIndexer) start(client *opensearchapi.Client) error {
	var startErr error
	tbi.bulkIndexer, startErr = newOpenSearchBulkIndexer(client, tbi.onIndexerError, tbi.pipeline)
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
		// Indexer-level errors are transport/flush failures (connection refused,
		// timeout, DNS). They are transient, so surface them as a retryable error
		// and let exporterhelper's retry_on_failure resend the batch instead of
		// dropping it as permanent.
		tbi.errs = append(tbi.errs, indexerErr)
	}
}

func (tbi *traceBulkIndexer) appendPermanentError(e error) {
	tbi.errs = append(tbi.errs, consumererror.NewPermanent(e))
}

func (tbi *traceBulkIndexer) appendRetryTraceError(err error, trace ptrace.Traces) {
	tbi.errs = append(tbi.errs, consumererror.NewTraces(err, trace))
}

func (tbi *traceBulkIndexer) submit(ctx context.Context, td ptrace.Traces, ir *indexResolver, cfg *Config, timestamp time.Time) {
	keys := ir.extractPlaceholderKeys(cfg.TracesIndex)
	timeSuffix := ir.calculateTimeSuffix(cfg.TracesIndexTimeFormat, timestamp)
	resourceSpans := td.ResourceSpans()

	for i := 0; i < resourceSpans.Len(); i++ {
		il := resourceSpans.At(i)
		resource := il.Resource()
		resourceAttrs := ir.collectResourceAttributes(resource, keys)
		scopeSpans := il.ScopeSpans()

		for j := 0; j < scopeSpans.Len(); j++ {
			scopeSpan := scopeSpans.At(j)
			scopeAttrs := ir.collectScopeAttributes(scopeSpan.Scope(), keys)
			spans := scopeSpans.At(j).Spans()

			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				indexName := ir.resolveIndexName(cfg.TracesIndex, cfg.TracesIndexFallback, span.Attributes(), keys, scopeAttrs, resourceAttrs, timeSuffix)
				tbi.processItem(ctx, indexName, resource, il.SchemaUrl(), scopeSpan.Scope(), scopeSpan.SchemaUrl(), span)
			}
		}
	}
}

func (tbi *traceBulkIndexer) processItem(ctx context.Context, indexName string, resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span) {
	payload, err := tbi.model.encodeTrace(resource, scope, scopeSchemaURL, span)
	if err != nil {
		tbi.appendPermanentError(err)
	} else {
		ItemFailureHandler := func(_ context.Context, _ opensearchutil.BulkIndexerItem, resp opensearchapi.BulkRespItem, itemErr error) {
			// Setup error handler. The handler handles the per item response status based on the
			// selective ACKing in the bulk response.
			tbi.processItemFailure(resp, itemErr, makeTrace(resource, resourceSchemaURL, scope, scopeSchemaURL, span))
		}
		bi := tbi.newBulkIndexerItem(payload, indexName)
		bi.OnFailure = ItemFailureHandler
		err = tbi.bulkIndexer.Add(ctx, bi)
		if err != nil {
			tbi.appendRetryTraceError(err, makeTrace(resource, resourceSchemaURL, scope, scopeSchemaURL, span))
		}
	}
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

func (tbi *traceBulkIndexer) processItemFailure(resp opensearchapi.BulkRespItem, itemErr error, traces ptrace.Traces) {
	switch {
	case shouldRetryEvent(resp.Status):
		// Recoverable OpenSearch error
		tbi.appendRetryTraceError(responseAsError(resp), traces)
	case resp.Status != 0 && itemErr == nil:
		// Non-recoverable OpenSearch error while indexing document
		tbi.appendPermanentError(responseAsError(resp))
	default:
		// No server status classified the item, so this is either a flush/
		// transport failure (retry) or an encoding failure we never sent
		// (permanent). On a flush failure opensearchutil reports the same error
		// through both this per-item path and onIndexerError, so both must land
		// on retryable or the joined error is still permanent via errors.As.
		//
		// The retryable error is deliberately bare rather than carrying this one
		// item. A flush failure fires this callback for every buffered item, and
		// exporterhelper's OnError resolves the first consumererror it finds and
		// retries only that payload, so wrapping here would narrow the retry to a
		// single record and silently drop the rest of the batch. With no payload
		// attached, OnError falls through and the whole request is resent.
		if isRetryableError(itemErr) {
			tbi.errs = append(tbi.errs, itemErr)
		} else {
			tbi.appendPermanentError(itemErr)
		}
	}
}

// responseAsError converts an opensearchapi.BulkRespItem.Error into an error
func responseAsError(item opensearchapi.BulkRespItem) error {
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
	return slices.Contains(retryOnStatus, status)
}

// isRetryableError reports whether err is a transient transport/flush failure
// (connection refused, timeout, DNS) that should be retried rather than dropped
// as permanent. Encoding failures, which never leave the process, are not
// transport errors and remain permanent.
//
// The net.Error check is deliberately broad. It also matches durable
// misconfigurations that arrive wrapped in *net.OpError or *url.Error, such as
// an untrusted certificate or a hostname that does not resolve, so those are
// retried until the retry sender gives up rather than dropped immediately. That
// is bounded by max_elapsed_time and is the safer default: misclassifying a
// transient failure as permanent loses data, whereas misclassifying a permanent
// one only costs retries.
//
// context.Canceled is included because a cancelled context reaches this path as
// a flush failure for data that was never accepted. Treating it as permanent
// would drop that batch on shutdown; treating it as retryable lets the retry
// sender return immediately on ctx.Done() and leave the data to the queue.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func (tbi *traceBulkIndexer) newBulkIndexerItem(document []byte, indexName string) opensearchutil.BulkIndexerItem {
	body := bytes.NewReader(document)
	item := opensearchutil.BulkIndexerItem{Action: tbi.bulkAction, Index: indexName, Body: body}
	return item
}

func newOpenSearchBulkIndexer(client *opensearchapi.Client, onIndexerError func(context.Context, error), pipeline string) (opensearchutil.BulkIndexer, error) {
	return opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		NumWorkers: 1,
		Client:     client,
		OnError:    onIndexerError,
		Pipeline:   pipeline,
	})
}
