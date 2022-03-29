// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

func TestJobInstanceRelabelEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("This test can take a long time")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 1. Setup the server that exposes metrics.
	scrapeServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fmt.Fprint(rw, `
# HELP jvm_memory_bytes_used Used bytes of a given JVM memory area.
# TYPE jvm_memory_bytes_used gauge
jvm_memory_bytes_used{area="heap"} 100`)
	}))
	defer scrapeServer.Close()

	serverURL, err := url.Parse(scrapeServer.URL)
	require.Nil(t, err)

	// 2. Set up the Prometheus RemoteWrite endpoint.
	prweUploads := make(chan *prompb.WriteRequest)
	prweServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Snappy decode the uploads.
		payload, rerr := ioutil.ReadAll(req.Body)
		if err != nil {
			panic(rerr)
		}
		recv := make([]byte, len(payload))
		decoded, derr := snappy.Decode(recv, payload)
		if err != nil {
			panic(derr)
		}

		writeReq := new(prompb.WriteRequest)
		if uerr := proto.Unmarshal(decoded, writeReq); uerr != nil {
			panic(uerr)
		}

		select {
		case <-ctx.Done():
			return
		case prweUploads <- writeReq:
		}
	}))
	defer prweServer.Close()

	// 3. Set the OpenTelemetry Prometheus receiver.
	config := fmt.Sprintf(`
receivers:
  prometheus:
    config:
      scrape_configs:
      - job_name: 'test'
        scrape_interval: 2ms
        metric_relabel_configs:
        - action: replace
          target_label: instance
          replacement: relabeled-instance
        - action: replace
          target_label: job
          replacement: not-test
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

	confFile, err := ioutil.TempFile(os.TempDir(), "conf-")
	require.Nil(t, err)
	defer os.Remove(confFile.Name())
	_, err = confFile.Write([]byte(config))
	require.Nil(t, err)
	// 4. Run the OpenTelemetry Collector.
	receivers, err := component.MakeReceiverFactoryMap(prometheusreceiver.NewFactory())
	require.Nil(t, err)
	exporters, err := component.MakeExporterFactoryMap(prometheusremotewriteexporter.NewFactory())
	require.Nil(t, err)
	processors, err := component.MakeProcessorFactoryMap(batchprocessor.NewFactory())
	require.Nil(t, err)

	factories := component.Factories{
		Receivers:  receivers,
		Exporters:  exporters,
		Processors: processors,
	}

	appSettings := service.CollectorSettings{
		Factories:      factories,
		ConfigProvider: service.MustNewDefaultConfigProvider([]string{confFile.Name()}, nil),
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

	app, err := service.New(appSettings)
	require.Nil(t, err)

	go func() {
		if err = app.Run(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	defer app.Shutdown()

	// Wait until the collector has actually started.
	for notYetStarted := true; notYetStarted; {
		state := app.GetState()
		switch state {
		case service.Running, service.Closed, service.Closing:
			notYetStarted = false
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
	totalSamples := 0
	require.True(t, len(wReqL) > 0, "Expecting at least one WriteRequest")
	for i, wReq := range wReqL {
		name := fmt.Sprintf("WriteRequest#%d", i)
		require.True(t, len(wReq.Timeseries) > 0, "Expecting at least 1 timeSeries for:: "+name)
		for j, ts := range wReq.Timeseries {
			fullName := fmt.Sprintf("%s/TimeSeries#%d", name, j)
			assert.True(t, len(ts.Samples) > 0, "Expected at least 1 Sample in:: "+fullName)

			// We are strictly counting series directly included in the scrapes, and no
			// internal timeseries like "up" nor "scrape_seconds" etc.
			metricName := ""
			job := ""
			instance := ""
			for _, label := range ts.Labels {
				if label.Name == "__name__" {
					metricName = label.Value
				}

				if label.Name == "job" {
					job = label.Value
				}

				if label.Name == "instance" {
					instance = label.Value
				}
			}

			if !strings.HasPrefix(metricName, "jvm") {
				continue
			}

			require.Equal(t, "not-test", job)
			require.Equal(t, "relabeled-instance", instance)

			for range ts.Samples {
				totalSamples++
			}
		}
	}

	require.True(t, totalSamples > 0, "Expected at least 1 sample")
}
