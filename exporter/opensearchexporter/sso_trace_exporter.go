// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"net/http"
	"time"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type ssoTracesExporter struct {
	client        *opensearchapi.Client
	Namespace     string
	Dataset       string
	bulkAction    string
	model         mappingModel
	httpSettings  confighttp.ClientConfig
	telemetry     component.TelemetrySettings
	config        *Config
	indexResolver *indexResolver
}

func newSSOTracesExporter(cfg *Config, set exporter.Settings) *ssoTracesExporter {
	model := &encodeModel{
		dataset:   cfg.Dataset,
		namespace: cfg.Namespace,
	}

	return &ssoTracesExporter{
		telemetry:     set.TelemetrySettings,
		Namespace:     cfg.Namespace,
		Dataset:       cfg.Dataset,
		bulkAction:    cfg.BulkAction,
		model:         model,
		httpSettings:  cfg.ClientConfig,
		config:        cfg,
		indexResolver: newIndexResolver(),
	}
}

func (s *ssoTracesExporter) Start(ctx context.Context, host component.Host) error {
	httpClient, err := s.httpSettings.ToClient(ctx, host, s.telemetry)
	if err != nil {
		return err
	}

	client, err := newOpenSearchClient(s.httpSettings.Endpoint, httpClient, s.telemetry.Logger)
	if err != nil {
		return err
	}

	s.client = client
	return nil
}

func (s *ssoTracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	// Resolve index name using the common index resolver
	// Use collector time for consistency with logs and elasticsearch exporter
	traceTimestamp := time.Now()
	indexName := s.indexResolver.ResolveTraceIndex(s.config, td, traceTimestamp)

	indexer := newTraceBulkIndexer(indexName, s.bulkAction, s.model)
	startErr := indexer.start(s.client)
	if startErr != nil {
		return startErr
	}
	indexer.submit(ctx, td)
	indexer.close(ctx)
	return indexer.joinedError()
}

func newOpenSearchClient(endpoint string, httpClient *http.Client, logger *zap.Logger) (*opensearchapi.Client, error) {
	return opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Transport: httpClient.Transport,

			// configure connection setup
			Addresses:    []string{endpoint},
			DisableRetry: true,

			// configure internal metrics reporting and logging
			EnableMetrics:     false, // TODO
			EnableDebugLogger: false, // TODO
			Logger:            newClientLogger(logger),
		},
	})
}
