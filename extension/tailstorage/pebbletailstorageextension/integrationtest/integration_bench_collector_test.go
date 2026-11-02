// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/nopexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
)

func setupCollectorBenchmark(b *testing.B, backend string, shape benchmarkShape) (string, func(), string) {
	ctx := b.Context()
	endpoint := availableLocalAddress(b)

	cfgPath, storageDir := writeCollectorBenchmarkConfig(b, backend, endpoint, shape)
	factories := collectorBenchmarkFactories(b)
	app := newCollectorForBenchmark(b, cfgPath, factories)

	var wg sync.WaitGroup
	wg.Go(func() {
		_ = app.Run(ctx)
	})
	waitForCollectorRunning(app)

	return endpoint, func() {
		app.Shutdown()
		wg.Wait()
	}, storageDir
}

func enableTailStorageFeatureGateForBenchmark(tb testing.TB) {
	tb.Helper()
	prev := isTailStorageFeatureGateEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(tailStorageFeatureGateID, true))
	tb.Cleanup(func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(tailStorageFeatureGateID, prev))
	})
}

func isTailStorageFeatureGateEnabled() bool {
	enabled := false
	featuregate.GlobalRegistry().VisitAll(func(gate *featuregate.Gate) {
		if gate.ID() == tailStorageFeatureGateID {
			enabled = gate.IsEnabled()
		}
	})
	return enabled
}

func collectorBenchmarkFactories(b *testing.B) otelcol.Factories {
	var err error
	factories := otelcol.Factories{
		Telemetry: otelconftelemetry.NewFactory(),
	}
	factories.Receivers, err = otelcol.MakeFactoryMap[receiver.Factory](
		otlpreceiver.NewFactory(),
	)
	require.NoError(b, err)
	factories.Processors, err = otelcol.MakeFactoryMap[processor.Factory](
		tailsamplingprocessor.NewFactory(),
	)
	require.NoError(b, err)
	factories.Exporters, err = otelcol.MakeFactoryMap[exporter.Factory](
		nopexporter.NewFactory(),
	)
	require.NoError(b, err)
	factories.Extensions, err = otelcol.MakeFactoryMap[extension.Factory](
		pebbletailstorageextension.NewFactory(),
	)
	require.NoError(b, err)
	return factories
}

func newCollectorForBenchmark(b *testing.B, cfgPath string, factories otelcol.Factories) *otelcol.Collector {
	app, err := otelcol.NewCollector(otelcol.CollectorSettings{
		Factories: func() (otelcol.Factories, error) { return factories, nil },
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs:              []string{cfgPath},
				ProviderFactories: []confmap.ProviderFactory{fileprovider.NewFactory()},
			},
		},
		BuildInfo: component.BuildInfo{
			Command:     "otelcol",
			Description: "collector benchmark",
			Version:     "tests",
		},
	})
	require.NoError(b, err)
	return app
}

func waitForCollectorRunning(app *otelcol.Collector) {
	for {
		switch app.GetState() {
		case otelcol.StateRunning, otelcol.StateClosed, otelcol.StateClosing:
			return
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func writeCollectorBenchmarkConfig(b *testing.B, backend, endpoint string, shape benchmarkShape) (string, string) {
	policyName := shape.policy.nameForBackend(backend)
	policyCfg := shape.policy.yaml()

	var cfg string
	storageDir := ""
	if backend == "pebble" {
		storageDir = filepath.Join(b.TempDir(), "pebble")
		cfg = fmt.Sprintf(`
extensions:
  pebble_tail_storage/bench:
    directory: %s
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: %s
processors:
  tail_sampling:
    sampling_strategy: span-ingest
    decision_wait: %s
    num_traces: %d
    block_on_overflow: true
    tail_storage: pebble_tail_storage/bench
    policies:
      - name: %s
%s
exporters:
  nop:
service:
  telemetry:
    logs:
      level: error
      output_paths: [/dev/null]
      error_output_paths: [/dev/null]
  extensions: [pebble_tail_storage/bench]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [tail_sampling]
      exporters: [nop]
`, storageDir, endpoint, shape.decisionWait, shape.numTraces, policyName, policyCfg)
	} else {
		cfg = fmt.Sprintf(`
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: %s
processors:
  tail_sampling:
    sampling_strategy: span-ingest
    decision_wait: %s
    num_traces: %d
    block_on_overflow: true
    policies:
      - name: %s
%s
exporters:
  nop:
service:
  telemetry:
    logs:
      level: error
      output_paths: [/dev/null]
      error_output_paths: [/dev/null]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [tail_sampling]
      exporters: [nop]
`, endpoint, shape.decisionWait, shape.numTraces, policyName, policyCfg)
	}

	path := filepath.Join(b.TempDir(), "collector_bench.yaml")
	require.NoError(b, os.WriteFile(path, []byte(cfg), 0o600))
	return path, storageDir
}

func availableLocalAddress(b *testing.B) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(b, err)
	defer func() { _ = l.Close() }()
	return l.Addr().String()
}
