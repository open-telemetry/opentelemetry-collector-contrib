// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"strings"

	"github.com/opensearch-project/opensearch-go/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
)

type logExporter struct {
	client       *opensearch.Client
	Index        string
	bulkAction   string
	model        mappingModel
	httpSettings confighttp.ClientConfig
	telemetry    component.TelemetrySettings
}

func newLogExporter(cfg *Config, set exporter.Settings) *logExporter {
	model := &encodeModel{
		dedup:             cfg.Dedup,
		dedot:             cfg.Dedot,
		sso:               cfg.Mode == MappingSS4O.String(),
		flattenAttributes: cfg.Mode == MappingFlattenAttributes.String(),
		timestampField:    cfg.TimestampField,
		unixTime:          cfg.UnixTimestamp,
		dataset:           cfg.Dataset,
		namespace:         cfg.Namespace,
	}

	return &logExporter{
		telemetry:    set.TelemetrySettings,
		Index:        getIndexName(cfg.Dataset, cfg.Namespace, cfg.LogsIndex),
		bulkAction:   cfg.BulkAction,
		httpSettings: cfg.ClientConfig,
		model:        model,
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
	indexer := newLogBulkIndexer(l.Index, l.bulkAction, l.model)
	startErr := indexer.start(l.client)
	if startErr != nil {
		return startErr
	}
	indexer.submit(ctx, ld)
	indexer.close(ctx)
	return indexer.joinedError()
}

func getIndexName(dataset, namespace, index string) string {
	if len(index) != 0 {
		return index
	}

	return strings.Join([]string{"ss4o_logs", dataset, namespace}, "-")
}
