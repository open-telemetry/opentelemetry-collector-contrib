// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver/internal/metadata"
)

type metricsReceiver struct {
	config        *Config
	set           receiver.Settings
	clientFactory clientFactory
	scraper       *containerScraper
	mb            *metadata.MetricsBuilder
	cancel        context.CancelFunc
}

func newMetricsReceiver(
	set receiver.Settings,
	config *Config,
	clientFactory clientFactory,
) *metricsReceiver {
	if clientFactory == nil {
		clientFactory = newLibpodClient
	}

	return &metricsReceiver{
		config:        config,
		clientFactory: clientFactory,
		set:           set,
		mb:            metadata.NewMetricsBuilder(config.MetricsBuilderConfig, set),
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	podmanConfig := config.(*Config)

	recv := newMetricsReceiver(params, podmanConfig, nil)
	scrp, err := scraper.NewMetrics(recv.scrape, scraper.WithStart(recv.start), scraper.WithShutdown(recv.shutdown))
	if err != nil {
		return nil, err
	}
	return scraperhelper.NewMetricsController(&recv.config.ControllerConfig, params, consumer, scraperhelper.AddScraper(metadata.Type, scrp))
}

func (r *metricsReceiver) start(ctx context.Context, _ component.Host) error {
	podmanClient, err := r.clientFactory(r.set.Logger, r.config)
	if err != nil {
		return err
	}

	r.scraper = newContainerScraper(podmanClient, r.set.Logger, r.config)
	if err = r.scraper.loadContainerList(ctx); err != nil {
		return err
	}

	// context for long-running operation
	cctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	go r.scraper.containerEventLoop(cctx)

	return nil
}

func (r *metricsReceiver) shutdown(context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}

type result struct {
	container      container
	containerStats containerStats
	err            error
}

func (r *metricsReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	containers := r.scraper.getContainers()
	results := make(chan result, len(containers))

	wg := &sync.WaitGroup{}
	wg.Add(len(containers))
	for _, c := range containers {
		go func(c container) {
			defer wg.Done()
			stats, err := r.scraper.fetchContainerStats(ctx, c)
			results <- result{container: c, containerStats: stats, err: err}
		}(c)
	}

	wg.Wait()
	close(results)

	var errs error
	now := pcommon.NewTimestampFromTime(time.Now())

	for res := range results {
		if res.err != nil {
			// Don't know the number of failed metrics, but one container fetch is a partial error.
			errs = multierr.Append(errs, scrapererror.NewPartialScrapeError(res.err, 0))
			continue
		}
		r.recordContainerStats(now, res.container, &res.containerStats)
	}
	return r.mb.Emit(), errs
}

func (r *metricsReceiver) recordContainerStats(now pcommon.Timestamp, container container, stats *containerStats) {
	r.recordCPUMetrics(now, stats)
	r.recordNetworkMetrics(now, stats)
	r.recordMemoryMetrics(now, stats)
	r.recordIOMetrics(now, stats)

	rb := r.mb.NewResourceBuilder()
	rb.SetContainerRuntime("podman")
	rb.SetContainerName(stats.Name)
	rb.SetContainerID(stats.ContainerID)
	rb.SetContainerImageName(container.Image)

	r.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (r *metricsReceiver) recordCPUMetrics(now pcommon.Timestamp, stats *containerStats) {
	r.mb.RecordContainerCPUUsageSystemDataPoint(now, int64(toSecondsWithNanosecondPrecision(stats.CPUSystemNano)))
	r.mb.RecordContainerCPUUsageTotalDataPoint(now, int64(toSecondsWithNanosecondPrecision(stats.CPUNano)))
	r.mb.RecordContainerCPUPercentDataPoint(now, stats.CPU)

	for i, cpu := range stats.PerCPU {
		r.mb.RecordContainerCPUUsagePercpuDataPoint(now, int64(toSecondsWithNanosecondPrecision(cpu)), fmt.Sprintf("cpu%d", i))
	}
}

func (r *metricsReceiver) recordNetworkMetrics(now pcommon.Timestamp, stats *containerStats) {
	r.mb.RecordContainerNetworkIoUsageRxBytesDataPoint(now, int64(stats.NetOutput))
	r.mb.RecordContainerNetworkIoUsageTxBytesDataPoint(now, int64(stats.NetInput))
}

func (r *metricsReceiver) recordMemoryMetrics(now pcommon.Timestamp, stats *containerStats) {
	r.mb.RecordContainerMemoryUsageTotalDataPoint(now, int64(stats.MemUsage))
	r.mb.RecordContainerMemoryUsageLimitDataPoint(now, int64(stats.MemLimit))
	r.mb.RecordContainerMemoryPercentDataPoint(now, stats.MemPerc)
}

func (r *metricsReceiver) recordIOMetrics(now pcommon.Timestamp, stats *containerStats) {
	r.mb.RecordContainerBlockioIoServiceBytesRecursiveReadDataPoint(now, int64(stats.BlockInput))
	r.mb.RecordContainerBlockioIoServiceBytesRecursiveWriteDataPoint(now, int64(stats.BlockOutput))
}

// nanoseconds to seconds conversion truncating the fractional part
func toSecondsWithNanosecondPrecision(nanoseconds uint64) uint64 {
	return nanoseconds / 1e9
}
