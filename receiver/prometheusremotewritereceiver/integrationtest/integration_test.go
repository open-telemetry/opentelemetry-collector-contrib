// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/integrationtest"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	promconfig "github.com/prometheus/prometheus/config"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"
)

func TestIntegration(t *testing.T) {
	port := "48999"  // Use a port below the dynamic range and not in excluded ranges
	endpoint := fmt.Sprintf("0.0.0.0:%s", port)
	t.Setenv("PRW_ENDPOINT", endpoint)

	factories := getIntegrationTestComponents(t)
	app := getIntegrationTestCollector(t, "integration_test_panic_config.yaml", factories)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.Run(context.Background())
	}()
	defer func() {
		app.Shutdown()
		wg.Wait()
	}()

	waitForReadiness(app)

	sendMetrics(t, port)

	// TODO: Verify that data was processed
}

// getIntegrationTestComponents returns the component factories needed for the integration test
func getIntegrationTestComponents(t *testing.T) otelcol.Factories {
	var (
		factories otelcol.Factories
		err       error
	)

	factories.Receivers, err = otelcol.MakeFactoryMap[receiver.Factory](
		[]receiver.Factory{
			prometheusremotewritereceiver.NewFactory(),
		}...,
	)
	require.NoError(t, err)

	factories.Processors, err = otelcol.MakeFactoryMap[processor.Factory](
		[]processor.Factory{}...,
	)
	require.NoError(t, err)

	factories.Connectors, err = otelcol.MakeFactoryMap[connector.Factory](
		[]connector.Factory{}...,
	)
	require.NoError(t, err)

	factories.Exporters, err = otelcol.MakeFactoryMap[exporter.Factory](
		[]exporter.Factory{
			debugexporter.NewFactory(),
		}...,
	)
	require.NoError(t, err)

	return factories
}

func getIntegrationTestCollector(t *testing.T, cfgFile string, factories otelcol.Factories) *otelcol.Collector {
	_, err := otelcoltest.LoadConfigAndValidate(cfgFile, factories)
	require.NoError(t, err, "Configuration must be valid")

	appSettings := otelcol.CollectorSettings{
		Factories: func() (otelcol.Factories, error) { return factories, nil },
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs:              []string{cfgFile},
				ProviderFactories: []confmap.ProviderFactory{fileprovider.NewFactory(), envprovider.NewFactory()},
			},
		},
		BuildInfo: component.BuildInfo{
			Command:     "otelcol",
			Description: "OpenTelemetry Collector",
			Version:     "tests",
		},
	}

	app, err := otelcol.NewCollector(appSettings)
	require.NoError(t, err)

	return app
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

// Sends data to the prometheus remote write receiver
// https://prometheus.io/docs/specs/prw/remote_write_spec_2_0/
func sendMetrics(t *testing.T, port string) {
	url := fmt.Sprintf("http://localhost:%s/api/v1/write", port)

	// Create sample Prometheus Remote Write v2 data
	sampleData := &writev2.Request{
		Symbols: []string{
			"",
			"__name__", "test",
			"job", "integration-test",
			"instance", "test-server",
		},
		Timeseries: []writev2.TimeSeries{
			{
				Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
				LabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
				Samples:    []writev2.Sample{{Value: 5.5, Timestamp: time.Now().UnixMilli()}},
			},
		},
	}

	pBuf := proto.NewBuffer(nil)
	err := pBuf.Marshal(sampleData)
	require.NoError(t, err)

	compressed := snappy.Encode(nil, pBuf.Bytes())

	for i := 1; i <= 3; i++ {
		t.Logf("Sending Prometheus remote write request %d", i)

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(compressed))
		require.NoError(t, err)

		// Required headers for Prometheus Remote Write v2
		req.Header.Set("Content-Type", fmt.Sprintf("application/x-protobuf;proto=%s", promconfig.RemoteWriteProtoMsgV2))
		req.Header.Set("Content-Encoding", "snappy")
		req.Header.Set("X-Prometheus-Remote-Write-Version", "2.0.0")
		req.Header.Set("User-Agent", "test/1.0")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode, "Response body: %s", string(body))

		t.Logf("Request %d: Successfully processed by receiver", i)

		time.Sleep(100 * time.Millisecond)
	}
}
