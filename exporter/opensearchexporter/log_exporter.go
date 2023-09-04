// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"

	"github.com/opensearch-project/opensearch-go/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
)

type logExporter struct {
	client       *opensearch.Client
	index        string
	model        mappingModel
	httpSettings confighttp.HTTPClientSettings
	telemetry    component.TelemetrySettings
}

func newLogExporter(cfg *Config, set exporter.CreateSettings) (*logExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	model := &encodeModel{
		dedup:             true,
		dedot:             false,
		flattenAttributes: false,
		timestampField:    cfg.MappingsSettings.TimestampField,
		unixTime:          cfg.MappingsSettings.UnixTimestamp,
	}
	if cfg.MappingsSettings.Mode == MappingFlattenAttributes.String() {
		model.flattenAttributes = true
	}

	return &logExporter{
		index:        cfg.LogsIndex,
		telemetry:    set.TelemetrySettings,
		httpSettings: cfg.HTTPClientSettings,
		model:        model,
	}, nil
}

func (l *logExporter) Start(_ context.Context, host component.Host) error {
	httpClient, err := l.httpSettings.ToClient(host, l.telemetry)
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
	indexer := newLogBulkIndexer(l.index, l.model)
	startErr := indexer.start(l.client)
	if startErr != nil {
		return startErr
	}
	indexer.submit(ctx, ld)
	indexer.close(ctx)
	return indexer.joinedError()
}
