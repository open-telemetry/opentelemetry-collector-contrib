// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package elasticsearchexporter contains an opentelemetry-collector exporter
// for Elasticsearch.
package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type elasticsearchTracesExporter struct {
	logger *zap.Logger

	index          string
	logstashFormat LogstashFormatSettings
	dynamicIndex   bool

	client      *esClientCurrent
	bulkIndexer *esBulkIndexerCurrent
	model       mappingModel
}

func newTracesExporter(logger *zap.Logger, cfg *Config) (*elasticsearchTracesExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newElasticsearchClient(logger, cfg)
	if err != nil {
		return nil, err
	}

	bulkIndexer, err := newBulkIndexer(logger, client, cfg)
	if err != nil {
		return nil, err
	}

	model := &encodeModel{
		dedup: cfg.Mapping.Dedup,
		dedot: cfg.Mapping.Dedot,
		mode:  cfg.MappingMode(),
	}

	return &elasticsearchTracesExporter{
		logger:      logger,
		client:      client,
		bulkIndexer: bulkIndexer,

		index:          cfg.TracesIndex,
		dynamicIndex:   cfg.TracesDynamicIndex.Enabled,
		model:          model,
		logstashFormat: cfg.LogstashFormat,
	}, nil
}

func (e *elasticsearchTracesExporter) Shutdown(ctx context.Context) error {
	return e.bulkIndexer.Close(ctx)
}

func (e *elasticsearchTracesExporter) traceDataToRequest(
	ctx context.Context,
	td ptrace.Traces,
) (exporterhelper.Request, error) {
	req := newRequest(e.bulkIndexer)
	var errs []error
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		il := resourceSpans.At(i)
		resource := il.Resource()
		scopeSpans := il.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			scopeSpan := scopeSpans.At(j)
			scope := scopeSpan.Scope()
			spans := scopeSpan.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				item, err := e.traceRecordToItem(ctx, resource, span, scope)
				if err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return req, cerr
					}
					errs = append(errs, err)
					continue
				}
				req.Add(item)
			}
		}
	}

	return req, errors.Join(errs...)
}

func (e *elasticsearchTracesExporter) traceRecordToItem(ctx context.Context, resource pcommon.Resource, span ptrace.Span, scope pcommon.InstrumentationScope) (bulkIndexerItem, error) {
	fIndex := e.index
	if e.dynamicIndex {
		prefix := getFromAttributes(indexPrefix, resource, scope, span)
		suffix := getFromAttributes(indexSuffix, resource, scope, span)

		fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return bulkIndexerItem{}, err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeSpan(resource, span, scope)
	if err != nil {
		return bulkIndexerItem{}, fmt.Errorf("Failed to encode trace record: %w", err)
	}
	return bulkIndexerItem{
		Index: fIndex,
		Body:  document,
	}, nil
}

func pushDocuments(ctx context.Context, index string, document []byte, current *esBulkIndexerCurrent) error {
	return fmt.Errorf("not implemented")
}
