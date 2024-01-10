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

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type elasticsearchLogsExporter struct {
	logger *zap.Logger

	index          string
	logstashFormat LogstashFormatSettings
	dynamicIndex   bool
	maxAttempts    int

	client      *esClientCurrent
	bulkIndexer esBulkIndexerCurrent
	model       mappingModel
}

var retryOnStatus = []int{500, 502, 503, 504, 429}

const createAction = "create"

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

	maxAttempts := 1
	if cfg.Retry.Enabled {
		maxAttempts = cfg.Retry.MaxRequests
	}

	model := &encodeModel{dedup: cfg.Mapping.Dedup, dedot: cfg.Mapping.Dedot}

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
		maxAttempts:    maxAttempts,
		model:          model,
		logstashFormat: cfg.LogstashFormat,
	}
	return esLogsExp, nil
}

func (e *elasticsearchLogsExporter) Shutdown(ctx context.Context) error {
	return e.bulkIndexer.Close(ctx)
}

func (e *elasticsearchLogsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
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
				if err := e.pushLogRecord(ctx, resource, logs.At(k), scope); err != nil {
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

func (e *elasticsearchLogsExporter) pushLogRecord(ctx context.Context, resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope) error {
	fIndex := e.index
	if e.dynamicIndex {
		prefix := getFromBothResourceAndAttribute(indexPrefix, resource, record)
		suffix := getFromBothResourceAndAttribute(indexSuffix, resource, record)

		fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeLog(resource, record, scope)
	if err != nil {
		return fmt.Errorf("Failed to encode log event: %w", err)
	}
	return pushDocuments(ctx, e.logger, fIndex, document, e.bulkIndexer, e.maxAttempts)
}
