// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package opensearchexporter contains an opentelemetry-collector exporter
// for OpenSearch.
package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type opensearchLogsExporter struct {
	logger *zap.Logger

	index       string
	maxAttempts int

	client      *osClientCurrent
	bulkIndexer osBulkIndexerCurrent
	model       mappingModel
}

var retryOnStatus = []int{500, 502, 503, 504, 429}

const createAction = "create"

func newLogsExporter(logger *zap.Logger, cfg *Config) (*opensearchLogsExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newOpenSearchClient(logger, cfg)
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

	// TODO: Apply encoding and field mapping settings.
	model := &encodeModel{dedup: true, dedot: false}

	indexStr := cfg.LogsIndex

	logsExp := &opensearchLogsExporter{
		logger:      logger,
		client:      client,
		bulkIndexer: bulkIndexer,
		index:       indexStr,
		maxAttempts: maxAttempts,
		model:       model,
	}
	return logsExp, nil
}

func (e *opensearchLogsExporter) Shutdown(ctx context.Context) error {
	return e.bulkIndexer.Close(ctx)
}

func (e *opensearchLogsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	var errs []error

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()
		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				if err := e.pushLogRecord(ctx, resource, logs.At(k)); err != nil {
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

func (e *opensearchLogsExporter) pushLogRecord(ctx context.Context, resource pcommon.Resource, record plog.LogRecord) error {
	document, err := e.model.encodeLog(resource, record)
	if err != nil {
		return fmt.Errorf("Failed to encode log event: %w", err)
	}
	return pushDocuments(ctx, e.logger, e.index, document, e.bulkIndexer, e.maxAttempts)
}
