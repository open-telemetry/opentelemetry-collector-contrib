// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
)

type otelColConfig struct {
	GRPCEndpoint  string
	StorageDir    string
	ESEndpoint    string
	ESLogsIndex   string
	FlushInterval time.Duration
	Debug         bool
}

// otelCol represents the otel collector instance created for testing the ES
// exporter with otlp receivers.
type otelCol struct {
	mu       sync.Mutex
	col      *otelcol.Collector
	settings otelcol.CollectorSettings
}

// newTestCollector creates a new instance of OTEL collector. The collector wraps
// the real OTEL collector and provides testing functions on top of it.
func newTestCollector(t testing.TB, cfg otelColConfig) *otelCol {
	var (
		err       error
		factories otelcol.Factories
	)
	factories.Receivers, err = receiver.MakeFactoryMap(
		otlpreceiver.NewFactory(),
	)
	require.NoError(t, err)
	factories.Extensions, err = extension.MakeFactoryMap(
		filestorage.NewFactory(),
	)
	require.NoError(t, err)
	factories.Exporters, err = exporter.MakeFactoryMap(
		elasticsearchexporter.NewFactory(),
		debugexporter.NewFactory(),
	)
	require.NoError(t, err)

	cfgFile, err := getCollectorCfg(cfg)
	require.NoError(t, err)
	_, err = otelcoltest.LoadConfigAndValidate(cfgFile.Name(), factories)
	require.NoError(t, err)

	fp := fileprovider.NewWithSettings(confmap.ProviderSettings{})
	cfgProviderSettings := otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs:      []string{cfgFile.Name()},
			Providers: map[string]confmap.Provider{fp.Scheme(): fp},
		},
	}
	collectorSettings := otelcol.CollectorSettings{
		Factories:              func() (otelcol.Factories, error) { return factories, nil },
		ConfigProviderSettings: cfgProviderSettings,
		BuildInfo: component.BuildInfo{
			Command: "otelcol",
			Version: "v0.0.0",
		},
	}
	collector, err := otelcol.NewCollector(collectorSettings)
	require.NoError(t, err)

	wrappedCollector := &otelCol{
		col:      collector,
		settings: collectorSettings,
	}
	t.Cleanup(wrappedCollector.Shutdown)
	return wrappedCollector
}

// Recreate recreates the collector with the same configuration. Note
// that after recreating the collector `Run` should be called to get the
// collector running again.
func (otel *otelCol) Recreate() error {
	otel.mu.Lock()
	defer otel.mu.Unlock()

	if otel.col != nil {
		otel.col.Shutdown()
	}
	newCollector, err := otelcol.NewCollector(otel.settings)
	if err != nil {
		return fmt.Errorf("failed to restart collector: %w", err)
	}
	otel.col = newCollector
	return nil
}

// Run wraps the original collector run to protect the updates on the
// collector variable.
func (otel *otelCol) Run(ctx context.Context) error {
	otel.mu.Lock()
	collector := otel.col
	otel.mu.Unlock()

	if collector == nil {
		return fmt.Errorf("nil collector")
	}

	return collector.Run(ctx)
}

// IsRunning checks if the otel collector is currently running or not.
func (otel *otelCol) IsRunning() bool {
	otel.mu.Lock()
	defer otel.mu.Unlock()

	if otel.col == nil {
		return false
	}
	return otel.col.GetState() == otelcol.StateRunning
}

// Shutdown shuts down the otel collector. This test wrapper allows for
// restarting the collector after shutdown by creating a new otel collector
// with the same configuration as the original collector.
func (otel *otelCol) Shutdown() {
	otel.mu.Lock()
	defer otel.mu.Unlock()

	if otel.col != nil {
		otel.col.Shutdown()
		otel.col = nil
	}
}

const cfgTemplate = `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: {{.GRPCEndpoint}}

exporters:
  debug:
    verbosity: {{if .Debug}} detailed {{else}} basic {{end}}
  elasticsearch:
    endpoints: [ {{.ESEndpoint}} ]
    logs_index: {{.ESLogsIndex}}
    sending_queue:
      enabled: true
      storage: file_storage/elasticsearchexporter
      num_consumers: 1
      queue_size: 10000000
    flush:
      interval: {{.FlushInterval}}
    retry:
      enabled: true
      max_requests: 10000

extensions:
  file_storage/elasticsearchexporter:
    directory: {{.StorageDir}}

service:
  extensions: [file_storage/elasticsearchexporter]
  pipelines:
    logs:
      receivers: [otlp]
      processors: []
      exporters: [elasticsearch, debug]
`

func getCollectorCfg(cfg otelColConfig) (*os.File, error) {
	cfgFile, err := os.CreateTemp(cfg.StorageDir, "otelconf-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL configuration file: %w", err)
	}
	tmpl, err := template.New("otel-config").Parse(cfgTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL collector configuration template: %w", err)
	}
	if err := tmpl.Execute(cfgFile, cfg); err != nil {
		return nil, fmt.Errorf("failed to create OTEL collector configuration: %w", err)
	}
	return cfgFile, nil
}
