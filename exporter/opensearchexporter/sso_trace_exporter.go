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

const (
	// defaultNamespace value is used as SSOTracesExporter.Namespace when component.Config.Namespace is not set.
	defaultNamespace = "namespace"

	// defaultDataset value is used as SSOTracesExporter.Dataset when component.Config.Dataset is not set.
	defaultDataset = "default"
)

type SSOTracesExporter struct {
	client       *opensearch.Client
	Namespace    string
	Dataset      string
	httpSettings confighttp.HTTPClientSettings
	telemetry    component.TelemetrySettings
}

func newSSOTracesExporter(cfg *Config, set exporter.CreateSettings) (*SSOTracesExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &SSOTracesExporter{
		telemetry:    set.TelemetrySettings,
		Namespace:    defaultIfEmpty(cfg.Namespace, defaultNamespace),
		Dataset:      defaultIfEmpty(cfg.Dataset, defaultDataset),
		httpSettings: cfg.HTTPClientSettings,
	}, nil
}

func (s *SSOTracesExporter) Start(_ context.Context, host component.Host) error {
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

func (s *SSOTracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	indexer := newTraceBulkIndexer(s.Dataset, s.Namespace)
	startErr := indexer.Start(s.client)
	if startErr != nil {
		return startErr
	}
	indexer.Submit(ctx, td)
	indexer.Close(ctx)
	return indexer.JoinedError()
}

func defaultIfEmpty(value string, def string) string {
	if value == "" {
		return def
	}
	return value
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
		Logger:            (*clientLogger)(logger),
	})
}
