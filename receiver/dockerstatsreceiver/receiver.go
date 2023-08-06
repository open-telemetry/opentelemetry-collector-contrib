// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	dtypes "github.com/docker/docker/api/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

const (
	defaultDockerAPIVersion         = 1.23
	minimalRequiredDockerAPIVersion = 1.22
)

type resultV2 struct {
	stats     *dtypes.StatsJSON
	container *docker.Container
	err       error
}

type receiver struct {
	config   *Config
	settings rcvr.CreateSettings
	client   *docker.Client
	mb       *metadata.MetricsBuilder
}

func newReceiver(set rcvr.CreateSettings, config *Config) *receiver {
	return &receiver{
		config:   config,
		settings: set,
		mb:       metadata.NewMetricsBuilder(config.MetricsBuilderConfig, set),
	}
}

func (r *receiver) start(ctx context.Context, _ component.Host) error {
	dConfig, err := docker.NewConfig(r.config.Endpoint, r.config.Timeout, r.config.ExcludedImages, r.config.DockerAPIVersion)
	if err != nil {
		return err
	}

	r.client, err = docker.NewDockerClient(dConfig, r.settings.Logger)
	if err != nil {
		return err
	}

	if err = r.client.LoadContainerList(ctx); err != nil {
		return err
	}

	go r.client.ContainerEventLoop(ctx)
	return nil
}

