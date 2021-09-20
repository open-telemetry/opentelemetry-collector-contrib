// Copyright OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package compliance

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/compliance/remote_write/cases"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/parserprovider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

const configFormat = `
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: 'test'
          scrape_interval: 1s
          static_configs:
            - targets: [ '%s' ]
processors:
  batch:
exporters:
  prometheusremotewrite:
    endpoint: '%s'
service:
  pipelines:
    metrics:
      receivers: [prometheus]
      processors: [batch]
      exporters: [prometheusremotewrite]
`

var (
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	tests  = []func() cases.Test{
		// Test each type.
		cases.CounterTest,
		cases.GaugeTest,
		cases.HistogramTest,
		cases.SummaryTest,

		// Test Up metrics.
		cases.UpTest,
		cases.InvalidTest,

		// Test for various labels
		cases.JobLabelTest,
		cases.InstanceLabelTest,
		cases.SortedLabelsTest,
		cases.RepeatedLabelsTest,
		cases.EmptyLabelsTest,
		cases.NameLabelTest,
		cases.HonorLabelsTest,

		// Other misc tests.
		cases.StalenessTest,
		cases.TimestampTest,
		cases.HeadersTest,
		cases.OrderingTest,
		cases.Retries500Test,
		cases.Retries400Test,
	}
)

func TestRemoteWrite(t *testing.T) {
	var _ = tests // To appease golangci-lint until the data races mentioned below are fixed.
	t.Skip("Skipping because the remote write test setup currently has data races. See https://github.com/prometheus/compliance/issues/40")
	for _, fn := range tests {
		tc := fn()
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			runTest(t, tc)
		})
	}
}

func runTest(t *testing.T, tc cases.Test) {
	ap := cases.Appendable{}
	writeHandler := remote.NewWriteHandler(logger, &ap)
	if tc.Writes != nil {
		writeHandler = tc.Writes(writeHandler)
	}

	// Start a HTTP server to expose some metrics and a receive remote write requests.
	m := http.NewServeMux()
	m.Handle("/metrics", tc.Metrics)
	m.Handle("/push", writeHandler)
	s := http.Server{
		Handler: m,
	}
	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	go s.Serve(l)
	defer s.Close()

	// Start the collector.
	shutdown := startCollector(t, l.Addr())
	defer shutdown()

	time.Sleep(10 * time.Second)

	// Check we got some data.
	tc.Expected(t, ap.Batches)
}

// startCollector starts an instance of the collector that scrapes from and sends remote writes to the specified address.
func startCollector(t *testing.T, addr net.Addr) func() {
	scrapeTarget := addr.String()
	receiveEndpoint := fmt.Sprintf("http://%s/push", addr.String())
	config := fmt.Sprintf(configFormat, scrapeTarget, receiveEndpoint)

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
		ParserProvider: parserprovider.NewInMemory(strings.NewReader(config)),
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
		if err := app.Run(context.Background()); err != nil {
			t.Error(err)
		}
	}()

	// Wait until the collector has actually started.
	stateChannel := app.GetStateChannel()
	for notYetStarted := true; notYetStarted; {
		switch state := <-stateChannel; state {
		case service.Running, service.Closed, service.Closing:
			notYetStarted = false
		}
	}

	return func() {
		app.Shutdown()
		// Wait until we are closed.
		<-stateChannel
		for notClosed := true; notClosed; {
			switch state := <-stateChannel; state {
			case service.Closed:
				notClosed = false
			}
		}
	}

}
