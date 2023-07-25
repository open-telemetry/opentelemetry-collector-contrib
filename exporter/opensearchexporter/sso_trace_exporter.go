// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchutil"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	// defaultNamespace value is used as SSOTracesExporter.Namespace when component.Config.Namespace is not set.
	defaultNamespace = "namespace"

	// defaultDataset value is used as SSOTracesExporter.Dataset when component.Config.Dataset is not set.
	defaultDataset = "default"
)

type SSOTracesExporter struct {
	client       *opensearch.Client
	Namespace    string
	Dataset      string
	httpSettings confighttp.HTTPClientSettings
	telemetry    component.TelemetrySettings
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

func (s *SSOTracesExporter) getIndexName() string {
	return strings.Join([]string{"sso_traces", s.Dataset, s.Namespace}, "-")
}

func (s *SSOTracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	// TODO Refactor
	// Generate JSON first, then create bulk indexer to send them.
	var errs []error

	bulkIndexer, err := newBulkIndexer(s.telemetry.Logger, s.client)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	defer func(bulkIndexer opensearchutil.BulkIndexer, ctx context.Context) {
		deferErr := bulkIndexer.Close(ctx)
		if deferErr != nil {
			errs = append(errs, deferErr)
		}
	}(bulkIndexer, ctx)

	var spansToRetry []int
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		il := resourceSpans.At(i)
		resource := il.Resource()
		scopeSpans := il.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			scope := scopeSpans.At(j).Scope()
			schemaURL := scopeSpans.At(j).SchemaUrl()
			for k := 0; k < spans.Len(); k++ {
				payload, err := s.createJSONDocument(resource, scope, schemaURL, spans.At(k))
				if err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, consumererror.NewPermanent(err))
				} else {
					// construct destination index name by combining Dataset and Namespace options if they are set.
					bi := s.newBulkIndexerItem(payload)

					// Setup error handler. The handler handles the per item response status based on the
					// selective ACKing in the bulk response.
					bi.OnFailure = func(ctx context.Context, item opensearchutil.BulkIndexerItem, resp opensearchutil.BulkIndexerResponseItem, err error) {
						switch {
						case shouldRetryEvent(resp.Status):
							spansToRetry = append(spansToRetry, k)
							s.telemetry.Logger.Debug("Retrying to index",
								zap.String("name", item.Index),
								zap.Int("status", resp.Status),
								zap.NamedError("reason", err))

						case resp.Status == 0 && err != nil:
							// Encoding error. We didn't even attempt to send the event
							s.telemetry.Logger.Error("Drop docs: failed to add docs to the bulk request buffer.",
								zap.NamedError("reason", err))
							errs = append(errs, consumererror.NewPermanent(err))

						case err != nil:
							s.telemetry.Logger.Error("Drop docs: failed to index",
								zap.String("name", item.Index),
								zap.Int("status", resp.Status),
								zap.NamedError("reason", err))

						default:
							// OpenSearch error while indexing document
							errorJSON, _ := json.Marshal(resp.Error)
							s.telemetry.Logger.Error(fmt.Sprintf("Drop docs: failed to index: %s", errorJSON),
								zap.Int("status", resp.Status))
						}
					}

					err = bulkIndexer.Add(ctx, bi)
					if err != nil {
						s.telemetry.Logger.Error(fmt.Sprintf("Failed to add item to bulk indexer: %s", err))
					}
				}
			}
		}
	}

	return multierr.Combine(errs...)
}

func defaultIfEmpty(value string, def string) string {
	if value == "" {
		return def
	}
	return value
}

func (s *SSOTracesExporter) createJSONDocument(
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
	if s.Dataset != "" {
		dataStream.Dataset = s.Dataset
	}

	if s.Namespace != "" {
		dataStream.Namespace = s.Namespace
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

func (s *SSOTracesExporter) Start(_ context.Context, host component.Host) error {
	httpClient, err := s.httpSettings.ToClient(host, s.telemetry)
	if err != nil {
		return err
	}

	client, err := newOpenSearchClient(s.httpSettings.Endpoint, httpClient, s.telemetry.Logger)
	if err != nil {
		return err
	}

	s.client = client
	return nil
}

func newSSOTracesExporter(cfg *Config, set exporter.CreateSettings) (*SSOTracesExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &SSOTracesExporter{
		telemetry:    set.TelemetrySettings,
		Namespace:    defaultIfEmpty(cfg.Namespace, defaultNamespace),
		Dataset:      defaultIfEmpty(cfg.Dataset, defaultDataset),
		httpSettings: cfg.HTTPClientSettings,
	}, nil
}

func (s *SSOTracesExporter) newBulkIndexerItem(document []byte) opensearchutil.BulkIndexerItem {

	body := bytes.NewReader(document)
	item := opensearchutil.BulkIndexerItem{Action: "create", Index: s.getIndexName(), Body: body}
	return item
}

func newOpenSearchClient(endpoint string, httpClient *http.Client, logger *zap.Logger) (*opensearch.Client, error) {

	transport := httpClient.Transport

	return opensearch.NewClient(opensearch.Config{
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

func newBulkIndexer(logger *zap.Logger, client *opensearch.Client) (opensearchutil.BulkIndexer, error) {
	return opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		NumWorkers: 1,
		Client:     client,
		OnError: func(_ context.Context, err error) {
			logger.Error(fmt.Sprintf("Bulk indexer error: %v", err))
		},
	})
}
