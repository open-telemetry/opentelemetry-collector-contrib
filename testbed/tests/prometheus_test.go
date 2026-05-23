// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"fmt"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestPrometheusReceiver(t *testing.T) {
	tests := []struct {
		name         string
		payload      datasenders.PrometheusStaticPayloadConfig
		resourceSpec testbed.ResourceSpec
	}{
		{
			name: "Baseline/1k",
			payload: datasenders.PrometheusStaticPayloadConfig{
				SeriesCount:     1000,
				LabelsPerSeries: 5,
			},
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 200,
				ExpectedMaxRAM: 300,
			},
		},
		{
			name: "Baseline/10k",
			payload: datasenders.PrometheusStaticPayloadConfig{
				SeriesCount:     10000,
				LabelsPerSeries: 5,
			},
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 200,
				ExpectedMaxRAM: 400,
			},
		},
		{
			name: "WithTargetInfo/10k",
			payload: datasenders.PrometheusStaticPayloadConfig{
				SeriesCount:     10000,
				LabelsPerSeries: 5,
				WithTargetInfo:  true,
			},
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 200,
				ExpectedMaxRAM: 400,
			},
		},
		{
			name: "NativeHistogram/10k",
			payload: datasenders.PrometheusStaticPayloadConfig{
				SeriesCount:          10000,
				WithNativeHistograms: true,
			},
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 200,
				ExpectedMaxRAM: 500,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioPrometheusScrape(t, tt.payload, tt.resourceSpec)
		})
	}
}

// scenarioPrometheusScrape runs a benchmark for the Prometheus receiver.
// It starts a static HTTP endpoint serving Prometheus metrics, configures
// a child-process collector to scrape it, and sinks data into an OTLP backend.
func scenarioPrometheusScrape(
	t *testing.T,
	payloadCfg datasenders.PrometheusStaticPayloadConfig,
	resourceSpec testbed.ResourceSpec,
) {
	sender := datasenders.NewPrometheusStaticSender(
		testbed.DefaultHost,
		testutil.GetAvailablePort(t),
		payloadCfg,
	)

	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))
	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)
	configStr := createPromConfigYaml(sender, receiver, resultDir)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	t.Cleanup(configCleanup)

	loadGen := &staticScrapeLoadGenerator{sender: sender}
	tc := testbed.NewLoadGeneratorTestCase(
		t,
		loadGen,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{IncludeLimitsInReport: true},
		performanceResultsSummary,
		testbed.WithResourceLimits(resourceSpec),
	)
	t.Cleanup(tc.Stop)

	var senderStop func()
	require.NoError(t, sender.Start())
	senderStop = func() {
		if stopper, ok := sender.(interface{ Stop() error }); ok {
			_ = stopper.Stop()
		}
	}
	t.Cleanup(senderStop)

	tc.StartBackend()
	tc.StartAgent()

	require.Eventually(t, func() bool { return tc.MockBackend.DataItemsReceived() > 0 },
		10*time.Second, 100*time.Millisecond, "collector never produced data from scrapes")

	benchDuration := tc.Duration
	time.Sleep(benchDuration)

	// Stop the scrape target before taking final counters to avoid races with in-flight pull cycles.
	senderStop()
	received := waitForBackendDrain(t, tc.MockBackend, time.Second)
	throughput := float64(received) / benchDuration.Seconds()
	rc := agentProc.GetTotalConsumption()
	loadGen.SetDataItemsSent(received)

	t.Logf("Received %d data items in %s (%.0f items/sec). CPU avg/max=%.1f%%/%.1f%%, RAM avg/max=%d/%d MiB",
		received, benchDuration, throughput, rc.CPUPercentAvg, rc.CPUPercentMax, rc.RAMMiBAvg, rc.RAMMiBMax)
}

// createPromConfigYaml generates collector config YAML for the Prometheus
// scrape benchmark. We can't reuse createConfigYaml from scenarios.go because
// the sender is not a MetricDataSender (it's pull-based, not push-based).
func createPromConfigYaml(sender testbed.DataSender, receiver testbed.DataReceiver, resultDir string) string {
	return fmt.Sprintf(`
receivers:%s
exporters:%s
extensions:
  pprof:
    save_to_file: %s/cpu.prof

service:
  extensions: [pprof]
  pipelines:
    metrics:
      receivers: [%s]
      exporters: [%s]
`,
		sender.GenConfigYAMLStr(),
		receiver.GenConfigYAMLStr(),
		resultDir,
		sender.ProtocolName(),
		receiver.ProtocolName(),
	)
}

type staticScrapeLoadGenerator struct {
	sender testbed.DataSender
	sent   atomic.Uint64
}

func (*staticScrapeLoadGenerator) Start(testbed.LoadOptions) {}
func (*staticScrapeLoadGenerator) Stop()                     {}
func (lg *staticScrapeLoadGenerator) DataItemsSent() uint64  { return lg.sent.Load() }
func (lg *staticScrapeLoadGenerator) IncDataItemsSent()      { lg.sent.Add(1) }
func (*staticScrapeLoadGenerator) PermanentErrors() uint64   { return 0 }
func (*staticScrapeLoadGenerator) GetStats() string          { return "" }

func (lg *staticScrapeLoadGenerator) SetDataItemsSent(n uint64) {
	lg.sent.Store(n)
}

func waitForBackendDrain(t *testing.T, backend *testbed.MockBackend, timeout time.Duration) uint64 {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := backend.DataItemsReceived()
	stableReads := 0
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		cur := backend.DataItemsReceived()
		if cur == last {
			stableReads++
			if stableReads >= 2 {
				return cur
			}
			continue
		}
		last = cur
		stableReads = 0
	}
	return backend.DataItemsReceived()
}

func (lg *staticScrapeLoadGenerator) IsReady() bool {
	endpoint := lg.sender.GetEndpoint()
	if endpoint == nil {
		return true
	}
	conn, err := net.Dial(endpoint.Network(), endpoint.String())
	if err == nil && conn != nil {
		_ = conn.Close()
		return true
	}
	return false
}
