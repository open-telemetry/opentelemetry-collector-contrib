// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type elasticsearchExporter struct {
	logger *zap.Logger

	index            string
	logstashFormat   LogstashFormatSettings
	dynamicIndex     bool
	dynamicIndexMode dynIdxMode

	client      *esClientCurrent
	bulkIndexer *esBulkIndexerCurrent
	model       mappingModel
	mode        MappingMode
}

type dynIdxMode int // dynamic index mode

const (
	dynIdxModePrefixSuffix dynIdxMode = iota
	dynIdxModeDataStream
)

const (
	sDynIdxModePrefixSuffix  = "prefix_suffix"
	sDynIdxModedimDataStream = "data_stream"
)

var errUnsupportedDynamicIndexMappingMode = errors.New("unsupported dynamic indexing mode")

func parseDIMode(s string) (dynIdxMode, error) {
	switch s {
	case "", sDynIdxModePrefixSuffix:
		return dynIdxModePrefixSuffix, nil
	case sDynIdxModedimDataStream:
		return dynIdxModeDataStream, nil
	}
	return dynIdxModePrefixSuffix, errUnsupportedDynamicIndexMappingMode
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

	dimMode, err := parseDIMode(cfg.LogsDynamicIndex.Mode)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, cfg.LogsDynamicIndex.Mode)
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

		index:            index,
		dynamicIndex:     dynamicIndex,
		dynamicIndexMode: dimMode,
		model:            model,
		mode:             cfg.MappingMode(),
		logstashFormat:   cfg.LogstashFormat,
	}, nil
}

func (e *elasticsearchExporter) Shutdown(ctx context.Context) error {
	return e.bulkIndexer.Close(ctx)
}

func (e *elasticsearchExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
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
				if err := e.pushLogRecord(ctx, resource, rl.SchemaUrl(), logs.At(k), scope, ill.SchemaUrl()); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					errs = append(errs, err)
				}
			}
		}
	}

	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushLogRecord(ctx context.Context,
	resource pcommon.Resource,
	resourceSchemaUrl string,
	record plog.LogRecord,
	scope pcommon.InstrumentationScope,
	scopeSchemaUrl string) error {
	fIndex := e.index
	if e.dynamicIndex {
		switch e.dynamicIndexMode {
		case dynIdxModePrefixSuffix:
			prefix := getFromAttributes(indexPrefix, resource, scope, record)
			suffix := getFromAttributes(indexSuffix, resource, scope, record)
			fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
		case dynIdxModeDataStream:
			const otelSuffix = ".otel"
			typ := getFromAttributesWithDefault(dataStreamType, resource, scope, record, defaultDataStreamType)
			dataset := getFromAttributesWithDefault(dataStreamDataset, resource, scope, record, defaultDataStreamDataset)
			namespace := getFromAttributesWithDefault(dataStreamNamespace, resource, scope, record, defaultDataStreamNamespace)

			// The naming convension for datastream is expected to be the following
			// "logs-[dataset].otel-[namespace]"
			if e.mode == MappingOTel && !strings.HasSuffix(dataset, otelSuffix) {
				dataset += otelSuffix
			}
			fIndex = fmt.Sprintf("%s-%s-%s", typ, dataset, namespace)
		}
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeLog(resource, resourceSchemaUrl, record, scope, scopeSchemaUrl)
	if err != nil {
		return fmt.Errorf("failed to encode log event: %w", err)
	}
	return pushDocuments(ctx, fIndex, document, e.bulkIndexer)
}

func (e *elasticsearchExporter) pushTraceData(
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
			scopeSpan := scopeSpans.At(j)
			scope := scopeSpan.Scope()
			spans := scopeSpan.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if err := e.pushTraceRecord(ctx, resource, span, scope); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, err)
				}
			}
		}
	}

	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushTraceRecord(ctx context.Context, resource pcommon.Resource, span ptrace.Span, scope pcommon.InstrumentationScope) error {
	fIndex := e.index
	if e.dynamicIndex {
		prefix := getFromAttributes(indexPrefix, resource, scope, span)
		suffix := getFromAttributes(indexSuffix, resource, scope, span)

		fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeSpan(resource, span, scope)
	if err != nil {
		return fmt.Errorf("Failed to encode trace record: %w", err)
	}
	return pushDocuments(ctx, fIndex, document, e.bulkIndexer)
}
