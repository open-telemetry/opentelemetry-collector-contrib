// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package elasticsearchexporter contains an opentelemetry-collector exporter
// for Elasticsearch.
package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type elasticsearchTracesExporter struct {
	logger *zap.Logger

	index        string
	dynamicIndex bool
	maxAttempts  int

	client      *esClientCurrent
	bulkIndexer esBulkIndexerCurrent
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

	maxAttempts := 1
	if cfg.Retry.Enabled {
		maxAttempts = cfg.Retry.MaxRequests
	}

	traceExporter := &elasticsearchTracesExporter{
		logger:       logger,
		client:       client,
		bulkIndexer:  bulkIndexer,
		index:        cfg.TracesIndex,
		dynamicIndex: cfg.TracesDynamicIndex.Enabled,
		maxAttempts:  maxAttempts,
	}

	if m, ok := mappingModes[cfg.Mapping.Mode]; ok {
		switch m {
		case MappingECS:
			traceExporter.model = &encodeModel{dedup: cfg.Mapping.Dedup, dedot: cfg.Mapping.Dedot}
		case MappingJaeger:
			traceExporter.model = &encodeJaegerModel{}
		default:
			traceExporter.model = &encodeModel{dedup: cfg.Mapping.Dedup, dedot: cfg.Mapping.Dedot}
		}
	}

	return traceExporter, nil
}

func (e *elasticsearchTracesExporter) Shutdown(ctx context.Context) error {
	return e.bulkIndexer.Close(ctx)
}

func (e *elasticsearchTracesExporter) pushTraceData(
	ctx context.Context,
	td ptrace.Traces,
) error {
	var errs []error
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		il := resourceSpans.At(i)
		resource := il.Resource()
		scopeSpans := il.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			ils := scopeSpans.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				if err := e.pushTraceRecord(ctx, resource, ils.Scope(), spans.At(k)); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, err)
				}
			}
		}
	}

	return multierr.Combine(errs...)
}

func (e *elasticsearchTracesExporter) pushTraceRecord(ctx context.Context, resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) error {
	fIndex := e.index
	if e.dynamicIndex {
		prefix := getFromBothResourceAndAttribute(indexPrefix, resource, span)
		suffix := getFromBothResourceAndAttribute(indexSuffix, resource, span)

		fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
	}

	document, err := e.model.encodeSpan(resource, scope, span)
	if err != nil {
		return fmt.Errorf("Failed to encode trace record: %w", err)
	}
	return pushDocuments(ctx, e.logger, fIndex, document, e.bulkIndexer, e.maxAttempts)
}
