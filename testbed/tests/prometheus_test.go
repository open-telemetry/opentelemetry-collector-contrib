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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers/prometheusdatareceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders/prometheusstaticdatasender"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func prometheusBenchmarkTests() []struct {
	name         string
	payload      prometheusstaticdatasender.PrometheusStaticPayloadConfig
	resourceSpec testbed.ResourceSpec
} {
	return []struct {
		name         string
		payload      prometheusstaticdatasender.PrometheusStaticPayloadConfig
		resourceSpec testbed.ResourceSpec
	}{
		{
			name: "Baseline/1k",
			payload: prometheusstaticdatasender.PrometheusStaticPayloadConfig{
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
			payload: prometheusstaticdatasender.PrometheusStaticPayloadConfig{
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
			payload: prometheusstaticdatasender.PrometheusStaticPayloadConfig{
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
			payload: prometheusstaticdatasender.PrometheusStaticPayloadConfig{
				SeriesCount:          10000,
				WithNativeHistograms: true,
			},
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 200,
				ExpectedMaxRAM: 500,
			},
		},
	}
}

func TestPrometheusReceiver(t *testing.T) {
	for _, tt := range prometheusBenchmarkTests() {
		t.Run(tt.name, func(t *testing.T) {
			scenarioPrometheusScrape(t, tt.payload, tt.resourceSpec)
		})
	}
}

func TestPrometheusExporter(t *testing.T) {
	for _, tt := range prometheusBenchmarkTests() {
		t.Run(tt.name, func(t *testing.T) {
			scenarioPrometheusExport(t, tt.payload, tt.resourceSpec)
		})
	}
}

// scenarioPrometheusScrape runs a benchmark for the Prometheus receiver.
// It starts a static HTTP endpoint serving Prometheus metrics, configures
// a child-process collector to scrape it, and sinks data into an OTLP backend.
func scenarioPrometheusScrape(
	t *testing.T,
	payloadCfg prometheusstaticdatasender.PrometheusStaticPayloadConfig,
	resourceSpec testbed.ResourceSpec,
) {
	sender := prometheusstaticdatasender.NewPrometheusStaticSender(
		testbed.DefaultHost,
		testutil.GetAvailablePort(t),
		payloadCfg,
	)

	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))
	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)
	configStr := createPromConfigYaml(t, sender, receiver, resultDir)
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

// scenarioPrometheusExport runs a benchmark for the Prometheus exporter.
// It pushes OTLP metrics into a child-process collector configured with
// prometheusexporter and scrapes its /metrics endpoint into MockBackend.
func scenarioPrometheusExport(
	t *testing.T,
	payloadCfg prometheusstaticdatasender.PrometheusStaticPayloadConfig,
	resourceSpec testbed.ResourceSpec,
) {
	sender := testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := prometheusdatareceiver.NewPrometheusDataReceiverWithNativeHistograms(testutil.GetAvailablePort(t), payloadCfg.WithNativeHistograms)

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))
	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)
	configStr := createPromConfigYaml(t, sender, receiver, resultDir)
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

	tc.StartAgent()
	require.NoError(t, sender.Start())
	require.NoError(t, sender.ConsumeMetrics(t.Context(), buildPrometheusExportPayload(payloadCfg)))
	sender.Flush()

	stopPush := make(chan struct{})
	pushDone := make(chan struct{})
	go func() {
		defer close(pushDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopPush:
				return
			case <-ticker.C:
				_ = sender.ConsumeMetrics(t.Context(), buildPrometheusExportPayload(payloadCfg))
				sender.Flush()
			}
		}
	}()
	stopPushOnce := func() {
		select {
		case <-stopPush:
		default:
			close(stopPush)
		}
		<-pushDone
	}
	t.Cleanup(stopPushOnce)

	tc.StartBackend()

	require.Eventually(t, func() bool { return tc.MockBackend.DataItemsReceived() > 0 },
		10*time.Second, 100*time.Millisecond, "scraper never received metrics from prometheusexporter")

	benchDuration := tc.Duration
	time.Sleep(benchDuration)

	// Stop pushing and scraping before taking final counters to avoid races with in-flight pull cycles.
	stopPushOnce()
	tc.StopBackend()
	received := waitForBackendDrain(t, tc.MockBackend, time.Second)
	throughput := float64(received) / benchDuration.Seconds()
	rc := agentProc.GetTotalConsumption()
	loadGen.SetDataItemsSent(received)
	tc.ValidateData()

	t.Logf("Received %d data items in %s (%.0f items/sec). CPU avg/max=%.1f%%/%.1f%%, RAM avg/max=%d/%d MiB",
		received, benchDuration, throughput, rc.CPUPercentAvg, rc.CPUPercentMax, rc.RAMMiBAvg, rc.RAMMiBMax)
}

