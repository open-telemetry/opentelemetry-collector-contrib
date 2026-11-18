// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
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
	receiver := newScrapeValidatingReceiver(
		prometheusdatareceiver.NewPrometheusDataReceiverWithNativeHistograms(testutil.GetAvailablePort(t), payloadCfg.WithNativeHistograms),
		payloadCfg.SeriesCount,
		payloadCfg.WithNativeHistograms,
	)

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

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		s := receiver.snapshot()
		assert.Positivef(c, s.completeScrapes,
			"scraper never completed a successful scrape with %d expected series from prometheusexporter (successful=%d failed=%d short=%d minSeries=%d)",
			payloadCfg.SeriesCount, s.successfulScrapes, s.failedScrapes, s.shortScrapes, s.minSeriesPerScrape)
	}, 10*time.Second, 100*time.Millisecond)

	benchDuration := tc.Duration
	time.Sleep(benchDuration)

	// Snapshot scrape health before stopping the scraper, since canceling
	// an in-flight scrape during shutdown can emit a final up=0 report.
	stats := receiver.snapshot()

	// Stop pushing and scraping before taking final counters to avoid races with in-flight pull cycles.
	stopPushOnce()
	tc.StopBackend()
	received := waitForBackendDrain(t, tc.MockBackend, time.Second)
	throughput := float64(received) / benchDuration.Seconds()
	rc := agentProc.GetTotalConsumption()
	loadGen.SetDataItemsSent(received)

	require.Zerof(t, stats.failedScrapes, "scrape failures occurred during benchmark (up != 1; successful=%d failed=%d maxScrape=%s)",
		stats.successfulScrapes, stats.failedScrapes, stats.maxScrapeDuration.Round(time.Millisecond))
	require.Positive(t, stats.successfulScrapes, "expected at least one successful scrape")
	require.Zerof(t, stats.shortScrapes, "successful scrape returned fewer than %d expected series (min seen: %d)",
		payloadCfg.SeriesCount, stats.minSeriesPerScrape)

	t.Logf("Received %d data items (%d successful scrapes before stop, max scrape=%s) in %s (%.0f items/sec). CPU avg/max=%.1f%%/%.1f%%, RAM avg/max=%d/%d MiB",
		received, stats.successfulScrapes, stats.maxScrapeDuration.Round(time.Millisecond), benchDuration, throughput, rc.CPUPercentAvg, rc.CPUPercentMax, rc.RAMMiBAvg, rc.RAMMiBMax)
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

type scrapeStats struct {
	successfulScrapes  uint64
	completeScrapes    uint64
	failedScrapes      uint64
	shortScrapes       uint64
	minSeriesPerScrape uint64
	maxScrapeDuration  time.Duration
}

// scrapeValidatingReceiver wraps a testbed.DataReceiver to inspect scraped metrics on
// the fly without recording full pmetric.Metrics payloads in memory.
type scrapeValidatingReceiver struct {
	testbed.DataReceiver
	expectedSeries         uint64
	expectNativeHistograms bool

	mu    sync.Mutex
	stats scrapeStats
}

func newScrapeValidatingReceiver(
	inner testbed.DataReceiver,
	expectedSeries int,
	expectNativeHistograms bool,
) *scrapeValidatingReceiver {
	return &scrapeValidatingReceiver{
		DataReceiver:           inner,
		expectedSeries:         uint64(max(expectedSeries, 0)),
		expectNativeHistograms: expectNativeHistograms,
	}
}

func (r *scrapeValidatingReceiver) Start(tc consumer.Traces, mc consumer.Metrics, lc consumer.Logs) error {
	wrapped, err := consumer.NewMetrics(func(ctx context.Context, md pmetric.Metrics) error {
		r.recordBatch(md)
		return mc.ConsumeMetrics(ctx, md)
	})
	if err != nil {
		return err
	}
	return r.DataReceiver.Start(tc, wrapped, lc)
}

func (r *scrapeValidatingReceiver) snapshot() scrapeStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats
}

func (r *scrapeValidatingReceiver) recordBatch(md pmetric.Metrics) {
	var batchSuccess, batchFailed, batchSeries uint64
	var batchMaxDur time.Duration

	rms := md.ResourceMetrics()
	for i := range rms.Len() {
		sms := rms.At(i).ScopeMetrics()
		for j := range sms.Len() {
			ms := sms.At(j).Metrics()
			for k := range ms.Len() {
				m := ms.At(k)
				switch m.Name() {
				case "up":
					if m.Type() != pmetric.MetricTypeGauge {
						continue
					}
					dps := m.Gauge().DataPoints()
					for l := range dps.Len() {
						dp := dps.At(l)
						if dp.Flags().NoRecordedValue() {
							continue
						}
						val := dp.DoubleValue()
						if dp.ValueType() == pmetric.NumberDataPointValueTypeInt {
							val = float64(dp.IntValue())
						}
						if val == 1 {
							batchSuccess++
						} else {
							batchFailed++
						}
					}
				case "scrape_duration_seconds":
					if m.Type() != pmetric.MetricTypeGauge {
						continue
					}
					dps := m.Gauge().DataPoints()
					for l := range dps.Len() {
						dp := dps.At(l)
						if dp.Flags().NoRecordedValue() {
							continue
						}
						if d := time.Duration(dp.DoubleValue() * float64(time.Second)); d > batchMaxDur {
							batchMaxDur = d
						}
					}
				default:
					if strings.HasPrefix(m.Name(), "bench_") {
						batchSeries += countValidSeries(m, r.expectNativeHistograms)
					}
				}
			}
		}
	}

	if batchSuccess == 0 && batchFailed == 0 && batchMaxDur == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if batchMaxDur > r.stats.maxScrapeDuration {
		r.stats.maxScrapeDuration = batchMaxDur
	}
	r.stats.failedScrapes += batchFailed
	if batchSuccess > 0 {
		perScrape := batchSeries / batchSuccess
		if r.stats.successfulScrapes == 0 || perScrape < r.stats.minSeriesPerScrape {
			r.stats.minSeriesPerScrape = perScrape
		}
		r.stats.successfulScrapes += batchSuccess
		if batchSeries < r.expectedSeries*batchSuccess {
			r.stats.shortScrapes += batchSuccess
		} else {
			r.stats.completeScrapes += batchSuccess
		}
	}
}

func countValidSeries(m pmetric.Metric, expectNativeHistograms bool) uint64 {
	if expectNativeHistograms && m.Type() != pmetric.MetricTypeExponentialHistogram {
		return 0
	}
	var count uint64
	switch m.Type() {
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := range dps.Len() {
			if !dps.At(i).Flags().NoRecordedValue() {
				count++
			}
		}
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := range dps.Len() {
			if !dps.At(i).Flags().NoRecordedValue() {
				count++
			}
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := range dps.Len() {
			if !dps.At(i).Flags().NoRecordedValue() {
				count++
			}
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := range dps.Len() {
			if !dps.At(i).Flags().NoRecordedValue() {
				count++
			}
		}
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		for i := range dps.Len() {
			if !dps.At(i).Flags().NoRecordedValue() {
				count++
			}
		}
	case pmetric.MetricTypeEmpty:
		return 0
	}
	return count
}
