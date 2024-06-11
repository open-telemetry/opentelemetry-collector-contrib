// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type elasticsearchExporter struct {
	logger *zap.Logger

	index          string
	logstashFormat LogstashFormatSettings
	dynamicIndex   bool

	client      *esClientCurrent
	bulkIndexer *esBulkIndexerCurrent
	model       mappingModel
}

func newExporter(logger *zap.Logger, cfg *Config, index string, dynamicIndex bool) (*elasticsearchExporter, error) {
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

	return &elasticsearchExporter{
		logger:      logger,
		client:      client,
		bulkIndexer: bulkIndexer,

		index:          index,
		dynamicIndex:   dynamicIndex,
		model:          model,
		logstashFormat: cfg.LogstashFormat,
	}, nil
}

func (e *elasticsearchExporter) Shutdown(ctx context.Context) error {
	return e.bulkIndexer.Close(ctx)
}

func (e *elasticsearchExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	items := make([]esBulkIndexerItem, 0, ld.LogRecordCount())
	var errs []error
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()
		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			ill := ills.At(j)
			scope := ill.Scope()
			logs := ill.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				item, err := e.logRecordToItem(ctx, resource, logs.At(k), scope)
				if err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					errs = append(errs, err)
					continue
				}
				items = append(items, item)
			}
		}
	}
	if err := e.bulkIndexer.AddBatchAndFlush(ctx, items); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *elasticsearchExporter) logRecordToItem(ctx context.Context, resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope) (esBulkIndexerItem, error) {
	fIndex := e.index
	if e.dynamicIndex {
		prefix := getFromAttributes(indexPrefix, resource, scope, record)
		suffix := getFromAttributes(indexSuffix, resource, scope, record)

		fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return esBulkIndexerItem{}, err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeLog(resource, record, scope)
	if err != nil {
		return esBulkIndexerItem{}, fmt.Errorf("Failed to encode log event: %w", err)
	}
	return esBulkIndexerItem{
		Index: fIndex,
		Body:  bytes.NewReader(document),
	}, nil
}

func (e *elasticsearchExporter) pushTraceData(
	ctx context.Context,
	td ptrace.Traces,
) error {
	items := make([]esBulkIndexerItem, 0, td.SpanCount())
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
						return cerr
					}
					errs = append(errs, err)
					continue
				}
				items = append(items, item)
			}
		}
	}
	if err := e.bulkIndexer.AddBatchAndFlush(ctx, items); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *elasticsearchExporter) traceRecordToItem(ctx context.Context, resource pcommon.Resource, span ptrace.Span, scope pcommon.InstrumentationScope) (esBulkIndexerItem, error) {
	fIndex := e.index
	if e.dynamicIndex {
		prefix := getFromAttributes(indexPrefix, resource, scope, span)
		suffix := getFromAttributes(indexSuffix, resource, scope, span)

		fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return esBulkIndexerItem{}, err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeSpan(resource, span, scope)
	if err != nil {
		return esBulkIndexerItem{}, fmt.Errorf("Failed to encode trace record: %w", err)
	}
	return esBulkIndexerItem{
		Index: fIndex,
		Body:  bytes.NewReader(document),
	}, nil
}
