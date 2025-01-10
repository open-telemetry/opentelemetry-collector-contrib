// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver

import (
	"context"
	"errors"
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
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	conventions "go.opentelemetry.io/collector/semconv/v1.9.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"
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
	"system.paging.operations",
}

var resourceMetrics = []string{
	"process.cpu.time",
	"process.memory.usage",
	"process.memory.virtual",
	"process.disk.io",
}

var systemSpecificMetrics = map[string][]string{
	"linux":   {"system.disk.merged", "system.disk.weighted_io_time", "system.filesystem.inodes.usage", "system.paging.faults", "system.processes.created", "system.processes.count"},
	"darwin":  {"system.filesystem.inodes.usage", "system.paging.faults", "system.processes.count"},
	"freebsd": {"system.filesystem.inodes.usage", "system.paging.faults", "system.processes.count"},
	"openbsd": {"system.filesystem.inodes.usage", "system.paging.faults", "system.processes.created", "system.processes.count"},
	"solaris": {"system.filesystem.inodes.usage", "system.paging.faults"},
}

func TestGatherMetrics_EndToEnd(t *testing.T) {
	sink := new(consumertest.MetricsSink)

	cfg := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 100 * time.Millisecond,
		},
		Scrapers: map[component.Type]component.Config{
			cpuscraper.Type:        scraperFactories[cpuscraper.Type].CreateDefaultConfig(),
			diskscraper.Type:       scraperFactories[diskscraper.Type].CreateDefaultConfig(),
			filesystemscraper.Type: (&filesystemscraper.Factory{}).CreateDefaultConfig(),
			loadscraper.Type:       scraperFactories[loadscraper.Type].CreateDefaultConfig(),
			memoryscraper.Type:     scraperFactories[memoryscraper.Type].CreateDefaultConfig(),
			networkscraper.Type:    scraperFactories[networkscraper.Type].CreateDefaultConfig(),
			pagingscraper.Type:     scraperFactories[pagingscraper.Type].CreateDefaultConfig(),
			processesscraper.Type:  scraperFactories[processesscraper.Type].CreateDefaultConfig(),
		},
	}

	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		cfg.Scrapers[processscraper.Type] = scraperFactories[processscraper.Type].CreateDefaultConfig()
	}

	recv, err := NewFactory().CreateMetrics(context.Background(), creationSet, cfg, sink)

	require.NoError(t, err, "Failed to create metrics receiver: %v", err)

	ctx, cancelFn := context.WithCancel(context.Background())
	err = recv.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err, "Failed to start metrics receiver: %v", err)
	defer func() { assert.NoError(t, recv.Shutdown(context.Background())) }()

	// canceling the context provided to Start should not cancel any async processes initiated by the receiver
	cancelFn()

	const tick = 50 * time.Millisecond
	const waitFor = 10 * time.Second
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
		assert.EqualValues(t, conventions.SchemaURL, rm.SchemaUrl(),
			"SchemaURL is incorrect for metrics: %v", returnedMetricNames)
		if rm.Resource().Attributes().Len() == 0 {
			appendMapInto(returnedMetrics, returnedMetricNames)
		} else {
			appendMapInto(returnedResourceMetrics, returnedMetricNames)
		}
	}

	// verify the expected list of metrics returned (os/arch dependent)
	expectedMetrics := allMetrics
	if !(runtime.GOOS == "linux" && runtime.GOARCH == "arm64") {
		expectedMetrics = append(expectedMetrics, "system.paging.usage")
	}

	expectedMetrics = append(expectedMetrics, systemSpecificMetrics[runtime.GOOS]...)
	assert.Equal(t, len(expectedMetrics), len(returnedMetrics))
	for _, expected := range expectedMetrics {
		assert.Contains(t, returnedMetrics, expected)
	}

	// verify the expected list of resource metrics returned (Linux & Windows only)
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return
	}

	var expectedResourceMetrics []string
	expectedResourceMetrics = append(expectedResourceMetrics, resourceMetrics...)
	assert.Equal(t, len(expectedResourceMetrics), len(returnedResourceMetrics))
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

func appendMapInto(m1 map[string]struct{}, m2 map[string]struct{}) {
	for k, v := range m2 {
		m1[k] = v
	}
}

var mockType = component.MustNewType("mock")

type mockConfig struct{}

type errFactory struct{}

func (m *errFactory) CreateDefaultConfig() component.Config { return &mockConfig{} }
func (m *errFactory) CreateMetricsScraper(context.Context, receiver.Settings, component.Config) (scraper.Metrics, error) {
	return nil, errors.New("err1")
}

