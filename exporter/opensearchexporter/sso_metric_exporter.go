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
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type metricExporter struct {
	client        *opensearchapi.Client
	bulkAction    string
	model         mappingModel
	httpSettings  confighttp.ClientConfig
	telemetry     component.TelemetrySettings
	config        *Config
	indexResolver *indexResolver
}

func newMetricExporter(cfg *Config, set exporter.Settings) *metricExporter {
	model := &encodeModel{
		sso:       cfg.MappingsSettings.Mode == MappingSS4O.String(),
		otelV1:    cfg.MappingsSettings.Mode == MappingOTelV1.String(),
		dataset:   cfg.Dataset,
		namespace: cfg.Namespace,
	}

	defaultPrefix := "ss4o_metrics"
	dataset := cfg.Dataset
	namespace := cfg.Namespace
	if cfg.MappingsSettings.Mode == MappingOTelV1.String() {
		defaultPrefix = "otel-v1-metrics"
		dataset = ""
		namespace = ""
	}

	return &metricExporter{
		telemetry:     set.TelemetrySettings,
		bulkAction:    cfg.BulkAction,
		httpSettings:  cfg.ClientConfig,
		model:         model,
		config:        cfg,
		indexResolver: newIndexResolver(defaultPrefix, dataset, namespace),
	}
}

func (m *metricExporter) Start(ctx context.Context, host component.Host) error {
	httpClient, err := m.httpSettings.ToClient(ctx, host.GetExtensions(), m.telemetry)
	if err != nil {
		return err
	}

	client, err := newOpenSearchClient(m.httpSettings.Endpoint, httpClient, m.telemetry.Logger)
	if err != nil {
		return err
	}

	m.client = client

	return nil
}

func (m *metricExporter) pushMetricData(ctx context.Context, md pmetric.Metrics) error {
	indexer := newMetricBulkIndexer(m.bulkAction, m.model, m.config.Pipeline)
	startErr := indexer.start(m.client)
	if startErr != nil {
		return startErr
	}

	// Use timestamp for index resolution
	metricTimestamp := time.Now()
	indexer.submit(ctx, md, m.indexResolver, m.config, metricTimestamp)
	indexer.close(ctx)
	return indexer.joinedError()
}
