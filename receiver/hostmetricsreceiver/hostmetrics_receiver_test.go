// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver

import (
	"context"
	"errors"
	"maps"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/systemscraper"
)

var allMetrics = []string{
	"system.cpu.time",
	"system.cpu.load_average.1m",
	"system.cpu.load_average.5m",
	"system.cpu.load_average.15m",
	"system.disk.io",
	"system.disk.io_time",
	"system.disk.operations",
	"system.disk.operation_time",
	"system.disk.pending_operations",
	"system.filesystem.usage",
	"system.memory.usage",
	"system.network.connections",
	"system.network.dropped",
	"system.network.errors",
	"system.network.io",
	"system.network.packets",
	"system.paging.faults",
	"system.paging.operations",
	"system.paging.usage",
}

var resourceMetrics = []string{
	"process.cpu.time",
	"process.memory.usage",
	"process.memory.virtual",
	"process.disk.io",
}

var systemSpecificMetrics = map[string][]string{
	"linux":   {"system.disk.merged", "system.disk.weighted_io_time", "system.filesystem.inodes.usage", "system.processes.created", "system.processes.count"},
	"darwin":  {"system.filesystem.inodes.usage", "system.processes.count"},
	"freebsd": {"system.filesystem.inodes.usage", "system.processes.count"},
	"openbsd": {"system.filesystem.inodes.usage", "system.processes.created", "system.processes.count"},
	"solaris": {"system.filesystem.inodes.usage"},
}

var systemSpecificMetricsNFS = map[string][]string{
	"linux": {"nfs.client.net.count", "nfs.client.net.tcp.connection.accepted", "nfs.client.rpc.count", "nfs.client.rpc.retransmit.count", "nfs.client.rpc.authrefresh.count", "nfs.client.procedure.count", "nfs.client.operation.count", "nfs.server.repcache.requests", "nfs.server.fh.stale.count", "nfs.server.io", "nfs.server.thread.count", "nfs.server.net.count", "nfs.server.net.tcp.connection.accepted", "nfs.server.rpc.count", "nfs.server.procedure.count", "nfs.server.operation.count"},
}

func TestGatherMetrics_EndToEnd(t *testing.T) {
	sink := new(consumertest.MetricsSink)

	cfg := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 100 * time.Millisecond,
		},
		Scrapers: newScrapersConfigs(
			cpuscraper.NewFactory(),
			diskscraper.NewFactory(),
			filesystemscraper.NewFactory(),
			loadscraper.NewFactory(),
			memoryscraper.NewFactory(),
			networkscraper.NewFactory(),
			pagingscraper.NewFactory(),
			processesscraper.NewFactory(),
		),
	}

	if runtime.GOOS == "linux" && nfsscraper.CanScrapeAll() {
		f := nfsscraper.NewFactory()
		cfg.Scrapers[f.Type()] = f.CreateDefaultConfig()
	}

	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		f := processscraper.NewFactory()
		cfg.Scrapers[f.Type()] = f.CreateDefaultConfig()
	}

	recv, err := NewFactory().CreateMetrics(t.Context(), creationSet, cfg, sink)
	require.NoError(t, err)

	ctx, cancelFn := context.WithCancel(t.Context())
	err = recv.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { assert.NoError(t, recv.Shutdown(t.Context())) }()

	// canceling the context provided to Start should not cancel any async processes initiated by the receiver
	cancelFn()

	const tick = 200 * time.Millisecond
	const waitFor = 30 * time.Second
	require.Eventuallyf(t, func() bool {
		got := sink.AllMetrics()
		if len(got) == 0 {
			return false
		}

		assertIncludesExpectedMetrics(t, got[0])
		return true
	}, waitFor, tick, "No metrics were collected after %v", waitFor)
}

func assertIncludesExpectedMetrics(t *testing.T, got pmetric.Metrics) {
	// get the superset of metrics returned by all resource metrics (excluding the first)
	returnedMetrics := make(map[string]struct{})
	returnedResourceMetrics := make(map[string]struct{})
	rms := got.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		metrics := getMetricSlice(t, rm)
		returnedMetricNames := getReturnedMetricNames(metrics)
		assert.Equal(t, "https://opentelemetry.io/schemas/1.9.0", rm.SchemaUrl(),
			"SchemaURL is incorrect for metrics: %v", returnedMetricNames)
		if rm.Resource().Attributes().Len() == 0 {
			maps.Copy(returnedMetrics, returnedMetricNames)
		} else {
			maps.Copy(returnedResourceMetrics, returnedMetricNames)
		}
	}

	// verify the expected list of metrics returned (os/arch dependent)
	expectedMetrics := allMetrics

	expectedMetrics = append(expectedMetrics, systemSpecificMetrics[runtime.GOOS]...)
	if nfsscraper.CanScrapeAll() {
		expectedMetrics = append(expectedMetrics, systemSpecificMetricsNFS[runtime.GOOS]...)
	}

	assert.Len(t, returnedMetrics, len(expectedMetrics))
	for _, expected := range expectedMetrics {
		assert.Contains(t, returnedMetrics, expected)
	}

	// verify the expected list of resource metrics returned (Linux & Windows only)
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return
	}

	var expectedResourceMetrics []string
	expectedResourceMetrics = append(expectedResourceMetrics, resourceMetrics...)
	assert.Len(t, returnedResourceMetrics, len(expectedResourceMetrics))
	for _, expected := range expectedResourceMetrics {
		assert.Contains(t, returnedResourceMetrics, expected)
	}
}

