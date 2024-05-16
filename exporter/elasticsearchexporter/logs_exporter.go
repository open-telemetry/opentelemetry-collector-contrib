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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type elasticsearchLogsExporter struct {
	logger *zap.Logger

	index          string
	logstashFormat LogstashFormatSettings
	dynamicIndex   bool

	client      *esClientCurrent
	bulkIndexer *esBulkIndexerCurrent
	model       mappingModel
}

func newLogsExporter(logger *zap.Logger, cfg *Config) (*elasticsearchLogsExporter, error) {
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

	indexStr := cfg.LogsIndex
	if cfg.Index != "" {
		indexStr = cfg.Index
	}
	esLogsExp := &elasticsearchLogsExporter{
		logger:      logger,
		client:      client,
		bulkIndexer: bulkIndexer,

		index:          indexStr,
		dynamicIndex:   cfg.LogsDynamicIndex.Enabled,
		model:          model,
		logstashFormat: cfg.LogstashFormat,
	}
	return esLogsExp, nil
}

func (e *elasticsearchLogsExporter) Shutdown(ctx context.Context) error {
	return e.bulkIndexer.Close(ctx)
}

func (e *elasticsearchLogsExporter) logsDataToRequest(ctx context.Context, ld plog.Logs) (exporterhelper.Request, error) {
	req := newRequest(e.bulkIndexer)
	var errs []error
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()
		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			scope := ills.At(j).Scope()
			logs := ills.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				item, err := e.logRecordToItem(ctx, resource, logs.At(k), scope)
				if err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return req, cerr
					}

					errs = append(errs, err)
					continue
				}
				req.add(item)
			}
		}
	}

	return req, errors.Join(errs...)
}

func (e *elasticsearchLogsExporter) logRecordToItem(ctx context.Context, resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope) (bulkIndexerItem, error) {
	fIndex := e.index
	if e.dynamicIndex {
		prefix := getFromAttributes(indexPrefix, resource, scope, record)
		suffix := getFromAttributes(indexSuffix, resource, scope, record)

		fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return bulkIndexerItem{}, err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeLog(resource, record, scope)
	if err != nil {
		return bulkIndexerItem{}, fmt.Errorf("Failed to encode log event: %w", err)
	}
	return bulkIndexerItem{
		Index: fIndex,
		Body:  document,
	}, nil
}