func buildPrometheusExportPayload(cfg prometheusstaticdatasender.PrometheusStaticPayloadConfig) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rAttrs := rm.Resource().Attributes()
	rAttrs.PutStr("service.name", "bench-service")
	rAttrs.PutStr("service.instance.id", "instance-0")
	if cfg.WithTargetInfo {
		rAttrs.PutStr("env", "prod")
		rAttrs.PutStr("region", "us-east-1")
		rAttrs.PutStr("cluster", "bench-01")
	}

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("bench.scope")
	sm.Scope().SetVersion("1.0.0")

	now := pcommon.NewTimestampFromTime(time.Now())

	if cfg.WithNativeHistograms {
		numHists := 50
		seriesPerHist := max(cfg.SeriesCount/numHists, 1)
		for h := range numHists {
			m := sm.Metrics().AppendEmpty()
			m.SetName(fmt.Sprintf("bench_native_histogram_%d", h))
			m.SetDescription(fmt.Sprintf("Benchmark native histogram %d", h))
			eh := m.SetEmptyExponentialHistogram()
			eh.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			dps := eh.DataPoints()
			dps.EnsureCapacity(seriesPerHist)
			for s := range seriesPerHist {
				dp := dps.AppendEmpty()
				dp.SetTimestamp(now)
				dp.SetScale(2)
				dp.SetCount(105)
				dp.SetSum(52.5)
				dp.SetZeroCount(2)
				dp.SetZeroThreshold(0)
				dp.Positive().SetOffset(0)
				dp.Positive().BucketCounts().FromRaw([]uint64{10, 20, 30, 20, 10, 5})
				dp.Negative().SetOffset(0)
				dp.Negative().BucketCounts().FromRaw([]uint64{2, 4, 2})
				dp.Attributes().PutStr("series_id", fmt.Sprintf("s_%d", s))
				dp.Attributes().PutStr("le_group", fmt.Sprintf("g_%d", s%10))
			}
		}
		return md
	}

	numMetrics := 100
	seriesPerMetric := max(cfg.SeriesCount/numMetrics, 1)
	numLabels := max(cfg.LabelsPerSeries, 1)
	for i := range numMetrics {
		m := sm.Metrics().AppendEmpty()
		m.SetName(fmt.Sprintf("bench_metric_%d", i))
		m.SetDescription(fmt.Sprintf("Benchmark gauge metric %d", i))
		g := m.SetEmptyGauge()
		dps := g.DataPoints()
		dps.EnsureCapacity(seriesPerMetric)
		for j := range seriesPerMetric {
			dp := dps.AppendEmpty()
			dp.SetTimestamp(now)
			dp.SetDoubleValue(float64(i*seriesPerMetric + j))
			attrs := dp.Attributes()
			attrs.PutStr("series_id", fmt.Sprintf("series_%d", j))
			for k := 1; k < numLabels; k++ {
				attrs.PutStr(fmt.Sprintf("label_%d", k), fmt.Sprintf("val_%d_%d", k, j%50))
			}
		}
	}
	return md
}

// createPromConfigYaml generates collector config YAML for the Prometheus
// scrape and export benchmarks.
func createPromConfigYaml(t *testing.T, sender testbed.DataSender, receiver testbed.DataReceiver, resultDir string) string {
	return fmt.Sprintf(`
receivers:%s
exporters:%s
extensions:
  pprof:
    endpoint: 127.0.0.1:%d
    save_to_file: %s/cpu.prof

service:
  telemetry:
    metrics:
      readers:
        - pull:
            exporter:
              prometheus:
                host: 127.0.0.1
                port: %d
  extensions: [pprof]
  pipelines:
    metrics:
      receivers: [%s]
      exporters: [%s]
`,
		sender.GenConfigYAMLStr(),
		receiver.GenConfigYAMLStr(),
		testutil.GetAvailablePort(t),
		resultDir,
		testutil.GetAvailablePort(t),
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