func TestGatherMetrics_ScraperKeyConfigError(t *testing.T) {
	tmp := scraperFactories
	scraperFactories = map[component.Type]internal.ScraperFactory{}
	defer func() {
		scraperFactories = tmp
	}()

	cfg := &Config{Scrapers: map[component.Type]component.Config{component.MustNewType("error"): &mockConfig{}}}
	_, err := NewFactory().CreateMetrics(context.Background(), creationSet, cfg, consumertest.NewNop())
	require.Error(t, err)
}

func TestGatherMetrics_CreateMetricsScraperError(t *testing.T) {
	mFactory := &errFactory{}
	tmp := scraperFactories
	scraperFactories = map[component.Type]internal.ScraperFactory{mockType: mFactory}
	defer func() {
		scraperFactories = tmp
	}()

	cfg := &Config{Scrapers: map[component.Type]component.Config{mockType: &mockConfig{}}}
	_, err := NewFactory().CreateMetrics(context.Background(), creationSet, cfg, consumertest.NewNop())
	require.Error(t, err)
}

type notifyingSink struct {
	receivedMetrics bool
	timesCalled     int
	ch              chan int
}

func (s *notifyingSink) Capabilities() consumer.Capabilities {
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

	options, err := createAddScraperOptions(context.Background(), receivertest.NewNopSettings(), cfg, scraperFactories)
	require.NoError(b, err)
	options = append(options, scraperhelper.WithTickerChannel(tickerCh))

	receiver, err := scraperhelper.NewScraperControllerReceiver(&cfg.ControllerConfig, receivertest.NewNopSettings(), sink, options...)
	require.NoError(b, err)

	require.NoError(b, receiver.Start(context.Background(), componenttest.NewNopHost()))

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
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
		Scrapers:         map[component.Type]component.Config{cpuscraper.Type: (&cpuscraper.Factory{}).CreateDefaultConfig()},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeDiskMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         map[component.Type]component.Config{diskscraper.Type: (&diskscraper.Factory{}).CreateDefaultConfig()},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeFileSystemMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         map[component.Type]component.Config{filesystemscraper.Type: (&filesystemscraper.Factory{}).CreateDefaultConfig()},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeLoadMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         map[component.Type]component.Config{loadscraper.Type: (&loadscraper.Factory{}).CreateDefaultConfig()},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeMemoryMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         map[component.Type]component.Config{memoryscraper.Type: (&memoryscraper.Factory{}).CreateDefaultConfig()},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeNetworkMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         map[component.Type]component.Config{networkscraper.Type: (&networkscraper.Factory{}).CreateDefaultConfig()},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeProcessesMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         map[component.Type]component.Config{processesscraper.Type: (&processesscraper.Factory{}).CreateDefaultConfig()},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapePagingMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         map[component.Type]component.Config{pagingscraper.Type: (&pagingscraper.Factory{}).CreateDefaultConfig()},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeProcessMetrics(b *testing.B) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		b.Skip("skipping test on non linux/windows")
	}

	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         map[component.Type]component.Config{processscraper.Type: (&processscraper.Factory{}).CreateDefaultConfig()},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeUptimeMetrics(b *testing.B) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		b.Skip("skipping test on non linux/windows")
	}

	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers:         map[component.Type]component.Config{systemscraper.Type: (&systemscraper.Factory{}).CreateDefaultConfig()},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeSystemMetrics(b *testing.B) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers: map[component.Type]component.Config{
			cpuscraper.Type:        (&cpuscraper.Factory{}).CreateDefaultConfig(),
			diskscraper.Type:       (&diskscraper.Factory{}).CreateDefaultConfig(),
			filesystemscraper.Type: (&filesystemscraper.Factory{}).CreateDefaultConfig(),
			loadscraper.Type:       (&loadscraper.Factory{}).CreateDefaultConfig(),
			memoryscraper.Type:     (&memoryscraper.Factory{}).CreateDefaultConfig(),
			networkscraper.Type:    (&networkscraper.Factory{}).CreateDefaultConfig(),
			pagingscraper.Type:     (&pagingscraper.Factory{}).CreateDefaultConfig(),
			processesscraper.Type:  (&processesscraper.Factory{}).CreateDefaultConfig(),
		},
	}

	benchmarkScrapeMetrics(b, cfg)
}

func Benchmark_ScrapeSystemAndProcessMetrics(b *testing.B) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		b.Skip("skipping test on non linux/windows")
	}

	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Scrapers: map[component.Type]component.Config{
			filesystemscraper.Type: (&filesystemscraper.Factory{}).CreateDefaultConfig(),
			pagingscraper.Type:     (&pagingscraper.Factory{}).CreateDefaultConfig(),
		},
	}

	benchmarkScrapeMetrics(b, cfg)
}
