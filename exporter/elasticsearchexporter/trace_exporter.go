// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	index       string
	maxAttempts int

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

	// TODO: Apply encoding and field mapping settings.
	model := &encodeModel{dedup: true, dedot: false}

	return &elasticsearchTracesExporter{
		logger:      logger,
		client:      client,
		bulkIndexer: bulkIndexer,

		index:       cfg.TracesIndex,
		maxAttempts: maxAttempts,
		model:       model,
	}, nil
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
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				if err := e.pushTraceRecord(ctx, resource, spans.At(k)); err != nil {
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

func (e *elasticsearchTracesExporter) pushTraceRecord(ctx context.Context, resource pcommon.Resource, span ptrace.Span) error {
	document, err := e.model.encodeSpan(resource, span)
	if err != nil {
		return fmt.Errorf("Failed to encode trace record: %w", err)
	}
	return pushDocuments(ctx, e.logger, e.index, document, e.bulkIndexer, e.maxAttempts)
}
