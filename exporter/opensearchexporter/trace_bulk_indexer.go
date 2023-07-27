// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchutil"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

type traceBulkIndexer struct {
	dataset     string
	namespace   string
	errs        []error
	bulkIndexer opensearchutil.BulkIndexer
}

func newTraceBulkIndexer(dataset string, namespace string) *traceBulkIndexer {
	return &traceBulkIndexer{dataset, namespace, nil, nil}
}

func (tbi *traceBulkIndexer) JoinedError() error {
	return multierr.Combine(tbi.errs...)
}

func (tbi *traceBulkIndexer) Start(client *opensearch.Client) error {
	var startErr error
	tbi.bulkIndexer, startErr = newOpenSearchBulkIndexer(client, tbi.OnIndexerError)
	return startErr
}

func (tbi *traceBulkIndexer) Close(ctx context.Context) {
	closeErr := tbi.bulkIndexer.Close(ctx)
	if closeErr != nil {
		tbi.errs = append(tbi.errs, closeErr)
	}
}

func (tbi *traceBulkIndexer) HasErrors() bool {
	return len(tbi.errs) > 0
}

func (tbi *traceBulkIndexer) OnIndexerError(_ context.Context, indexerErr error) {
	if indexerErr != nil {
		tbi.errs = append(tbi.errs, consumererror.NewPermanent(indexerErr))
	}
}

func (tbi *traceBulkIndexer) appendPermanentError(e error) {
	tbi.errs = append(tbi.errs, consumererror.NewPermanent(e))
}

func (tbi *traceBulkIndexer) appendRetryTraceError(err error, trace ptrace.Traces) {
	tbi.errs = append(tbi.errs, consumererror.NewTraces(err, trace))
}

func (tbi *traceBulkIndexer) Submit(ctx context.Context, td ptrace.Traces) {
	forEachSpan(td, func(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span) {
		payload, err := tbi.createJSON(resource, scope, scopeSchemaURL, span)
		if err != nil {
			tbi.appendPermanentError(err)
		} else {
			ItemFailureHandler := func(ctx context.Context, item opensearchutil.BulkIndexerItem, resp opensearchutil.BulkIndexerResponseItem, itemErr error) {
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

func (tbi *traceBulkIndexer) createJSON(
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	schemaURL string,
	span ptrace.Span,
) ([]byte, error) {
	sso := SSOSpan{}
	sso.Attributes = span.Attributes().AsRaw()
	sso.DroppedAttributesCount = span.DroppedAttributesCount()
	sso.DroppedEventsCount = span.DroppedEventsCount()
	sso.DroppedLinksCount = span.DroppedLinksCount()
	sso.EndTime = span.EndTimestamp().AsTime()
	sso.Kind = span.Kind().String()
	sso.Name = span.Name()
	sso.ParentSpanID = span.ParentSpanID().String()
	sso.Resource = resource.Attributes().AsRaw()
	sso.SpanID = span.SpanID().String()
	sso.StartTime = span.StartTimestamp().AsTime()
	sso.Status.Code = span.Status().Code().String()
	sso.Status.Message = span.Status().Message()
	sso.TraceID = span.TraceID().String()
	sso.TraceState = span.TraceState().AsRaw()

	if span.Events().Len() > 0 {
		sso.Events = make([]SSOSpanEvent, span.Events().Len())
		for i := 0; i < span.Events().Len(); i++ {
			e := span.Events().At(i)
			ssoEvent := &sso.Events[i]
			ssoEvent.Attributes = e.Attributes().AsRaw()
			ssoEvent.DroppedAttributesCount = e.DroppedAttributesCount()
			ssoEvent.Name = e.Name()
			ts := e.Timestamp().AsTime()
			if ts.Unix() != 0 {
				ssoEvent.Timestamp = &ts
			} else {
				now := time.Now()
				ssoEvent.ObservedTimestamp = &now
			}
		}
	}

	dataStream := DataStream{}
	if tbi.dataset != "" {
		dataStream.Dataset = tbi.dataset
	}

	if tbi.namespace != "" {
		dataStream.Namespace = tbi.namespace
	}

	if dataStream != (DataStream{}) {
		dataStream.Type = "span"
		sso.Attributes["data_stream"] = dataStream
	}

	sso.InstrumentationScope.Name = scope.Name()
	sso.InstrumentationScope.DroppedAttributesCount = scope.DroppedAttributesCount()
	sso.InstrumentationScope.Version = scope.Version()
	sso.InstrumentationScope.SchemaURL = schemaURL
	sso.InstrumentationScope.Attributes = scope.Attributes().AsRaw()

	if span.Links().Len() > 0 {
		sso.Links = make([]SSOSpanLinks, span.Links().Len())
		for i := 0; i < span.Links().Len(); i++ {
			link := span.Links().At(i)
			ssoLink := &sso.Links[i]
			ssoLink.Attributes = link.Attributes().AsRaw()
			ssoLink.DroppedAttributesCount = link.DroppedAttributesCount()
			ssoLink.TraceID = link.TraceID().String()
			ssoLink.TraceState = link.TraceState().AsRaw()
			ssoLink.SpanID = link.SpanID().String()
		}
	}
	return json.Marshal(sso)
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

func shouldRetryEvent(status int) bool {
	var retryOnStatus = []int{500, 502, 503, 504, 429}
	for _, retryable := range retryOnStatus {
		if status == retryable {
			return true
		}
	}
	return false
}

func (tbi *traceBulkIndexer) newBulkIndexerItem(document []byte) opensearchutil.BulkIndexerItem {
	body := bytes.NewReader(document)
	item := opensearchutil.BulkIndexerItem{Action: "create", Index: tbi.getIndexName(), Body: body}
	return item
}

func (tbi *traceBulkIndexer) getIndexName() string {
	return strings.Join([]string{"sso_traces", tbi.dataset, tbi.namespace}, "-")
}

func newOpenSearchBulkIndexer(client *opensearch.Client, onIndexerError func(context.Context, error)) (opensearchutil.BulkIndexer, error) {
	return opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		NumWorkers: 1,
		Client:     client,
		OnError:    onIndexerError,
	})
}

func forEachSpan(td ptrace.Traces, iterator func(pcommon.Resource, string, pcommon.InstrumentationScope, string, ptrace.Span)) {
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
				iterator(resource, il.SchemaUrl(), scopeSpan.Scope(), scopeSpan.SchemaUrl(), span)
			}
		}
	}
}
