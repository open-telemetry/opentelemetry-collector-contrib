// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/integrationtest"

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
)

func TestIntegration(t *testing.T) {
	tests := []struct {
		name               string
		featureGateEnabled bool
	}{
		{
			name:               "with feature gate enabled",
			featureGateEnabled: true,
		},
		{
			name:               "with feature gate disabled",
			featureGateEnabled: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Set up mock Datadog server
			// See also https://github.com/DataDog/datadog-agent/blob/49c16e0d4deab396626238fa1d572b684475a53f/cmd/trace-agent/test/backend.go
			apmstatsRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.APMStatsEndpoint, ReqChan: make(chan []byte)}
			tracesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.TraceEndpoint, ReqChan: make(chan []byte)}
			server := testutil.DatadogServerMock(apmstatsRec.HandlerFunc, tracesRec.HandlerFunc)
			defer server.Close()

			// 2. Start in-process collector
			factories := getIntegrationTestComponents(t)
			app, confFilePath := getIntegrationTestCollector(t, server.URL, factories)
			if tt.featureGateEnabled {
				err := featuregate.GlobalRegistry().Set(datadog.ConnectorPerformanceFeatureGate.ID(), true)
				assert.NoError(t, err)
				defer func() {
					_ = featuregate.GlobalRegistry().Set(datadog.ConnectorPerformanceFeatureGate.ID(), false)
				}()
			}
			go func() {
				assert.NoError(t, app.Run(context.Background()))
			}()
			defer app.Shutdown()
			defer os.Remove(confFilePath)
			waitForReadiness(app)

			// 3. Generate and send traces
			sendTraces(t)

			// 4. Validate traces and APM stats from the mock server
			var spans []*pb.Span
			var stats []*pb.ClientGroupedStats

			// 5 sampled spans + APM stats on 10 spans are sent to datadog exporter
			for len(spans) < 5 || len(stats) < 10 {
				select {
				case tracesBytes := <-tracesRec.ReqChan:
					gz := getGzipReader(t, tracesBytes)
					slurp, err := io.ReadAll(gz)
					require.NoError(t, err)
					var traces pb.AgentPayload
					require.NoError(t, proto.Unmarshal(slurp, &traces))
					for _, tps := range traces.TracerPayloads {
						for _, chunks := range tps.Chunks {
							spans = append(spans, chunks.Spans...)
							for _, span := range chunks.Spans {
								assert.Equal(t, span.Meta["_dd.stats_computed"], "true")
							}
						}
					}

				case apmstatsBytes := <-apmstatsRec.ReqChan:
					gz := getGzipReader(t, apmstatsBytes)
					var spl pb.StatsPayload
					require.NoError(t, msgp.Decode(gz, &spl))
					for _, csps := range spl.Stats {
						for _, csbs := range csps.Stats {
							stats = append(stats, csbs.Stats...)
							for _, stat := range csbs.Stats {
								assert.True(t, strings.HasPrefix(stat.Resource, "TestSpan"))
								assert.Equal(t, stat.Hits, uint64(1))
								assert.Equal(t, stat.TopLevelHits, uint64(1))
							}
						}
					}
				}
			}

			// Verify we don't receive more than the expected numbers
			assert.Len(t, spans, 5)
			assert.Len(t, stats, 10)
		})
	}
}

func getIntegrationTestComponents(t *testing.T) otelcol.Factories {
	var (
		factories otelcol.Factories
		err       error
	)
	factories.Receivers, err = receiver.MakeFactoryMap(
		[]receiver.Factory{
			otlpreceiver.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Processors, err = processor.MakeFactoryMap(
		[]processor.Factory{
			batchprocessor.NewFactory(),
			tailsamplingprocessor.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Connectors, err = connector.MakeFactoryMap(
		[]connector.Factory{
			datadogconnector.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Exporters, err = exporter.MakeFactoryMap(
		[]exporter.Factory{
			datadogexporter.NewFactory(),
			debugexporter.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	return factories
}

func getIntegrationTestCollector(t *testing.T, url string, factories otelcol.Factories) (*otelcol.Collector, string) {
	cfg := fmt.Sprintf(`
receivers:
  otlp:
    protocols:
      http:
        endpoint: "localhost:4318"
      grpc:
        endpoint: "localhost:4317"

processors:
  batch:
    send_batch_size: 10
    timeout: 5s
  tail_sampling:
    decision_wait: 1s
    policies: [
        {
          name: sample_flag,
          type: boolean_attribute,
          boolean_attribute: { key: sampled, value: true },
        }
      ]

connectors:
  datadog/connector:

exporters:
  debug:
    verbosity: detailed
  datadog:
    api:
      key: "key"
    tls:
      insecure_skip_verify: true
    host_metadata:
      enabled: false
    traces:
      endpoint: %q
      trace_buffer: 10
    metrics:
      endpoint: %q

service:
  telemetry:
    metrics:
      level: none
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [datadog/connector]
    traces/2: # this pipeline uses sampling
      receivers: [datadog/connector]
      processors: [tail_sampling, batch]
      exporters: [datadog, debug]
    metrics:
      receivers: [datadog/connector]
      processors: [batch]
      exporters: [datadog, debug]`, url, url)

	confFile, err := os.CreateTemp(os.TempDir(), "conf-")
	require.NoError(t, err)
	_, err = confFile.Write([]byte(cfg))
	require.NoError(t, err)
	_, err = otelcoltest.LoadConfigAndValidate(confFile.Name(), factories)
	require.NoError(t, err, "All yaml config must be valid.")

	fmp := fileprovider.New()
	configProvider, err := otelcol.NewConfigProvider(
		otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs:      []string{confFile.Name()},
				Providers: map[string]confmap.Provider{fmp.Scheme(): fmp},
			},
		})
	require.NoError(t, err)

	appSettings := otelcol.CollectorSettings{
		Factories:      func() (otelcol.Factories, error) { return factories, nil },
		ConfigProvider: configProvider,
		BuildInfo: component.BuildInfo{
			Command:     "otelcol",
			Description: "OpenTelemetry Collector",
			Version:     "tests",
		},
	}

	app, err := otelcol.NewCollector(appSettings)
	require.NoError(t, err)
	return app, confFile.Name()
}

func waitForReadiness(app *otelcol.Collector) {
	for notYetStarted := true; notYetStarted; {
		state := app.GetState()
		switch state {
		case otelcol.StateRunning, otelcol.StateClosed, otelcol.StateClosing:
			notYetStarted = false
		case otelcol.StateStarting:
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func sendTraces(t *testing.T) {
	ctx := context.Background()

	// Set up OTel-Go SDK and exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	require.NoError(t, err)
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)
	defer func() {
		require.NoError(t, tracerProvider.Shutdown(ctx))
	}()

	tracer := otel.Tracer("test-tracer")
	for i := 0; i < 10; i++ {
		_, span := tracer.Start(ctx, fmt.Sprintf("TestSpan%d", i))
		// Only sample 5 out of the 10 spans
		if i < 5 {
			span.SetAttributes(attribute.Bool("sampled", true))
		}
		span.End()
	}
	time.Sleep(1 * time.Second)
}

func getGzipReader(t *testing.T, reqBytes []byte) io.Reader {
	buf := bytes.NewBuffer(reqBytes)
	reader, err := gzip.NewReader(buf)
	require.NoError(t, err)
	return reader
}
