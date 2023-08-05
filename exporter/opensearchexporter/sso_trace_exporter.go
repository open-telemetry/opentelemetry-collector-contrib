// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"net/http"

	"github.com/opensearch-project/opensearch-go/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type ssoTracesExporter struct {
	client       *opensearch.Client
	Namespace    string
	Dataset      string
	httpSettings confighttp.HTTPClientSettings
	telemetry    component.TelemetrySettings
}

func newSSOTracesExporter(cfg *Config, set exporter.CreateSettings) (*ssoTracesExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &ssoTracesExporter{
		telemetry:    set.TelemetrySettings,
		Namespace:    cfg.Namespace,
		Dataset:      cfg.Dataset,
		httpSettings: cfg.HTTPClientSettings,
	}, nil
}

func (s *ssoTracesExporter) Start(_ context.Context, host component.Host) error {
	httpClient, err := s.httpSettings.ToClient(host, s.telemetry)
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
	indexer := newTraceBulkIndexer(s.Dataset, s.Namespace)
	startErr := indexer.start(s.client)
	if startErr != nil {
		return startErr
	}
	indexer.submit(ctx, td)
	indexer.close(ctx)
	return indexer.joinedError()
}

func newOpenSearchClient(endpoint string, httpClient *http.Client, logger *zap.Logger) (*opensearch.Client, error) {
	transport := httpClient.Transport
	return opensearch.NewClient(opensearch.Config{
		Transport: transport,

		// configure connection setup
		Addresses:    []string{endpoint},
		DisableRetry: true,

		// configure internal metrics reporting and logging
		EnableMetrics:     false, // TODO
		EnableDebugLogger: false, // TODO
		Logger:            newClientLogger(logger),
	})
}
