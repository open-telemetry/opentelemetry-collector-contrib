// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/pool"
)

type logExporter struct {
	client        *opensearchapi.Client
	Index         string
	bulkAction    string
	model         mappingModel
	httpSettings  confighttp.ClientConfig
	telemetry     component.TelemetrySettings
	config        *Config
	indexResolver *indexResolver
}

func newLogExporter(cfg *Config, set exporter.Settings) *logExporter {
	var model mappingModel
	if cfg.Mode == MappingBodyMap.String() {
		model = &bodyMapMappingModel{
			bufferPool: pool.NewBufferPool(),
		}
	} else {
		model = &encodeModel{
			dedup:             cfg.Dedup,
			dedot:             cfg.Dedot,
			sso:               cfg.Mode == MappingSS4O.String(),
			flattenAttributes: cfg.Mode == MappingFlattenAttributes.String(),
			timestampField:    cfg.TimestampField,
			unixTime:          cfg.UnixTimestamp,
			dataset:           cfg.Dataset,
			namespace:         cfg.Namespace,
		}
	}

	return &logExporter{
		telemetry:     set.TelemetrySettings,
		bulkAction:    cfg.BulkAction,
		httpSettings:  cfg.ClientConfig,
		model:         model,
		config:        cfg,
		indexResolver: newIndexResolver(),
	}
}

func (l *logExporter) Start(ctx context.Context, host component.Host) error {
	httpClient, err := l.httpSettings.ToClient(ctx, host, l.telemetry)
	if err != nil {
		return err
	}

	client, err := newOpenSearchClient(l.httpSettings.Endpoint, httpClient, l.telemetry.Logger)
	if err != nil {
		return err
	}

	l.client = client
	return nil
}

func (l *logExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	indexer := newLogBulkIndexer("", l.bulkAction, l.model)
	startErr := indexer.start(l.client)
	if startErr != nil {
		return startErr
	}

	// Resolve index name using the common index resolver
	logTimestamp := time.Now() // Replace with actual log timestamp extraction
	indexName := l.indexResolver.ResolveLogIndex(l.config, ld, logTimestamp)

	indexer.index = indexName
	indexer.submit(ctx, ld)
	indexer.close(ctx)
	return indexer.joinedError()
}