func getMetricSlice(t *testing.T, rm pmetric.ResourceMetrics) pmetric.MetricSlice {
	ilms := rm.ScopeMetrics()
	require.Equal(t, 1, ilms.Len())
	return ilms.At(0).Metrics()
}

func getReturnedMetricNames(metrics pmetric.MetricSlice) map[string]struct{} {
	metricNames := make(map[string]struct{})
	for i := 0; i < metrics.Len(); i++ {
		metricNames[metrics.At(i).Name()] = struct{}{}
	}
	return metricNames
}

var mockType = component.MustNewType("mock")

type mockConfig struct{}

func errCreateDefaultConfig() component.Config { return &mockConfig{} }

func errCreateMetrics(context.Context, scraper.Settings, component.Config) (scraper.Metrics, error) {
	return nil, errors.New("err1")
}

func TestGatherMetrics_ScraperKeyConfigError(t *testing.T) {
	tmp := scraperFactories
	scraperFactories = map[component.Type]scraper.Factory{}
	defer func() {
		scraperFactories = tmp
	}()

	cfg := &Config{Scrapers: map[component.Type]component.Config{component.MustNewType("error"): &mockConfig{}}}
	_, err := NewFactory().CreateMetrics(t.Context(), creationSet, cfg, consumertest.NewNop())
	require.Error(t, err)
}

func TestGatherMetrics_CreateMetricsError(t *testing.T) {
	mFactory := scraper.NewFactory(mockType, errCreateDefaultConfig, scraper.WithMetrics(errCreateMetrics, component.StabilityLevelAlpha))
	tmp := scraperFactories
	scraperFactories = mustMakeFactories(mFactory)
	defer func() {
		scraperFactories = tmp
	}()

	cfg := &Config{Scrapers: map[component.Type]component.Config{mockType: &mockConfig{}}}
	_, err := NewFactory().CreateMetrics(t.Context(), creationSet, cfg, consumertest.NewNop())
	require.Error(t, err)
}

type notifyingSink struct {
	receivedMetrics bool
	timesCalled     int
	ch              chan int
}

func (*notifyingSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (s *notifyingSink) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	if md.MetricCount() > 0 {
		s.receivedMetrics = true
	}

	s.timesCalled++
	s.ch <- s.timesCalled
	return nil
}

func benchmarkScrapeMetrics(b *testing.B, cfg *Config) {
	sink := &notifyingSink{ch: make(chan int, 10)}
	tickerCh := make(chan time.Time)

	options, err := createAddScraperOptions(b.Context(), cfg, scraperFactories)
	require.NoError(b, err)
	options = append(options, scraperhelper.WithTickerChannel(tickerCh))

	receiver, err := scraperhelper.NewMetricsController(&cfg.ControllerConfig, receivertest.NewNopSettings(metadata.Type), sink, options...)
	require.NoError(b, err)

	require.NoError(b, receiver.Start(b.Context(), componenttest.NewNopHost()))
	b.Cleanup(func() { require.NoError(b, receiver.Shutdown(b.Context())) })

	for b.Loop() {
		tickerCh <- time.Now()
		<-sink.ch
	}

	if !sink.receivedMetrics {
		b.Fail()
	}
}

func Benchmark_ScrapeCpuMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(cpuscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeDiskMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(diskscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeFileSystemMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(filesystemscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeLoadMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(loadscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeMemoryMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(memoryscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeNetworkMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(networkscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeProcessesMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(processesscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapePagingMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(pagingscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeNFSMetrics(b *testing.B) {
	if !nfsscraper.CanScrapeAll() || runtime.GOOS != "linux" {
		b.Skip("skipping test on unsupported platform")
	}

	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(nfsscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeProcessMetrics(b *testing.B) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		b.Skip("skipping test on non linux/windows")
	}

	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(processscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeUptimeMetrics(b *testing.B) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		b.Skip("skipping test on non linux/windows")
	}

	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         newScrapersConfigs(systemscraper.NewFactory()),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeSystemMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers: newScrapersConfigs(
			cpuscraper.NewFactory(),
			diskscraper.NewFactory(),
			filesystemscraper.NewFactory(),
			loadscraper.NewFactory(),
			memoryscraper.NewFactory(),
			networkscraper.NewFactory(),
			pagingscraper.NewFactory(),
			processesscraper.NewFactory(),
		),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeSystemAndProcessMetrics(b *testing.B) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		b.Skip("skipping test on non linux/windows")
	}

	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers: newScrapersConfigs(
			filesystemscraper.NewFactory(),
			pagingscraper.NewFactory(),
		),
	}

	benchmarkScrapeMetrics(b, cfg)
}

func newScrapersConfigs(factories ...scraper.Factory) map[component.Type]component.Config {
	factoriesMap := mustMakeFactories(factories...)
	factoriesCfg := map[component.Type]component.Config{}
	for typ, factory := range factoriesMap {
		factoriesCfg[typ] = factory.CreateDefaultConfig()
	}
	return factoriesCfg
}