func (r *receiver) scrapeV2(ctx context.Context) (pmetric.Metrics, error) {
	containers := r.client.Containers()
	results := make(chan resultV2, len(containers))

	wg := &sync.WaitGroup{}
	wg.Add(len(containers))
	for _, container := range containers {
		go func(c docker.Container) {
			defer wg.Done()
			statsJSON, err := r.client.FetchContainerStatsAsJSON(ctx, c)
			if err != nil {
				results <- resultV2{nil, &c, err}
				return
			}

			results <- resultV2{
				stats:     statsJSON,
				container: &c,
				err:       nil}
		}(container)
	}

	wg.Wait()
	close(results)

	var errs error

	now := pcommon.NewTimestampFromTime(time.Now())
	for res := range results {
		if res.err != nil {
			// Don't know the number of failed stats, but one container fetch is a partial error.
			errs = multierr.Append(errs, scrapererror.NewPartialScrapeError(res.err, 0))
			continue
		}
		if err := r.recordContainerStats(now, res.stats, res.container); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	return r.mb.Emit(), errs
}

func (r *receiver) recordContainerStats(now pcommon.Timestamp, containerStats *dtypes.StatsJSON, container *docker.Container) error {
	// Always-present resource attrs + the user-configured resource attrs
	rb := r.mb.NewResourceBuilder()
	rb.SetContainerRuntime("docker")
	rb.SetContainerHostname(container.Config.Hostname)
	rb.SetContainerID(container.ID)
	rb.SetContainerImageName(container.Config.Image)
	rb.SetContainerName(strings.TrimPrefix(container.Name, "/"))
	rb.SetContainerImageID(container.Image)
	rb.SetContainerCommandLine(strings.Join(container.Config.Cmd, " "))
	resource := rb.Emit()

	for k, label := range r.config.EnvVarsToMetricLabels {
		if v := container.EnvMap[k]; v != "" {
			resource.Attributes().PutStr(label, v)
		}
	}
	for k, label := range r.config.ContainerLabelsToMetricLabels {
		if v := container.Config.Labels[k]; v != "" {
			resource.Attributes().PutStr(label, v)
		}
	}

	rmb := r.mb.ResourceMetricsBuilder(resource)
	var errs error
	r.recordCPUMetrics(now, rmb, &containerStats.CPUStats, &containerStats.PreCPUStats)
	r.recordMemoryMetrics(now, rmb, &containerStats.MemoryStats)
	r.recordBlkioMetrics(now, rmb, &containerStats.BlkioStats)
	r.recordNetworkMetrics(now, rmb, &containerStats.Networks)
	r.recordPidsMetrics(now, rmb, &containerStats.PidsStats)
	if err := r.recordBaseMetrics(now, rmb, container.ContainerJSONBase); err != nil {
		errs = multierr.Append(errs, err)
	}
	return errs
}

func (r *receiver) recordMemoryMetrics(now pcommon.Timestamp, rmb *metadata.ResourceMetricsBuilder,
	memoryStats *dtypes.MemoryStats) {
	totalUsage := calculateMemUsageNoCache(memoryStats)
	rmb.RecordContainerMemoryUsageTotalDataPoint(now, int64(totalUsage))

	rmb.RecordContainerMemoryUsageLimitDataPoint(now, int64(memoryStats.Limit))

	rmb.RecordContainerMemoryPercentDataPoint(now, calculateMemoryPercent(memoryStats.Limit, totalUsage))

	rmb.RecordContainerMemoryUsageMaxDataPoint(now, int64(memoryStats.MaxUsage))

	recorders := map[string]func(pcommon.Timestamp, int64){
		"cache":                     rmb.RecordContainerMemoryCacheDataPoint,
		"total_cache":               rmb.RecordContainerMemoryTotalCacheDataPoint,
		"rss":                       rmb.RecordContainerMemoryRssDataPoint,
		"total_rss":                 rmb.RecordContainerMemoryTotalRssDataPoint,
		"rss_huge":                  rmb.RecordContainerMemoryRssHugeDataPoint,
		"total_rss_huge":            rmb.RecordContainerMemoryTotalRssHugeDataPoint,
		"dirty":                     rmb.RecordContainerMemoryDirtyDataPoint,
		"total_dirty":               rmb.RecordContainerMemoryTotalDirtyDataPoint,
		"writeback":                 rmb.RecordContainerMemoryWritebackDataPoint,
		"total_writeback":           rmb.RecordContainerMemoryTotalWritebackDataPoint,
		"mapped_file":               rmb.RecordContainerMemoryMappedFileDataPoint,
		"total_mapped_file":         rmb.RecordContainerMemoryTotalMappedFileDataPoint,
		"pgpgin":                    rmb.RecordContainerMemoryPgpginDataPoint,
		"total_pgpgin":              rmb.RecordContainerMemoryTotalPgpginDataPoint,
		"pgpgout":                   rmb.RecordContainerMemoryPgpgoutDataPoint,
		"total_pgpgout":             rmb.RecordContainerMemoryTotalPgpgoutDataPoint,
		"pgfault":                   rmb.RecordContainerMemoryPgfaultDataPoint,
		"total_pgfault":             rmb.RecordContainerMemoryTotalPgfaultDataPoint,
		"pgmajfault":                rmb.RecordContainerMemoryPgmajfaultDataPoint,
		"total_pgmajfault":          rmb.RecordContainerMemoryTotalPgmajfaultDataPoint,
		"inactive_anon":             rmb.RecordContainerMemoryInactiveAnonDataPoint,
		"total_inactive_anon":       rmb.RecordContainerMemoryTotalInactiveAnonDataPoint,
		"active_anon":               rmb.RecordContainerMemoryActiveAnonDataPoint,
		"total_active_anon":         rmb.RecordContainerMemoryTotalActiveAnonDataPoint,
		"inactive_file":             rmb.RecordContainerMemoryInactiveFileDataPoint,
		"total_inactive_file":       rmb.RecordContainerMemoryTotalInactiveFileDataPoint,
		"active_file":               rmb.RecordContainerMemoryActiveFileDataPoint,
		"total_active_file":         rmb.RecordContainerMemoryTotalActiveFileDataPoint,
		"unevictable":               rmb.RecordContainerMemoryUnevictableDataPoint,
		"total_unevictable":         rmb.RecordContainerMemoryTotalUnevictableDataPoint,
		"hierarchical_memory_limit": rmb.RecordContainerMemoryHierarchicalMemoryLimitDataPoint,
		"hierarchical_memsw_limit":  rmb.RecordContainerMemoryHierarchicalMemswLimitDataPoint,
		"anon":                      rmb.RecordContainerMemoryAnonDataPoint,
		"file":                      rmb.RecordContainerMemoryFileDataPoint,
	}

	for name, val := range memoryStats.Stats {
		if recorder, ok := recorders[name]; ok {
			recorder(now, int64(val))
		}
	}
}

type blkioRecorder func(now pcommon.Timestamp, val int64, devMaj string, devMin string, operation string)

func (r *receiver) recordBlkioMetrics(now pcommon.Timestamp, rmb *metadata.ResourceMetricsBuilder,
	blkioStats *dtypes.BlkioStats) {
	recordSingleBlkioStat(now, blkioStats.IoMergedRecursive, rmb.RecordContainerBlockioIoMergedRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoQueuedRecursive, rmb.RecordContainerBlockioIoQueuedRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoServiceBytesRecursive, rmb.RecordContainerBlockioIoServiceBytesRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoServiceTimeRecursive, rmb.RecordContainerBlockioIoServiceTimeRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoServicedRecursive, rmb.RecordContainerBlockioIoServicedRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoTimeRecursive, rmb.RecordContainerBlockioIoTimeRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoWaitTimeRecursive, rmb.RecordContainerBlockioIoWaitTimeRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.SectorsRecursive, rmb.RecordContainerBlockioSectorsRecursiveDataPoint)
}

func recordSingleBlkioStat(now pcommon.Timestamp, statEntries []dtypes.BlkioStatEntry, recorder blkioRecorder) {
	for _, stat := range statEntries {
		recorder(
			now,
			int64(stat.Value),
			strconv.FormatUint(stat.Major, 10),
			strconv.FormatUint(stat.Minor, 10),
			strings.ToLower(stat.Op))
	}
}

func (r *receiver) recordNetworkMetrics(now pcommon.Timestamp, rmb *metadata.ResourceMetricsBuilder,
	networks *map[string]dtypes.NetworkStats) {
	if networks == nil || *networks == nil {
		return
	}

	for netInterface, stats := range *networks {
		rmb.RecordContainerNetworkIoUsageRxBytesDataPoint(now, int64(stats.RxBytes), netInterface)
		rmb.RecordContainerNetworkIoUsageTxBytesDataPoint(now, int64(stats.TxBytes), netInterface)
		rmb.RecordContainerNetworkIoUsageRxDroppedDataPoint(now, int64(stats.RxDropped), netInterface)
		rmb.RecordContainerNetworkIoUsageTxDroppedDataPoint(now, int64(stats.TxDropped), netInterface)
		rmb.RecordContainerNetworkIoUsageRxPacketsDataPoint(now, int64(stats.RxPackets), netInterface)
		rmb.RecordContainerNetworkIoUsageTxPacketsDataPoint(now, int64(stats.TxPackets), netInterface)
		rmb.RecordContainerNetworkIoUsageRxErrorsDataPoint(now, int64(stats.RxErrors), netInterface)
		rmb.RecordContainerNetworkIoUsageTxErrorsDataPoint(now, int64(stats.TxErrors), netInterface)
	}
}

func (r *receiver) recordCPUMetrics(now pcommon.Timestamp, rmb *metadata.ResourceMetricsBuilder,
	cpuStats *dtypes.CPUStats,
	prevStats *dtypes.CPUStats) {
	rmb.RecordContainerCPUUsageSystemDataPoint(now, int64(cpuStats.SystemUsage))
	rmb.RecordContainerCPUUsageTotalDataPoint(now, int64(cpuStats.CPUUsage.TotalUsage))
	rmb.RecordContainerCPUUsageKernelmodeDataPoint(now, int64(cpuStats.CPUUsage.UsageInKernelmode))
	rmb.RecordContainerCPUUsageUsermodeDataPoint(now, int64(cpuStats.CPUUsage.UsageInUsermode))
	rmb.RecordContainerCPUThrottlingDataThrottledPeriodsDataPoint(now, int64(cpuStats.ThrottlingData.ThrottledPeriods))
	rmb.RecordContainerCPUThrottlingDataPeriodsDataPoint(now, int64(cpuStats.ThrottlingData.Periods))
	rmb.RecordContainerCPUThrottlingDataThrottledTimeDataPoint(now, int64(cpuStats.ThrottlingData.ThrottledTime))
	rmb.RecordContainerCPUUtilizationDataPoint(now, calculateCPUPercent(prevStats, cpuStats))
	rmb.RecordContainerCPUPercentDataPoint(now, calculateCPUPercent(prevStats, cpuStats))

	for coreNum, v := range cpuStats.CPUUsage.PercpuUsage {
		rmb.RecordContainerCPUUsagePercpuDataPoint(now, int64(v), fmt.Sprintf("cpu%s", strconv.Itoa(coreNum)))
	}
}

func (r *receiver) recordPidsMetrics(now pcommon.Timestamp, rmb *metadata.ResourceMetricsBuilder,
	pidsStats *dtypes.PidsStats) {
	// pidsStats are available when kernel version is >= 4.3 and pids_cgroup is supported, it is empty otherwise.
	if pidsStats.Current != 0 {
		rmb.RecordContainerPidsCountDataPoint(now, int64(pidsStats.Current))
		if pidsStats.Limit != 0 {
			rmb.RecordContainerPidsLimitDataPoint(now, int64(pidsStats.Limit))
		}
	}
}

func (r *receiver) recordBaseMetrics(now pcommon.Timestamp, rmb *metadata.ResourceMetricsBuilder,
	base *types.ContainerJSONBase) error {
	t, err := time.Parse(time.RFC3339, base.State.StartedAt)
	if err != nil {
		// value not available or invalid
		return scrapererror.NewPartialScrapeError(fmt.Errorf("error retrieving container.uptime from Container.State.StartedAt: %w", err), 1)
	}
	if v := now.AsTime().Sub(t); v > 0 {
		rmb.RecordContainerUptimeDataPoint(now, v.Seconds())
	}
	return nil
}
