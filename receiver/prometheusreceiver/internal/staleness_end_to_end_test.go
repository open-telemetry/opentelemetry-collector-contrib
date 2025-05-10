// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

// Test that staleness markers are emitted for timeseries that intermittently disappear.
// This test runs the entire collector and end-to-end scrapes then checks with the
// Prometheus remotewrite exporter that staleness markers are emitted per timeseries.
// See https://github.com/open-telemetry/opentelemetry-collector/issues/3413
func TestStalenessMarkersEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("This test can take a long time")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 1. Setup the server that sends series that intermittently appear and disappear.
	n := &atomic.Uint64{}
	scrapeServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		// Increment the scrape count atomically per scrape.
		i := n.Add(1)

		select {
		case <-ctx.Done():
			return
		default:
		}

		// Alternate metrics per scrape so that every one of
		// them will be reported as stale.
		if i%2 == 0 {
			fmt.Fprintf(rw, `
# HELP jvm_memory_bytes_used Used bytes of a given JVM memory area.
# TYPE jvm_memory_bytes_used gauge
jvm_memory_bytes_used{area="heap"} %.1f`, float64(i))
		} else {
			fmt.Fprintf(rw, `
# HELP jvm_memory_pool_bytes_used Used bytes of a given JVM memory pool.
# TYPE jvm_memory_pool_bytes_used gauge
jvm_memory_pool_bytes_used{pool="CodeHeap 'non-nmethods'"} %.1f`, float64(i))
		}
	}))
	defer scrapeServer.Close()

	serverURL, err := url.Parse(scrapeServer.URL)
	require.NoError(t, err)

	// 2. Set up the Prometheus RemoteWrite endpoint.
	prweUploads := make(chan *prompb.WriteRequest)
	prweServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		// Snappy decode the uploads.
		payload, rerr := io.ReadAll(req.Body)
		assert.NoError(t, rerr)

		recv := make([]byte, len(payload))
		decoded, derr := snappy.Decode(recv, payload)
		assert.NoError(t, derr)

		writeReq := new(prompb.WriteRequest)
		assert.NoError(t, proto.Unmarshal(decoded, writeReq))

		select {
		case <-ctx.Done():
			return
		case prweUploads <- writeReq:
		}
	}))
	defer prweServer.Close()

	// 3. Set the OpenTelemetry Prometheus receiver.
	cfg := fmt.Sprintf(`
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: 'test'
          scrape_interval: 100ms
          static_configs:
            - targets: [%q]

processors:
  batch:
exporters:
  prometheusremotewrite:
    endpoint: %q
    tls:
      insecure: true

service:
  pipelines:
    metrics:
      receivers: [prometheus]
      processors: [batch]
      exporters: [prometheusremotewrite]`, serverURL.Host, prweServer.URL)

	confFile, err := os.CreateTemp(os.TempDir(), "conf-")
	require.NoError(t, err)
	defer os.Remove(confFile.Name())
	_, err = confFile.Write([]byte(cfg))
	require.NoError(t, err)
	// 4. Run the OpenTelemetry Collector.
	receivers, err := receiver.MakeFactoryMap(prometheusreceiver.NewFactory())
	require.NoError(t, err)
	exporters, err := exporter.MakeFactoryMap(prometheusremotewriteexporter.NewFactory())
	require.NoError(t, err)
	processors, err := processor.MakeFactoryMap(batchprocessor.NewFactory())
	require.NoError(t, err)

	factories := otelcol.Factories{
		Receivers:  receivers,
		Exporters:  exporters,
		Processors: processors,
	}

	appSettings := otelcol.CollectorSettings{
		Factories: func() (otelcol.Factories, error) { return factories, nil },
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs:              []string{confFile.Name()},
				ProviderFactories: []confmap.ProviderFactory{fileprovider.NewFactory()},
			},
		},
		BuildInfo: component.BuildInfo{
			Command:     "otelcol",
			Description: "OpenTelemetry Collector",
			Version:     "tests",
		},
		LoggingOptions: []zap.Option{
			// Turn off the verbose logging from the collector.
			zap.WrapCore(func(zapcore.Core) zapcore.Core {
				return zapcore.NewNopCore()
			}),
		},
	}

	app, err := otelcol.NewCollector(appSettings)
	require.NoError(t, err)

	go func() {
		assert.NoError(t, app.Run(context.Background()))
	}()
	defer app.Shutdown()

	// Wait until the collector has actually started.
	for notYetStarted := true; notYetStarted; {
		state := app.GetState()
		switch state {
		case otelcol.StateRunning, otelcol.StateClosed, otelcol.StateClosing:
			notYetStarted = false
		case otelcol.StateStarting:
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 5. Let's wait on 10 fetches.
	var wReqL []*prompb.WriteRequest
	for i := 0; i < 10; i++ {
		wReqL = append(wReqL, <-prweUploads)
	}
	defer cancel()

	// 6. Assert that we encounter the stale markers aka special NaNs for the various time series.
	staleMarkerCount := 0
	totalSamples := 0
	require.NotEmpty(t, wReqL, "Expecting at least one WriteRequest")
	for i, wReq := range wReqL {
		name := fmt.Sprintf("WriteRequest#%d", i)
		require.NotEmpty(t, wReq.Timeseries, "Expecting at least 1 timeSeries for:: "+name)
		for j, ts := range wReq.Timeseries {
			fullName := fmt.Sprintf("%s/TimeSeries#%d", name, j)
			assert.NotEmpty(t, ts.Samples, "Expected at least 1 Sample in:: "+fullName)

			// We are strictly counting series directly included in the scrapes, and no
			// internal timeseries like "up" nor "scrape_seconds" etc.
			metricName := ""
			for _, label := range ts.Labels {
				if label.Name == "__name__" {
					metricName = label.Value
				}
			}
			if !strings.HasPrefix(metricName, "jvm") {
				continue
			}

			for _, sample := range ts.Samples {
				totalSamples++
				if value.IsStaleNaN(sample.Value) {
					staleMarkerCount++
				}
			}
		}
	}

	require.Positive(t, totalSamples, "Expected at least 1 sample")
	// On every alternative scrape the prior scrape will be reported as sale.
	// Expect at least:
	//    * The first scrape will NOT return stale markers
	//    * (N-1 / alternatives) = ((10-1) / 2) = ~40% chance of stale markers being emitted.
	chance := float64(staleMarkerCount) / float64(totalSamples)
	require.GreaterOrEqualf(t, chance, 0.4, "Expected at least one stale marker: %.3f", chance)
}
